package tui

import (
	"context"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/substratusai/substratus/internal/client"
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
		log.Println("Initializing readiness")
		m.waiting = inProgress

	case objectUpdateMsg:
		m.Object = msg.Object

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

	if m.waiting == inProgress {
		kind := m.Object.GetObjectKind().GroupVersionKind().Kind
		v += fmt.Sprintf("%v (%v):\n", kind, m.Object.GetName())

		if w, ok := m.Object.(interface {
			GetConditions() *[]metav1.Condition
		}); ok {
			for _, c := range *w.GetConditions() {
				var prefix, suffix string
				if c.Status == metav1.ConditionTrue {
					prefix = checkMark.String() + " "
				} else {
					prefix = xMark.String() + " "
					suffix = " (" + c.Reason + ")"
				}
				v += lipgloss.NewStyle().Width(m.Style.GetWidth() - m.Style.GetHorizontalPadding()).
					Render(prefix + c.Type + suffix)
				v += "\n"
			}
		}
	} else if m.waiting == completed {
		kind := m.Object.GetObjectKind().GroupVersionKind().Kind
		v += fmt.Sprintf("%v (%v): Ready\n", kind, m.Object.GetName())
	}

	return v
}
