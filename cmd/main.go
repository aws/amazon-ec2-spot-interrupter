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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/amazon-ec2-spot-interrupter/pkg/cli"
	"github.com/aws/amazon-ec2-spot-interrupter/pkg/itn"
	"github.com/aws/amazon-ec2-spot-interrupter/pkg/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// TODOs(bwagner5):
//   1. Option to pass tags instead of instance IDs
//   2. Option to pass an OD instance and have this tool create a matching instance that is spot to test an interruption
//   3. Automated chaos - give this tool a tag or vpc and allow it to randomly interrupt spot instances at will

var version string

type Options struct {
	instanceIDs []string
	delay       time.Duration
	clean       bool
	version     bool
	region      string
	profile     string
	interactive bool
}

func main() {
	options := Options{}
	rootCmd := &cobra.Command{
		Use:   "ec2-spot-interrupter",
		Short: "ec2-spot-interrupter is a simple CLI tool that triggers Amazon EC2 Spot Instance Interruption Notifications and Rebalance Recommendations.",
		Run: func(cmd *cobra.Command, _ []string) {
			if options.version {
				fmt.Println(version)
				os.Exit(0)
			}
			ctx := context.Background()
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(options.region), config.WithSharedConfigProfile(options.profile))
			if err != nil {
				fmt.Printf("❌ %s\n", err)
				os.Exit(1)
			}
			interrupter := itn.New(cfg)
			if options.interactive {
				p := tea.NewProgram(tui.NewModel(ctx, interrupter))
				if err := p.Start(); err != nil {
					fmt.Printf("❌ Error initializing TUI: %v", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
			experiment, events, err := interrupter.Interrupt(context.Background(), options.instanceIDs, options.delay, options.clean)
			if err != nil {
				fmt.Printf("❌ %s\n", err)
				os.Exit(1)
			}
			cli.PrintMonitor(experiment, events)
		},
	}
	rootCmd.PersistentFlags().StringSliceVarP(&options.instanceIDs, "instance-ids", "i", []string{}, "instance IDs to interrupt")
	rootCmd.PersistentFlags().BoolVarP(&options.clean, "clean", "c", true, "clean up the underlying simulations")
	rootCmd.PersistentFlags().DurationVarP(&options.delay, "delay", "d", time.Second*15, "duration until the interruption notification is sent")
	rootCmd.PersistentFlags().BoolVarP(&options.version, "version", "v", false, "the version")
	rootCmd.PersistentFlags().BoolVar(&options.interactive, "interactive", false, "interactive TUI")
	rootCmd.PersistentFlags().StringVarP(&options.region, "region", "r", "", "the AWS Region")
	rootCmd.PersistentFlags().StringVarP(&options.profile, "profile", "p", "", "the AWS Profile")
	rootCmd.Execute()
}
