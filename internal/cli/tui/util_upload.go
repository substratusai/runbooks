package tui

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/substratusai/substratus/internal/cli/client"
)

type uploadModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Path      string
	Namespace string

	// Clients
	Client   client.Interface
	Resource *client.Resource

	Mode     uploadMode
	applying status
	creating status

	// Original Object (could be a Dataset, Model, or Server)
	Object client.Object

	// Tarring
	tarring         status
	tarredFileCount int
	tarball         *client.Tarball

	// Uploading
	uploading      status
	uploadProgress progress.Model

	// End times
	// finalError error
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

func (m *uploadModel) New() uploadModel {
	m.tarring = inProgress
	m.uploadProgress = progress.New(progress.WithDefaultGradient())
	return *m
}

func (m uploadModel) Init() tea.Cmd {
	return prepareTarballCmd(m.Ctx, m.Path)
}

func (m uploadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
			return m, createWithUploadCmd(m.Ctx, m.Resource, m.Object.DeepCopyObject().(client.Object), m.tarball)
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

	case tea.WindowSizeMsg:
		m.uploadProgress.Width = msg.Width - padding*2 - 4
		if m.uploadProgress.Width > maxWidth {
			m.uploadProgress.Width = maxWidth
		}
		return m, nil

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
	if m.tarring == inProgress {
		v += "Tarring...\n"
		v += fmt.Sprintf("File count: %v\n", m.tarredFileCount)
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
