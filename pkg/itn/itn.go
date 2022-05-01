// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package itn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/ptr"
	"go.uber.org/multierr"
)

const (
	trustPolicy = `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": [
					  "fis.amazonaws.com"
					]
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`
	rolePolicy = `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Sid": "AllowFISExperimentRoleSpotInstanceActions",
				"Effect": "Allow",
				"Action": [
					"ec2:SendSpotInstanceInterruptions"
				],
				"Resource": "arn:aws:ec2:*:*:instance/*"
			}
		]
	}`
	spotITNAction = "aws:ec2:send-spot-instance-interruptions"
)

type ITN struct {
	cfg    aws.Config
	stsAPI *sts.Client
	fisAPI *fis.Client
	iamAPI *iam.Client
	ec2API *ec2.Client
}

func New(cfg aws.Config) *ITN {
	return &ITN{
		cfg:    cfg,
		stsAPI: sts.NewFromConfig(cfg),
		fisAPI: fis.NewFromConfig(cfg),
		iamAPI: iam.NewFromConfig(cfg),
		ec2API: ec2.NewFromConfig(cfg),
	}
}

// Interrupt will start an FIS experiment to send Spot ITNs to the instance IDs specified and then monitor
// the experiment for the progress.
func (i ITN) Interrupt(ctx context.Context, instanceIDs []string, delay time.Duration, clean bool) error {
	if err := i.validate(ctx, instanceIDs); err != nil {
		return err
	}
	experiment, err := i.createInterruptions(ctx, instanceIDs, delay)
	if err != nil {
		return err
	}
	err = i.monitor(ctx, experiment, delay)
	return multierr.Append(err, i.Clean(ctx, *experiment))
}

func (i ITN) validate(ctx context.Context, instanceIDs []string) error {
	paginator := ec2.NewDescribeInstancesPaginator(i.ec2API, &ec2.DescribeInstancesInput{InstanceIds: instanceIDs})
	var instances []ec2types.Instance
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, r := range out.Reservations {
			instances = append(instances, r.Instances...)
		}
	}
	var err error
	for _, instance := range instances {
		if instance.InstanceLifecycle != ec2types.InstanceLifecycleTypeSpot {
			err = multierr.Append(err, fmt.Errorf("%s is not a Spot instance", *instance.InstanceId))
		}
	}
	return err
}

// Clean deletes the generated experiment template from FIS
func (i ITN) Clean(ctx context.Context, experiment types.Experiment) error {
	_, err := i.fisAPI.DeleteExperimentTemplate(ctx, &fis.DeleteExperimentTemplateInput{Id: experiment.ExperimentTemplateId})
	return err
}

func (i ITN) monitor(ctx context.Context, experiment *types.Experiment, delay time.Duration) error {
	// TODO: use a table lib to make this prettier
	fmt.Println("===================================================================")
	fmt.Printf("📖 Experiment Summary: \n")
	fmt.Printf("        ID: %s\n", *experiment.Id)
	fmt.Printf("  Role ARN: %s\n", *experiment.RoleArn)
	fmt.Printf("    Action: %s\n", spotITNAction)
	fmt.Println("  Targets:")
	for _, target := range experiment.Targets {
		for _, arn := range target.ResourceArns {
			fmt.Printf("    - %s\n", i.arnToInstanceID(arn))
		}
	}
	fmt.Println("===================================================================")
	time.Sleep(2 * time.Second)
	fmt.Println("✅ Rebalance Recommendation sent!")
	if experiment.StartTime != nil && time.Until(*experiment.StartTime) < delay {
		timeUntilStart := delay - time.Until(*experiment.StartTime)
		fmt.Printf("⏳ Experiment will start in %d seconds\n", int(timeUntilStart.Seconds()))
		time.Sleep(timeUntilStart)
	}
	fmt.Printf("🤩 Experiment %s is starting!\n", *experiment.Id)
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			experimentUpdate, err := i.fisAPI.GetExperiment(ctx, &fis.GetExperimentInput{Id: experiment.Id})
			if err != nil {
				return err
			}
			switch experimentUpdate.Experiment.State.Status {
			case types.ExperimentStatusPending:
				fmt.Println("⏲ Experiment is pending")
			case types.ExperimentStatusInitiating:
				fmt.Println("🔧 Experiment is initializing")
			case types.ExperimentStatusFailed, types.ExperimentStatusStopped:
				return fmt.Errorf(*experimentUpdate.Experiment.State.Reason)
			case types.ExperimentStatusCompleted:
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("timed out")
		}
	}
}

func (i ITN) createInterruptions(ctx context.Context, instanceIDs []string, delay time.Duration) (*types.Experiment, error) {
	accountID, err := i.getAccountID(ctx)
	if err != nil {
		return nil, err
	}
	roleARN, err := i.getOrCreateFISRole(ctx, accountID)
	if err != nil {
		return nil, err
	}
	template := &fis.CreateExperimentTemplateInput{
		Actions:        map[string]types.CreateExperimentTemplateActionInput{},
		Targets:        map[string]types.CreateExperimentTemplateTargetInput{},
		StopConditions: []types.CreateExperimentTemplateStopConditionInput{{Source: aws.String("none")}},
		RoleArn:        roleARN,
		Description:    aws.String(fmt.Sprintf("trigger spot ITN for instances %v", instanceIDs)),
	}
	for j, batch := range i.batchInstances(instanceIDs, 5) {
		key := fmt.Sprintf("itn%d", j)
		template.Actions[key] = types.CreateExperimentTemplateActionInput{
			ActionId: ptr.String(spotITNAction),
			Parameters: map[string]string{
				// durationBeforeInterruption is the time before the instance is terminated, so we add 2 minutes
				// so that a user can configure the notificatin delay rather than the termination delay.
				"durationBeforeInterruption": fmt.Sprintf("PT%dS", int((time.Minute*2 + delay).Seconds())),
			},
			Targets: map[string]string{"SpotInstances": key},
		}
		template.Targets[key] = types.CreateExperimentTemplateTargetInput{
			ResourceType:  ptr.String("aws:ec2:spot-instance"),
			SelectionMode: ptr.String("ALL"),
			ResourceArns:  i.instanceIDsToARNs(batch, i.cfg.Region, accountID),
		}
	}
	experimentTemplate, err := i.fisAPI.CreateExperimentTemplate(ctx, template)
	if err != nil {
		return nil, err
	}
	experiment, err := i.fisAPI.StartExperiment(ctx, &fis.StartExperimentInput{ExperimentTemplateId: experimentTemplate.ExperimentTemplate.Id})
	if err != nil {
		return nil, err
	}
	return experiment.Experiment, nil
}

func (i ITN) batchInstances(instanceIDs []string, size int) [][]string {
	instanceIDBatches := [][]string{}
	currentBatch := []string{}
	for i, instanceID := range instanceIDs {
		if i%size == 0 && len(currentBatch) > 0 {
			instanceIDBatches = append(instanceIDBatches, currentBatch)
			currentBatch = []string{}
		}
		currentBatch = append(currentBatch, instanceID)
	}
	if len(currentBatch) > 0 {
		instanceIDBatches = append(instanceIDBatches, currentBatch)
	}
	return instanceIDBatches
}

func (i ITN) getOrCreateFISRole(ctx context.Context, accountID string) (*string, error) {
	roleName := "aws-fis-itn"
	out, err := i.iamAPI.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 ptr.String(roleName),
		AssumeRolePolicyDocument: ptr.String(trustPolicy),
	})
	var alreadyExists *iamtypes.EntityAlreadyExistsException
	if errors.As(err, &alreadyExists) {
		return ptr.String(fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName)), nil
	}
	if err != nil {
		return nil, err
	}
	_, err = i.iamAPI.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		PolicyName:     ptr.String(fmt.Sprintf("%s-policy", roleName)),
		PolicyDocument: ptr.String(rolePolicy),
		RoleName:       out.Role.RoleName,
	})
	if err != nil {
		return nil, err
	}
	return out.Role.Arn, nil
}

func (i ITN) getAccountID(ctx context.Context) (string, error) {
	identity, err := i.stsAPI.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return *identity.Account, nil
}

func (i ITN) instanceIDsToARNs(instanceIDs []string, region string, accountID string) []string {
	var arns []string
	for _, instanceID := range instanceIDs {
		arns = append(arns, fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", region, accountID, instanceID))
	}
	return arns
}

func (i ITN) arnToInstanceID(arn string) string {
	return strings.Split(strings.Split(arn, ":")[5], "/")[1]
}
