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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/aws/itn/pkg/cli"
	"github.com/aws/itn/pkg/itn"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type monitor struct {
	events     <-chan itn.Event
	spinner    spinner.Model
	experiment *types.Experiment
	summary    string
	eventLog   []itn.Event
}

func NewMonitor(experiment *types.Experiment, events <-chan itn.Event) monitor {
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("206"))
	sp.Spinner = spinner.Points
	return monitor{
		experiment: experiment,
		summary:    cli.Summary(experiment),
		events:     events,
		spinner:    sp,
	}
}

type eventMsg itn.Event
type doneMsg bool

func eventListener(events <-chan itn.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return doneMsg(true)
		}
		return eventMsg(event)
	}
}

func (m monitor) Init() tea.Cmd {
	return tea.Batch(spinner.Tick, eventListener(m.events))
}

func (m monitor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case eventMsg:
		m.eventLog = append(m.eventLog, itn.Event(msg))
		return m, eventListener(m.events)
	case doneMsg:
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m monitor) View() string {
	s := fmt.Sprintf("%s\n", m.summary)
	for _, event := range m.eventLog {
		s += fmt.Sprintf("%s\n", event.Message)
	}
	s += m.spinner.View()
	s += help()
	return s
}
