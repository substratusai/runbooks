package tui

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/client-go/kubernetes"

	"github.com/substratusai/substratus/internal/client"
)

type ApplyModel struct {
	// Cancellation
	Ctx context.Context

	// Clients
	K8s    *kubernetes.Clientset
	Client client.Interface

	// Config
	Path      string
	Filename  string
	Namespace Namespace

	// Focal object
	object   client.Object
	resource *client.Resource

	// Processes
	upload    uploadModel
	readiness readinessModel
	pods      podsModel

	// End times
	finalError error

	Style lipgloss.Style
}

func (m *ApplyModel) New() ApplyModel {
	m.upload = (&uploadModel{
		Ctx:    m.Ctx,
		Client: m.Client,
		Path:   m.Path,
		Mode:   uploadModeCreate,
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

func (m ApplyModel) Init() tea.Cmd {
	return readManifest(m.Ctx, filepath.Join(m.Path, m.Filename))
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
	case readManifestMsg:
		// TODO: Expect to fail:
		m.Namespace.Set(msg.obj)
		m.object = msg.obj

		res, err := m.Client.Resource(msg.obj)
		if err != nil {
			m.finalError = fmt.Errorf("resource client: %w", err)
		}
		m.resource = res

		m.upload.Object = m.object
		m.upload.Resource = m.resource
		cmds = append(cmds, m.upload.Init())

	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if msg.String() == "q" {
			cmds = append(cmds, tea.Quit)
		}

	case tarballUploadedMsg:
		m.object = msg.Object

		m.readiness.Object = m.object
		m.readiness.Resource = m.resource
		m.pods.Object = m.object
		m.pods.Resource = m.resource
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
