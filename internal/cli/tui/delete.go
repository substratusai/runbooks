package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

	Objects map[string]client.Object

	Style lipgloss.Style

	// End times
	goodbye    string
	finalError error
}

func (m *DeleteModel) New() DeleteModel {
	m.Style = appStyle
	if m.Objects == nil {
		m.Objects = map[string]client.Object{}
	}
	return *m
}

func (m DeleteModel) Init() tea.Cmd {
	if len(m.Objects) == 0 {
		return listCmd(m.Ctx, m.Resource, m.Scope)
	} else {
		cmds := make([]tea.Cmd, 0, len(m.Objects))
		for _, obj := range m.Objects {
			cmds = append(cmds, deleteCmd(m.Ctx, m.Resource, obj))
		}
		return tea.Batch(cmds...)
	}
}

func (m DeleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case deletedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Deleted."
		}
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.Style.Width(msg.Width)
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m DeleteModel) View() (v string) {
	defer func() {
		v = m.Style.Render(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Width(m.Style.GetWidth()-m.Style.GetHorizontalMargins()-10).Render("Error: "+m.finalError.Error()) + "\n"
		return
	}

	if m.goodbye != "" {
		v += m.goodbye + "\n"
		return
	}

	return
}

type listMsg struct {
	items []client.Object
	error error
}

func listCmd(ctx context.Context, res *client.Resource, scope string) tea.Cmd {
	return func() tea.Msg {
		return listMsg{}
	}
}
