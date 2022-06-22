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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type ec2API interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type fisAPI interface {
	CreateExperimentTemplate(ctx context.Context, params *fis.CreateExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.CreateExperimentTemplateOutput, error)
	DeleteExperimentTemplate(ctx context.Context, params *fis.DeleteExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.DeleteExperimentTemplateOutput, error)
	GetExperiment(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error)
	StartExperiment(ctx context.Context, params *fis.StartExperimentInput, optFns ...func(*fis.Options)) (*fis.StartExperimentOutput, error)
}

type iamAPI interface {
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

type stsAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}
