package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/substratusai/substratus/internal/cli/client"
)

type DeleteModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Scope     string
	Namespace string

	// Clients
	Client   client.Interface
	Resource *client.Resource

	// Original Object (could be a Dataset, Model, or Server)
	Object client.Object

	// End times
	finalError error
}

func (m DeleteModel) kind() string {
	return m.Object.GetObjectKind().GroupVersionKind().Kind
}

func (m DeleteModel) Init() tea.Cmd {
	return nil
}

func (m DeleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return nil, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m DeleteModel) View() string {
	return ""
}
