package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

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

	// End times
	finalError error
}

func (m *readinessModel) New() readinessModel {
	m.waiting = inProgress
	return *m
}

func (m readinessModel) Init() tea.Cmd {
	return waitReadyCmd(m.Ctx, m.Resource, m.Object.DeepCopyObject().(client.Object))
}

func (m readinessModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case objectReadyMsg:
		m.waiting = completed
		m.Object = msg.Object
		return m, nil

	case error:
		m.finalError = msg
		return m, nil
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m readinessModel) View() (v string) {
	kind := m.Object.GetObjectKind().GroupVersionKind().Kind
	if m.waiting == inProgress {
		v += fmt.Sprintf("Waiting for %v to be ready...\n", kind)
	} else if m.waiting == completed {
		v += fmt.Sprintf("%v ready.\n", kind)
	}

	return v
}
