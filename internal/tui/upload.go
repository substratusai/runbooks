package tui

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/substratusai/substratus/internal/client"
)

type uploadModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Path string

	// Clients
	Client   client.Interface
	Resource *client.Resource

	Increment bool
	Replace   bool
	Mode      uploadMode
	applying  status
	creating  status

	// Original Object (could be a Dataset, Model, or Server)
	Object client.Object

	// Tarring
	tarring         status
	tarredFileCount int
	tarball         *client.Tarball

	// Uploading
	uploading      status
	uploadProgress progress.Model

	Style lipgloss.Style
}

type uploadMode int

const (
	uploadModeCreate = 0
	uploadModeApply  = 1
)

func (m uploadModel) kind() string {
	return m.Object.GetObjectKind().GroupVersionKind().Kind
}

func (m uploadModel) cleanup() {
	log.Println("Cleaning up")
	os.Remove(m.tarball.TempDir)
}

// New initializes all internal fields.
func (m *uploadModel) New() uploadModel {
	m.Style = lipgloss.NewStyle()
	m.uploadProgress = progress.New(progress.WithDefaultGradient())
	return *m
}

func (m uploadModel) Active() bool {
	return true
	//if m.Mode == uploadModeApply {
	//	return m.applying != completed
	//} else if m.Mode == uploadModeCreate {
	//	return m.creating != completed
	//} else {
	//	panic("Unrecognized mode")
	//}
}

func (m uploadModel) Init() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg { return uploadInitMsg{} },
		prepareTarballCmd(m.Ctx, m.Path),
	)
}

type uploadInitMsg struct{}

func (m uploadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case uploadInitMsg:
		m.tarring = inProgress

	case fileTarredMsg:
		m.tarredFileCount++
		return m, nil

	case tarballCompleteMsg:
		m.tarring = completed
		m.tarball = msg
		if m.Mode == uploadModeApply {
			m.applying = inProgress
			return m, applyWithUploadCmd(m.Ctx, m.Resource, m.Object.DeepCopyObject().(client.Object), m.tarball)
		} else if m.Mode == uploadModeCreate {
			m.creating = inProgress
			return m, createWithUploadCmd(m.Ctx, m.Resource, m.Object.DeepCopyObject().(client.Object), m.tarball, m.Increment, m.Replace)
		} else {
			panic("unkown upload mode")
		}

	case appliedWithUploadMsg:
		m.Object = msg.Object
		m.applying = completed
		m.uploading = inProgress
		return m, uploadTarballCmd(m.Ctx, m.Resource, m.Object.DeepCopyObject().(client.Object), m.tarball)

	case createdWithUploadMsg:
		m.Object = msg.Object
		m.creating = completed
		m.uploading = inProgress
		return m, uploadTarballCmd(m.Ctx, m.Resource, m.Object.DeepCopyObject().(client.Object), m.tarball)

	case uploadTarballProgressMsg:
		return m, m.uploadProgress.SetPercent(float64(msg))

	case tarballUploadedMsg:
		m.uploading = completed
		m.Object = msg.Object

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		progressModel, cmd := m.uploadProgress.Update(msg)
		m.uploadProgress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m uploadModel) View() (v string) {
	defer func() {
		if v != "" {
			v = m.Style.Render(v)
		}
	}()
	m.uploadProgress.Width = m.Style.GetWidth()

	if m.tarring == inProgress {
		v += "Tarring...\n"
		v += fmt.Sprintf("File count: %v\n", m.tarredFileCount)
	}

	if m.applying == inProgress {
		v += "Applying...\n"
	}

	if m.creating == inProgress {
		v += "Creating...\n"
	}

	if m.uploading == inProgress {
		v += "Uploading...\n\n"
		v += m.uploadProgress.View() + "\n\n"
	}

	return v
}
