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

package tui

import (
	"context"
	"fmt"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/itn/pkg/itn"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type options struct {
	instances      []*ec2types.Instance
	ctx            context.Context
	itn            *itn.ITN
	textInput      textinput.Model
	validationMsg  string
	processingOpts bool
}

func NewOptions(ctx context.Context, itn *itn.ITN, instances []*ec2types.Instance) options {
	ti := textinput.New()
	ti.SetValue("15s")
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 20
	return options{
		ctx:       ctx,
		itn:       itn,
		instances: instances,
		textInput: ti,
	}
}

type startInterruptMsg bool

func (o options) Init() tea.Cmd {
	return textinput.Blink
}

func (o options) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case startInterruptMsg:
		var instanceIDs []string
		for _, instance := range o.instances {
			instanceIDs = append(instanceIDs, *instance.InstanceId)
		}
		delay, err := o.validateDelay()
		if err != nil {
			o.textInput, cmd = o.textInput.Update(msg)
			return o, cmd
		}
		experiment, events, err := o.itn.Interrupt(o.ctx, instanceIDs, delay, true)
		if err != nil {
			return o, tea.Quit
		}
		monitor := NewMonitor(experiment, events)
		return monitor, monitor.Init()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return o, tea.Quit
		case "enter":
			o.processingOpts = true
			return o, func() tea.Msg {
				return startInterruptMsg(true)
			}
		}
	}
	o.textInput, cmd = o.textInput.Update(msg)
	return o, cmd
}

func (o *options) validateDelay() (time.Duration, error) {
	delay, err := time.ParseDuration(o.textInput.Value())
	if err != nil {
		o.validationMsg = "❌ invalid duration format (example: 1m)"
		return delay, fmt.Errorf("invalid duration format: %v", err)
	}
	o.validationMsg = "✅"
	return delay, err
}

func (o options) View() string {
	if o.processingOpts {
		return fmt.Sprintf("Creating Interruption Experiment \n%s", help())
	}
	return fmt.Sprintf(
		"How long to wait before sending the interruption notifications?\n%s\n%s\n%s",
		o.textInput.View(),
		o.validationMsg,
		help(),
	)
}
