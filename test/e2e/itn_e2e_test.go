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

package itn_e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// coupled with Makefile
const APP_PATH = "../../build/spot-itn"
const TEST_REGION = "us-west-2"
const API_RETRY_COUNT = 5
const RETRY_SLEEP_SEC = 7

var ctx = context.Background()

func TestSpotITN(t *testing.T) {
	// Pre-checks
	_, err := os.Stat(APP_PATH)
	require.Nil(t, err)

	// Setup
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(TEST_REGION))
	require.Nil(t, err)
	ec2Client := ec2.NewFromConfig(cfg)
	require.NotNil(t, ec2Client)

	launchTemplate, ltCleanup := CreateLTForTest(*ec2Client)
	require.NotNil(t, launchTemplate.LaunchTemplateId)
	defer ltCleanup()
	launchTemplateID := *launchTemplate.LaunchTemplateId

	spotInstance, fleetCleanup := CreateSpotInstance(*ec2Client, launchTemplateID)
	require.NotNil(t, spotInstance.InstanceId)
	defer fleetCleanup()
	spotInstanceID := *spotInstance.InstanceId

	// Run spot-interrupter with created Spot instance
	spotItnCommand := exec.Command(APP_PATH,
		"--instance-ids", spotInstanceID, "--region", TEST_REGION)
	spotiOutput, err := spotItnCommand.Output()
	require.Nil(t, err)
	spotiOutputClean := string(spotiOutput)

	// Validate expected events happened to the designated instance
	assert.Contains(t, spotiOutputClean, spotInstanceID)
	assert.Contains(t, spotiOutputClean, "✅ Rebalance Recommendation sent")
	// TODO issue: https://github.com/aws/amazon-ec2-spot-interrupter/issues/8
	// assert.Contains(t, spotiCleanOutput, "⏳ Interruption will be sent in 15 seconds")
	assert.Contains(t, spotiOutputClean, "✅ Spot 2-minute Interruption Notification sent")
	assert.Contains(t, spotiOutputClean, "✅ Spot Instance Shutdown sent")

	// Validate Spot instance terminating
	retry := API_RETRY_COUNT
	terminating := false
	for retry > 0 {
		time.Sleep(time.Second * RETRY_SLEEP_SEC)
		spotInstance = GetInstance(*ec2Client, spotInstanceID)
		if spotInstance.State != nil &&
			(spotInstance.State.Name == ec2types.InstanceStateNameTerminated ||
				spotInstance.State.Name == ec2types.InstanceStateNameShuttingDown) {
			terminating = true
			break
		}
		retry--
	}
	require.True(t, terminating)
}

/*
	Helper funcs
*/

// GetInstance returns an instance provided an instanceID
func GetInstance(client ec2.Client, instanceID string) *ec2types.Instance {
	instance := &ec2types.Instance{}
	output, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("instance-id"),
				Values: []string{string(instanceID)},
			},
		},
	})
	if err != nil {
		fmt.Printf("❌ DescribeInstances returned an error:  %s\n output: %+v\n", err, output)
		return instance
	}
	for _, r := range output.Reservations {
		for _, i := range r.Instances {
			return &i
		}
	}
	return instance
}

// CreateSpotInstance creates an 'instant' Fleet with a single Spot instance
// and returns the Spot instance(running) and cleanup func for the associated fleet
func CreateSpotInstance(client ec2.Client, lauchTemplateID string) (*ec2types.Instance, func()) {
	instance := &ec2types.Instance{}
	fleetConfigReq := ec2types.FleetLaunchTemplateConfigRequest{
		LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
			LaunchTemplateId: aws.String(lauchTemplateID),
			Version:          aws.String("$Default"),
		},
	}
	fleetInput := ec2.CreateFleetInput{
		Type:                  ec2types.FleetTypeInstant,
		LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{fleetConfigReq},
		TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
			TotalTargetCapacity:       aws.Int32(1),
			DefaultTargetCapacityType: ec2types.DefaultTargetCapacityTypeSpot,
			OnDemandTargetCapacity:    aws.Int32(0),
			SpotTargetCapacity:        aws.Int32(1),
		},
	}
	fleetOutput, err := client.CreateFleet(ctx, &fleetInput)
	if err != nil {
		fmt.Printf("❌ CreateFleet returned an error:  %s\n output: %+v\n", err, fleetOutput)
		return instance, nil
	}

	// cleanup func
	fleetCleanup := func() {
		output, err := client.DeleteFleets(ctx, &ec2.DeleteFleetsInput{
			FleetIds:           []string{*fleetOutput.FleetId},
			TerminateInstances: aws.Bool(true),
		})
		if err != nil {
			fmt.Printf("❌ DeleteFleets for fleet: %s returned an error:  %s\n output: %+v\n", *fleetOutput.FleetId, err, output)
		}
	}

	// Ensure spot instance is in a running state
	fleetInstanceID := ""
	for _, fi := range fleetOutput.Instances {
		for _, id := range fi.InstanceIds {
			fleetInstanceID = id
		}
	}
	retry := API_RETRY_COUNT
	for retry > 0 {
		time.Sleep(time.Second * RETRY_SLEEP_SEC)
		spotInstance := GetInstance(client, fleetInstanceID)
		if spotInstance.State != nil && spotInstance.State.Name == ec2types.InstanceStateNameRunning {
			instance = spotInstance
			break
		}
		retry--
	}

	return instance, fleetCleanup
}

// CreateLTForTest returns a LaunchTemplate with latest AL2 and wide
// vcpu and memory ranges to increase chances of acquiring Spot instance.
// Intended for testing due to its simplicity.
func CreateLTForTest(client ec2.Client) (*ec2types.LaunchTemplate, func()) {
	lt := &ec2types.LaunchTemplate{}

	// fetch latest AL2 image
	amiOutput, err := client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"amazon"},
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"amzn2-ami-hvm-*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"x86_64"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("virtualization-type"),
				Values: []string{"hvm"},
			},
			{
				Name:   aws.String("hypervisor"),
				Values: []string{"xen"},
			},
			{
				Name:   aws.String("description"),
				Values: []string{"Amazon Linux 2 AMI*"},
			},
			{
				Name:   aws.String("image-type"),
				Values: []string{"machine"},
			},
		},
	})
	if err != nil || len(amiOutput.Images) < 1 {
		fmt.Printf("❌ DescribeImages returned an error or no results:  %s\n output: %+v\n", err, amiOutput)
		return lt, nil
	}
	sort.Slice(amiOutput.Images, func(i, j int) bool {
		// sort images in descending order by CreationDate to
		// ensure the latest AL2 is first
		layout := "2006-01-02T15:04:05.000Z"
		iTime, _ := time.Parse(layout, *amiOutput.Images[i].CreationDate)
		jTime, _ := time.Parse(layout, *amiOutput.Images[j].CreationDate)
		return iTime.After(jTime)
	})
	latestAL2 := *amiOutput.Images[0].ImageId

	// build request and create LaunchTemplate
	instanceReqs := ec2types.InstanceRequirementsRequest{
		MemoryMiB: &ec2types.MemoryMiBRequest{
			Min: aws.Int32(0),
			Max: aws.Int32(16384), //16GiB
		},
		VCpuCount: &ec2types.VCpuCountRangeRequest{
			Min: aws.Int32(0),
			Max: aws.Int32(16),
		},
	}
	ltData := ec2types.RequestLaunchTemplateData{
		ImageId:              aws.String(latestAL2),
		InstanceRequirements: &instanceReqs,
	}
	ltReq := ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String("spoti-e2e-lt"),
		LaunchTemplateData: &ltData,
	}
	ltOutput, err := client.CreateLaunchTemplate(ctx, &ltReq)
	if err != nil {
		fmt.Printf("❌ CreateLaunchTemplate returned an error:  %s\n output: %+v\n", err, ltOutput)
		return lt, nil
	}
	launchTemplateID := *ltOutput.LaunchTemplate.LaunchTemplateId

	// cleanup func
	ltCleanup := func() {
		output, err := client.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{
			LaunchTemplateId: aws.String(launchTemplateID),
		})
		if err != nil {
			fmt.Printf("❌ DeleteLaunchTemplate returned an error:  %s\n output: %+v\n", err, output)
		}
	}

	// ensure LT available
	retry := API_RETRY_COUNT
	for retry > 0 {
		time.Sleep(time.Second * RETRY_SLEEP_SEC)
		ltDescribeOutput, err := client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
			LaunchTemplateIds: []string{launchTemplateID},
		})
		if err != nil {
			fmt.Printf("❌ DescribeLaunchTemplates returned an error:  %s\n output: %+v\n", err, ltDescribeOutput)
		}
		if len(ltDescribeOutput.LaunchTemplates) > 0 {
			lt = &ltDescribeOutput.LaunchTemplates[0]
			break
		}
		retry--
	}

	return lt, ltCleanup
}
