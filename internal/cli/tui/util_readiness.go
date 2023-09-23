package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/substratusai/substratus/internal/cli/client"
)

type readinessModel struct {
	// Cancellation
	Ctx context.Context

	// Clients
	Client   client.Interface
	Resource *client.Resource

	Object client.Object

	// Readiness
	waiting status

	Style lipgloss.Style
}

// New initializes all internal fields.
func (m *readinessModel) New() readinessModel {
	m.Style = lipgloss.NewStyle()
	return *m
}

func (m readinessModel) Active() bool {
	return m.waiting == inProgress
}

func (m readinessModel) Init() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg { return readinessInitMsg{} },
		waitReadyCmd(m.Ctx, m.Resource, m.Object.DeepCopyObject().(client.Object)),
	)
}

type readinessInitMsg struct{}

func (m readinessModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case readinessInitMsg:
		m.waiting = inProgress

	case objectReadyMsg:
		m.waiting = completed
		m.Object = msg.Object
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m readinessModel) View() (v string) {
	defer func() {
		if v != "" {
			v = m.Style.Render(v)
		}
	}()

	kind := m.Object.GetObjectKind().GroupVersionKind().Kind
	if m.waiting == inProgress {
		v += fmt.Sprintf("Waiting for %v to be ready...\n", kind)
	}

	return v
}
