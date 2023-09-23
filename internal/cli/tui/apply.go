package tui

import (
	"context"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/client-go/kubernetes"

	"github.com/substratusai/substratus/internal/cli/client"
)

type ApplyModel struct {
	Ctx context.Context

	// Clients
	Client   client.Interface
	Resource *client.Resource
	K8s      *kubernetes.Clientset

	Object client.Object

	Path      string
	Namespace string

	upload    uploadModel
	readiness readinessModel
	pods      podsModel

	finalError error

	Style lipgloss.Style
}

func (m *ApplyModel) New() ApplyModel {
	m.upload = (&uploadModel{
		Ctx:       m.Ctx,
		Client:    m.Client,
		Resource:  m.Resource,
		Object:    m.Object,
		Path:      m.Path,
		Namespace: m.Namespace,
		Mode:      uploadModeCreate,
	}).New()
	m.readiness = (&readinessModel{
		Ctx:      m.Ctx,
		Client:   m.Client,
		Resource: m.Resource,
		Object:   m.Object,
	}).New()
	m.pods = (&podsModel{
		Ctx:      m.Ctx,
		Client:   m.Client,
		Resource: m.Resource,
		K8s:      m.K8s,
		Object:   m.Object,
	}).New()
	m.Style = appStyle
	return *m
}

func (m ApplyModel) Init() tea.Cmd {
	return m.upload.Init()
}

func (m ApplyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	log.Printf("MSG: %T", msg)
	{
		mdl, cmd := m.upload.Update(msg)
		m.upload = mdl.(uploadModel)
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

	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if msg.String() == "q" {
			cmds = append(cmds, tea.Quit)
		}

	case tarballUploadedMsg:
		m.Object = msg.Object
		m.readiness.Object = m.Object
		m.pods.Object = m.Object
		cmds = append(cmds,
			m.readiness.Init(),
			m.pods.Init(),
		)

	case tea.WindowSizeMsg:
		m.Style.Width(msg.Width)
		innerWidth := m.Style.GetWidth() - m.Style.GetHorizontalPadding()
		// NOTE: Use background coloring for style debugging.
		m.upload.Style = lipgloss.NewStyle().Width(innerWidth)    //.Background(lipgloss.Color("12"))
		m.readiness.Style = lipgloss.NewStyle().Width(innerWidth) //.Background(lipgloss.Color("202"))
		m.pods.SetStyle(logStyle.Copy().Width(innerWidth))        //.Background(lipgloss.Color("86")))

	case error:
		log.Printf("Error message: %v", msg)
		m.finalError = msg
	}

	return m, tea.Batch(cmds...)
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m ApplyModel) View() (v string) {
	defer func() {
		v = m.Style.Render(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Width(m.Style.GetWidth()-m.Style.GetHorizontalPadding()).Render("Error: "+m.finalError.Error()) + "\n"
		v += helpStyle("Press \"q\" to quit")
		return v
	}

	v += m.upload.View()
	v += m.readiness.View()
	v += m.pods.View()

	v += helpStyle("Press \"q\" to quit")

	return v
}
