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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/ptr"
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
)

type ITN struct {
	cfg    aws.Config
	stsAPI *sts.Client
	fisAPI *fis.Client
	iamAPI *iam.Client
}

func New(cfg aws.Config) *ITN {
	return &ITN{
		cfg:    cfg,
		stsAPI: sts.NewFromConfig(cfg),
		fisAPI: fis.NewFromConfig(cfg),
		iamAPI: iam.NewFromConfig(cfg),
	}
}

func (i ITN) Interrupt(ctx context.Context, instanceIDs []string) error {
	experiment, err := i.createInterruptions(ctx, instanceIDs)
	if err != nil {
		return err
	}
	return i.monitor(ctx, experiment)
}

func (i ITN) monitor(ctx context.Context, experiment *types.Experiment) error {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			experimentUpdate, err := i.fisAPI.GetExperiment(ctx, &fis.GetExperimentInput{Id: experiment.Id})
			if err != nil {
				return err
			}

			if experimentUpdate.Experiment.State.Status == types.ExperimentStatusFailed ||
				experimentUpdate.Experiment.State.Status == types.ExperimentStatusStopped {
				return fmt.Errorf(*experimentUpdate.Experiment.State.Reason)
			}
			if experimentUpdate.Experiment.State.Status == types.ExperimentStatusCompleted {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("timed out")
		}
	}
}

func (i ITN) createInterruptions(ctx context.Context, instanceIDs []string) (*types.Experiment, error) {
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
			ActionId: ptr.String("aws:ec2:send-spot-instance-interruptions"),
			Parameters: map[string]string{
				"durationBeforeInterruption": "PT4M",
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
	arns := []string{}
	for _, instanceID := range instanceIDs {
		arns = append(arns, fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", region, accountID, instanceID))
	}
	return arns
}
