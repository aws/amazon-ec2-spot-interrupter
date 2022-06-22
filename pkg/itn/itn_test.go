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
	"fmt"
	"testing"
	"time"

	h "github.com/aws/itn/pkg/test"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	mockAccountID = "12345"
	mockRegion    = "us-weast-2"
)

func TestCreateInterruptions(t *testing.T) {
	ctx := context.Background()
	instanceIDs := []string{"InstanceID-1"}
	itn := ITN{
		cfg: aws.Config{
			Region: mockRegion,
		},
		fisClient: &fisMockClient{},
		iamClient: &iamMockClient{},
		stsClient: &stsMockClient{},
	}
	mockedDelay := time.Second * 5
	output, err := itn.createInterruptions(ctx, instanceIDs, time.Duration(mockedDelay))
	h.Ok(t, err)
	h.Equals(t, fmt.Sprintf("arn:aws:iam::%s:role/%s", mockAccountID, fisRoleName), *output.RoleArn)
	h.Equals(t, "none", *output.StopConditions[0].Source)

	// validate Actions
	actualAction := output.Actions["itn0"]
	h.Equals(t, "aws:ec2:send-spot-instance-interruptions", *actualAction.ActionId)
	h.Equals(t, "trigger spot ITN for instances [InstanceID-1]", *actualAction.Description)
	h.Equals(t, "PT125S", actualAction.Parameters["durationBeforeInterruption"])
	h.Equals(t, "itn0", actualAction.Targets["SpotInstances"])

	// validate Targets
	actualTarget := output.Targets["itn0"]
	h.Equals(t, []string{"arn:aws:ec2:us-weast-2:12345:instance/InstanceID-1"}, actualTarget.ResourceArns)
	h.Equals(t, "aws:ec2:spot-instance", *actualTarget.ResourceType)
	h.Equals(t, "ALL", *actualTarget.SelectionMode)
}

func TestValidate(t *testing.T) {
	// no instances
	ctx := context.Background()
	instanceIDs := []string{}
	itn := ITN{}
	err := itn.validate(ctx, instanceIDs)
	h.Nok(t, err)
	h.Equals(t, "no instances specified", err.Error())

	/*
		TODO(brycahta@) after refactoring pager:
		- no errors
		- spot/running instances cases
	*/

}

func TestGetOrCreateFISRole(t *testing.T) {
	ctx := context.Background()
	itn := ITN{
		iamClient: &iamMockClient{},
	}
	out, err := itn.getOrCreateFISRole(ctx, mockAccountID)
	h.Equals(t, fmt.Sprintf("arn:aws:iam::%s:role/%s", mockAccountID, fisRoleName), *out)
	h.Ok(t, err)

	// role already exists
	updatedContext := context.WithValue(ctx, "roleExists", "yup")
	out, err = itn.getOrCreateFISRole(updatedContext, mockAccountID)
	h.Equals(t, fmt.Sprintf("arn:aws:iam::%s:role/%s", mockAccountID, fisRoleName), *out)
	h.Ok(t, err)
}

func TestGetAccountID(t *testing.T) {
	ctx := context.Background()
	itn := ITN{
		stsClient: &stsMockClient{},
	}
	resp, err := itn.getAccountID(ctx)
	h.Equals(t, mockAccountID, resp)
	h.Ok(t, err)
}

func TestARNToInstanceID(t *testing.T) {
	expectedInstanceID := "someID"
	mockedARN := fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", mockRegion, mockAccountID, expectedInstanceID)
	h.Equals(t, expectedInstanceID, ARNToInstanceID(mockedARN))
}

func TestInstanceIDsToARNs(t *testing.T) {
	instanceIDs := []string{"some", "instance", "ids"}
	mockedARNPrefix := fmt.Sprintf("arn:aws:ec2:%s:%s:instance/", mockRegion, mockAccountID)
	expectedARNs := []string{
		mockedARNPrefix + "some",
		mockedARNPrefix + "instance",
		mockedARNPrefix + "ids",
	}
	itn := ITN{}
	out := itn.instanceIDsToARNs(instanceIDs, mockRegion, mockAccountID)
	h.Equals(t, expectedARNs, out)
}

func TestBatchInstances(t *testing.T) {
	instanceIDs := []string{}
	expectedBatches := [][]string{}
	itn := ITN{}
	// empty
	out := itn.batchInstances(instanceIDs, fisTargetLimit)
	h.Equals(t, expectedBatches, out)

	// < 5
	instanceIDs = []string{"some", "instance", "ids"}
	expectedBatches = [][]string{
		{"some", "instance", "ids"},
	}
	out = itn.batchInstances(instanceIDs, fisTargetLimit)
	h.Equals(t, expectedBatches, out)

	// multiple of 5
	instanceIDs = []string{
		"some",
		"instance",
		"ids",
		"four",
		"five",
		"six",
		"seven",
		"eight",
		"nine",
		"ten",
	}
	expectedBatches = [][]string{
		{"some", "instance", "ids", "four", "five"},
		{"six", "seven", "eight", "nine", "ten"},
	}
	out = itn.batchInstances(instanceIDs, fisTargetLimit)
	h.Equals(t, expectedBatches, out)

	// > 5
	instanceIDs = []string{
		"some",
		"instance",
		"ids",
		"four",
		"five",
		"six",
		"seven",
		"eight",
		"nine",
		"ten",
		"three",
		"more",
		"instances",
	}
	expectedBatches = [][]string{
		{"some", "instance", "ids", "four", "five"},
		{"six", "seven", "eight", "nine", "ten"},
		{"three", "more", "instances"},
	}
	out = itn.batchInstances(instanceIDs, fisTargetLimit)
	h.Equals(t, expectedBatches, out)
}

// Mocks

type fisMockClient struct {
	experimentTemplate fis.CreateExperimentTemplateOutput
}
type iamMockClient struct{}
type stsMockClient struct{}

func (f *fisMockClient) CreateExperimentTemplate(ctx context.Context, params *fis.CreateExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.CreateExperimentTemplateOutput, error) {
	mockedAction := types.ExperimentTemplateAction{
		ActionId:    params.Actions["itn0"].ActionId,
		Description: params.Description,
		Parameters:  params.Actions["itn0"].Parameters,
		Targets:     params.Actions["itn0"].Targets,
	}
	mockedActions := map[string]types.ExperimentTemplateAction{
		"itn0": mockedAction,
	}
	mockedStop := types.ExperimentTemplateStopCondition{
		Source: params.StopConditions[0].Source,
	}
	mockedTarget := types.ExperimentTemplateTarget{
		ResourceArns:  params.Targets["itn0"].ResourceArns,
		ResourceType:  params.Targets["itn0"].ResourceType,
		SelectionMode: params.Targets["itn0"].SelectionMode,
	}
	mockedTargets := map[string]types.ExperimentTemplateTarget{
		"itn0": mockedTarget,
	}
	mockedID := "id-12345"
	output := fis.CreateExperimentTemplateOutput{
		ExperimentTemplate: &types.ExperimentTemplate{
			Actions:        mockedActions,
			Description:    params.Description,
			Id:             &mockedID,
			RoleArn:        params.RoleArn,
			StopConditions: []types.ExperimentTemplateStopCondition{mockedStop},
			Targets:        mockedTargets,
		},
	}
	f.experimentTemplate = output
	return &output, nil
}

func (f *fisMockClient) DeleteExperimentTemplate(ctx context.Context, params *fis.DeleteExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.DeleteExperimentTemplateOutput, error) {
	return nil, nil
}

func (f *fisMockClient) GetExperiment(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error) {
	return nil, nil
}

func (f *fisMockClient) StartExperiment(ctx context.Context, params *fis.StartExperimentInput, optFns ...func(*fis.Options)) (*fis.StartExperimentOutput, error) {
	mockedExpTemplate := f.experimentTemplate.ExperimentTemplate
	mockedAction := types.ExperimentAction{
		ActionId:    mockedExpTemplate.Actions["itn0"].ActionId,
		Description: mockedExpTemplate.Actions["itn0"].Description,
		Parameters:  mockedExpTemplate.Actions["itn0"].Parameters,
		Targets:     mockedExpTemplate.Actions["itn0"].Targets,
	}
	mockedActions := map[string]types.ExperimentAction{
		"itn0": mockedAction,
	}
	mockedStop := types.ExperimentStopCondition{
		Source: mockedExpTemplate.StopConditions[0].Source,
	}
	mockedTarget := types.ExperimentTarget{
		ResourceArns:  mockedExpTemplate.Targets["itn0"].ResourceArns,
		ResourceType:  mockedExpTemplate.Targets["itn0"].ResourceType,
		SelectionMode: mockedExpTemplate.Targets["itn0"].SelectionMode,
	}
	mockedTargets := map[string]types.ExperimentTarget{
		"itn0": mockedTarget,
	}
	output := fis.StartExperimentOutput{
		Experiment: &types.Experiment{
			Actions:        mockedActions,
			RoleArn:        mockedExpTemplate.RoleArn,
			StopConditions: []types.ExperimentStopCondition{mockedStop},
			Targets:        mockedTargets,
		},
	}
	return &output, nil
}

func (i *iamMockClient) CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	if ctx.Value("roleExists") != nil {
		var alreadyExists *iamtypes.EntityAlreadyExistsException
		return nil, alreadyExists
	}
	mockRoleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", mockAccountID, fisRoleName)
	// cannot take address of const
	roleName := fisRoleName
	out := iam.CreateRoleOutput{
		Role: &iamtypes.Role{
			Arn:      &mockRoleARN,
			RoleName: &roleName,
		},
	}
	return &out, nil
}

func (i *iamMockClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	return nil, nil
}

func (s *stsMockClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	mockAcct := mockAccountID
	out := sts.GetCallerIdentityOutput{
		Account: &mockAcct,
	}
	return &out, nil
}
