package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/client-go/kubernetes"

	"github.com/substratusai/substratus/internal/client"
)

type PushModel struct {
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
	manifests manifestsModel
	upload    uploadModel
	readiness readinessModel
	pods      podsModel

	// End times
	finalError error

	Style lipgloss.Style
}

func (m *PushModel) New() PushModel {
	m.manifests = (&manifestsModel{
		Path:     m.Path,
		Filename: m.Filename,
		Kinds:    []string{"Model", "Dataset"},
	}).New()
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

func (m PushModel) Init() tea.Cmd {
	return m.manifests.Init()
}

func (m PushModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	log.Printf("MSG: %T", msg)
	{
		mdl, cmd := m.manifests.Update(msg)
		m.manifests = mdl.(manifestsModel)
		cmds = append(cmds, cmd)
	}

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
	case manifestSelectedMsg:
		m.object = msg.obj
		m.Namespace.Set(m.object)

		res, err := m.Client.Resource(m.object)
		if err != nil {
			b, _ := json.Marshal(m.object)
			log.Println("............", string(b))

			m.finalError = fmt.Errorf("resource client: %w", err)
			break
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
func (m PushModel) View() (v string) {
	defer func() {
		v = m.Style.Render(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Width(m.Style.GetWidth()-m.Style.GetHorizontalPadding()).Render("Error: "+m.finalError.Error()) + "\n"
		v += helpStyle("Press \"q\" to quit")
		return v
	}

	v += m.manifests.View()
	v += m.upload.View()
	v += m.readiness.View()
	v += m.pods.View()

	v += helpStyle("Press \"q\" to quit")

	return v
}
