package tui

import (
	"context"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
	"k8s.io/client-go/kubernetes"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/client"
)

type ServeModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Path          string
	Namespace     Namespace
	NoOpenBrowser bool

	// Clients
	Client client.Interface
	K8s    *kubernetes.Clientset

	// Current Server
	server   *apiv1.Server
	resource *client.Resource

	readiness readinessModel
	pods      podsModel

	// Ready to open browser
	portForwarding status
	localURL       string

	Style lipgloss.Style

	// End times
	quitting   bool
	goodbye    string
	finalError error
}

func (m *ServeModel) New() ServeModel {
	m.readiness = (&readinessModel{
		Ctx:    m.Ctx,
		Client: m.Client,
	}).New()
	m.pods = (&podsModel{
		Ctx:    m.Ctx,
		Client: m.Client,
		K8s:    m.K8s,
	}).New()

	m.Style = appStyle

	return *m
}

func (m ServeModel) Init() tea.Cmd {
	return readManifest(m.Ctx, m.Path)
}

func (m ServeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	log.Printf("MSG: %T", msg)

	{
		mdl, cmd := m.readiness.Update(msg)
		m.readiness = mdl.(readinessModel)
		cmds = append(cmds, cmd)
	}

	{
		mdl, cmd := m.pods.Update(msg)
		m.pods = mdl.(podsModel)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case readManifestMsg:
		m.Namespace.Set(msg.obj)
		// TODO: Expect to fail:
		m.server = msg.obj.(*apiv1.Server)

		res, err := m.Client.Resource(m.server)
		if err != nil {
			m.finalError = fmt.Errorf("resource client: %w", err)
		}
		m.resource = res

		m.readiness.Object = m.server
		m.readiness.Resource = m.resource

		m.pods.Object = m.server
		m.pods.Resource = m.resource

		cmds = append(cmds,
			m.readiness.Init(),
			m.pods.Init(),
		)

	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if m.quitting {
			switch msg.String() {
			case "esc":
				if m.finalError == nil {
					m.quitting = false
				}

			case "l":
				cmds = append(cmds, tea.Quit)
			case "s":
				cmds = append(cmds, suspendCmd(context.Background(), m.resource, m.server))
			case "d":
				cmds = append(cmds, deleteCmd(context.Background(), m.resource, m.server))
			}
		} else {
			if msg.String() == "q" {
				m.quitting = true
			}
		}

	case suspendedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Server suspended."
		}
		cmds = append(cmds, tea.Quit)

	case deletedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Server deleted."
		}
		cmds = append(cmds, tea.Quit)

	case objectReadyMsg:
		m.server = msg.Object.(*apiv1.Server)
		cmds = append(cmds) // TODO: Port-forward to Pod.
		// portForwardCmd(m.Ctx, m.Client, client.PodForNotebook(m.Server)),

	case portForwardReadyMsg:
		cmds = append(cmds, serverOpenInBrowser(m.server.DeepCopy()))

	case localURLMsg:
		m.localURL = string(msg)

	case tea.WindowSizeMsg:
		m.Style.Width(msg.Width)
		innerWidth := m.Style.GetWidth() - m.Style.GetHorizontalPadding()
		m.readiness.Style = lipgloss.NewStyle().Width(innerWidth)
		m.pods.SetStyle(logStyle.Width(innerWidth))

	case error:
		log.Printf("Error message: %v", msg)
		m.finalError = msg
		m.quitting = true
	}

	return m, tea.Batch(cmds...)
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m ServeModel) View() (v string) {
	defer func() {
		v = m.Style.Render(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Width(m.Style.GetWidth()-m.Style.GetHorizontalMargins()-10).Render("Error: "+m.finalError.Error()) + "\n"
		v += helpStyle("Press \"s\" to suspend, \"d\" to delete")
		// v += helpStyle("Press \"l\" to leave be, \"s\" to suspend, \"d\" to delete")
		return
	}

	if m.goodbye != "" {
		v += m.goodbye + "\n"
		return
	}

	if m.quitting {
		v += "Quitting...\n"
		v += helpStyle("Press \"s\" to suspend, \"d\" to delete, \"ESC\" to cancel")
		// v += helpStyle("Press \"l\" to leave be, \"s\" to suspend, \"d\" to delete, \"ESC\" to cancel")
		return
	}

	v += m.readiness.View()
	v += m.pods.View()

	if m.portForwarding == inProgress {
		v += "Port-forwarding...\n"
	}

	if m.localURL != "" && m.portForwarding == inProgress {
		v += "\n"
		v += fmt.Sprintf("Server URL: %v\n", m.localURL)
	}

	v += helpStyle("Press \"q\" to quit")

	return v
}

func serverOpenInBrowser(s *apiv1.Server) tea.Cmd {
	return func() tea.Msg {
		url := "http://localhost:8080"
		log.Printf("Opening browser to %s\n", url)
		browser.OpenURL(url)
		return localURLMsg(url)
	}
}
