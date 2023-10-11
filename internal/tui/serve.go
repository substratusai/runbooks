package tui

import (
	"context"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/client"
)

type ServeModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Namespace     Namespace
	Path          string
	Filename      string
	NoOpenBrowser bool

	// Clients
	Client client.Interface
	K8s    *kubernetes.Clientset

	// Current Server
	server   *apiv1.Server
	resource *client.Resource
	readyPod *corev1.Pod

	applying status

	manifests manifestsModel
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
	m.manifests = (&manifestsModel{
		Path:           m.Path,
		Filename:       m.Filename,
		Kinds:          []string{"Server"},
		SubstratusOnly: true,
	}).New()
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
	return m.manifests.Init()
}

func (m ServeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	log.Printf("MSG: %T", msg)

	{
		mdl, cmd := m.manifests.Update(msg)
		m.manifests = mdl.(manifestsModel)
		cmds = append(cmds, cmd)
	}

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

	apply := func() {
		res, err := m.Client.Resource(m.server)
		if err != nil {
			m.finalError = fmt.Errorf("resource client: %w", err)
			return
		}
		m.resource = res

		m.applying = inProgress
		cmds = append(cmds, applyCmd(m.Ctx, m.resource, &applyInput{Object: m.server.DeepCopy()}))
	}

	switch msg := msg.(type) {
	case manifestSelectedMsg:
		m.Namespace.Set(msg.obj)
		m.server = msg.obj.(*apiv1.Server)
		apply()

	case appliedMsg:
		if msg.err != nil {
			m.finalError = msg.err
			break
		}
		m.applying = completed
		m.server = msg.Object.(*apiv1.Server)

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
			// case "s":
			//	cmds = append(cmds, suspendCmd(context.Background(), m.resource, m.server))
			case "d":
				cmds = append(cmds, deleteCmd(context.Background(), m.resource, m.server))
			}
		} else {
			if msg.String() == "q" {
				m.quitting = true
			}
		}

	// case suspendedMsg:
	//	if msg.error != nil {
	//		m.finalError = msg.error
	//	} else {
	//		m.goodbye = "Server suspended."
	//	}
	//	cmds = append(cmds, tea.Quit)

	case deletedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Server deleted."
		}
		cmds = append(cmds, tea.Quit)

	case objectReadyMsg:
		m.server = msg.Object.(*apiv1.Server)
	// TODO: What to do?
	// cmds = append(cmds) // TODO: Port-forward to Pod.
	// portForwardCmd(m.Ctx, m.Client, client.PodForNotebook(m.Server)),
	//
	case podWatchMsg:
		if m.readyPod != nil {
			break
		}
		if msg.Pod.Labels == nil || msg.Pod.Labels["role"] != "run" {
			break
		}

		var ready bool
		for _, c := range msg.Pod.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}

		if ready {
			m.readyPod = msg.Pod.DeepCopy()
			cmds = append(cmds,
				portForwardCmd(m.Ctx, m.Client,
					types.NamespacedName{Namespace: m.readyPod.Namespace, Name: m.readyPod.Name},
					client.ForwardedPorts{Local: 8000, Pod: 8080},
				),
			)
		}

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
		// v += helpStyle("Press \"s\" to suspend, \"d\" to delete")
		v += helpStyle("Press \"l\" to leave be, \"d\" to delete")
		return
	}

	if m.goodbye != "" {
		v += m.goodbye + "\n"
		return
	}

	if m.quitting {
		v += "Quitting...\n"
		v += helpStyle("Press \"l\" to leave be, \"d\" to delete, \"ESC\" to cancel")
		// v += helpStyle("Press \"s\" to suspend, \"d\" to delete, \"ESC\" to cancel")
		// v += helpStyle("Press \"l\" to leave be, \"s\" to suspend, \"d\" to delete, \"ESC\" to cancel")
		return
	}

	if m.applying == inProgress {
		v += "Applying...\n\n"
	}

	v += m.manifests.View()
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
		url := "http://localhost:8000"
		log.Printf("Opening browser to %s\n", url)
		browser.OpenURL(url)
		return localURLMsg(url)
	}
}
