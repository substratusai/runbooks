package tui

import (
	"context"
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/substratusai/substratus/internal/cli/client"
)

type ApplyModel struct {
	Ctx      context.Context
	Client   client.Interface
	Resource *client.Resource
	Object   client.Object

	Path      string
	Namespace string

	upload    uploadModel
	readiness readinessModel
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
	return *m
}

func (m ApplyModel) Init() tea.Cmd {
	return m.upload.Init()
}

func (m ApplyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	{
		mdl, cmd := m.upload.Update(msg)
		m.upload = mdl.(uploadModel)
		cmds = append(cmds, cmd)
	}

	if m.readiness.waiting != notStarted {
		mdl, cmd := m.readiness.Update(msg)
		m.readiness = mdl.(readinessModel)
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
		m.readiness = (&readinessModel{
			Ctx:      m.Ctx,
			Object:   m.Object,
			Client:   m.Client,
			Resource: m.Resource,
		}).New()
		cmds = append(cmds, m.readiness.Init())
	}

	return m, tea.Batch(cmds...)
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m ApplyModel) View() (v string) {
	defer func() {
		v = appStyle(v)
	}()

	v += m.upload.View()
	if m.readiness.waiting != notStarted {
		v += m.readiness.View()
	}

	v += helpStyle("Press \"q\" to quit")

	return v
}
