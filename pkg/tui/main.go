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
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samber/lo"
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render

type model struct {
	choices     []ec2types.Instance
	cursor      int
	selected    map[int]*ec2types.Instance
	ctx         context.Context
	itn         *itn.ITN
	initialized bool
	spinner     spinner.Model
}

type spotInstancesMsg []ec2types.Instance
type retrySpotInstances time.Time

func NewModel(ctx context.Context, itn *itn.ITN) model {
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("206"))
	sp.Spinner = spinner.Points
	return model{
		selected: map[int]*ec2types.Instance{},
		ctx:      ctx,
		itn:      itn,
		spinner:  sp,
	}
}

func initialModel(ctx context.Context, itn *itn.ITN) tea.Cmd {
	return func() tea.Msg {
		instances, err := itn.SpotInstances(ctx)
		if err != nil {
			panic(err)
		}
		return spotInstancesMsg(instances)
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		spinner.Tick,
		initialModel(m.ctx, m.itn),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spotInstancesMsg:
		m.choices = msg
		m.initialized = true
		if len(msg) == 0 {
			return m, tea.Every(time.Second*15, func(t time.Time) tea.Msg {
				return retrySpotInstances(t)
			})
		}
	case retrySpotInstances:
		return m, initialModel(m.ctx, m.itn)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case " ":
			if _, ok := m.selected[m.cursor]; ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = &m.choices[m.cursor]
			}
		case "enter":
			opts := NewOptions(m.ctx, m.itn, lo.Values(m.selected))
			return opts, opts.Init()
		}
	}
	return m, nil
}

func instanceName(i ec2types.Instance) string {
	for _, tag := range i.Tags {
		if *tag.Key == "Name" {
			return *tag.Value
		}
	}
	return ""
}

func (m model) View() string {
	if !m.initialized {
		return fmt.Sprintf("Finding Spot instances %s\n%s", m.spinner.View(), help())
	}
	if len(m.choices) == 0 {
		return fmt.Sprintf("There are currently no Spot instances running...\nI'll keep checking though %s\n%s", m.spinner.View(), help())
	}
	// The header
	s := "Which Spot instances would you like to interrupt?\n\n"

	// Iterate over our choices
	for i, choice := range m.choices {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Is this choice selected?
		checked := " " // not selected
		if _, ok := m.selected[i]; ok {
			checked = "x" // selected!
		}

		// Render the row
		s += fmt.Sprintf("%s [%s] %s (%s)\n", cursor, checked, *choice.InstanceId, instanceName(choice))
	}

	// The footer
	s += help()

	// Send the UI for rendering
	return s
}

func help() string {
	return helpStyle("\nPress q to quit.\n")
}
