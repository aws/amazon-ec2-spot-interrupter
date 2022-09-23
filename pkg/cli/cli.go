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

package cli

import (
	"fmt"

	"github.com/aws/amazon-ec2-spot-interrupter/pkg/itn"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
)

func Summary(experiment *types.Experiment) string {
	// TODO: use a table lib to make this prettier
	s := ""
	s += "===================================================================\n"
	s += "📖 Experiment Summary: \n"
	s += fmt.Sprintf("        ID: %s\n", *experiment.Id)
	s += fmt.Sprintf("  Role ARN: %s\n", *experiment.RoleArn)
	s += fmt.Sprintf("    Action: %s\n", itn.SpotITNAction)
	s += "   Targets:\n"
	for _, target := range experiment.Targets {
		for _, arn := range target.ResourceArns {
			s += fmt.Sprintf("    - %s\n", itn.ARNToInstanceID(arn))
		}
	}
	s += "===================================================================\n"
	return s
}

func PrintMonitor(experiment *types.Experiment, events <-chan itn.Event) {
	fmt.Print(Summary(experiment))
	for event := range events {
		fmt.Printf("%s: %s\n", event.Timestamp.Format("2006-01-02T15:04:05"), event.Message)
	}
}
