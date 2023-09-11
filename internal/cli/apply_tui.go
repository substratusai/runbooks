package cli

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/substratusai/substratus/internal/cli/client"
)

type applyModel struct {
	// Cancellation
	ctx context.Context

	// Config
	path      string
	namespace string

	// Clients
	client   client.Interface
	resource *client.Resource

	// Original Object (could be a Dataset, Model, or Server)
	object client.Object

	// Tarring
	tarredFileCount int
	tarball         *client.Tarball

	// Uploading
	uploadProgress progress.Model

	// Keeping track of whats happening
	operations map[operation]status

	// File syncing
	currentSyncingFile string
	lastSyncFailure    error

	// End times
	finalError error
}

func (m applyModel) kind() string {
	return m.object.GetObjectKind().GroupVersionKind().Kind
}

func (m applyModel) cleanupAndQuitCmd() tea.Msg {
	log.Println("Cleaning up")
	os.Remove(m.tarball.TempDir)
	return tea.Quit()
}

func (m applyModel) Init() tea.Cmd {
	m.operations[tarring] = inProgress
	return prepareTarballCmd(m.ctx, m.path)
}

func (m applyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if msg.String() == "q" {
			return m, tea.Quit
		}

	case fileTarredMsg:
		m.tarredFileCount++
		return m, nil

	case tarballCompleteMsg:
		m.operations[tarring] = completed
		m.tarball = msg
		m.operations[creating] = inProgress
		return m, createWithUploadCmd(m.ctx, m.resource, m.object.DeepCopyObject().(client.Object), m.tarball)

	case createdWithUploadMsg:
		m.object = msg.Object
		m.operations[creating] = completed
		return m, uploadTarballCmd(m.ctx, m.resource, m.object.DeepCopyObject().(client.Object), m.tarball)

	case uploadTarballProgressMsg:
		m.operations[uploading] = inProgress
		return m, m.uploadProgress.SetPercent(float64(msg))

	case tarballUploadedMsg:
		m.operations[uploading] = completed
		m.object = msg.Object
		m.operations[waitingReady] = inProgress
		return m, waitReadyCmd(m.ctx, m.resource, m.object.DeepCopyObject().(client.Object))

	case objectReadyMsg:
		m.operations[waitingReady] = completed
		m.object = msg.Object
		m.operations[syncingFiles] = inProgress
		return m, nil

	case operationMsg:
		// TODO: Switch all other operations management to this.
		m.operations[msg.operation] = msg.status
		return m, nil

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

	case error:
		m.finalError = msg
		return m, nil
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m applyModel) View() (v string) {
	defer func() {
		v = appStyle(v)
	}()

	if m.finalError != nil {
		v += errorStyle("Error: "+m.finalError.Error()) + "\n"
		v += helpStyle("Press \"q\" to quit")
		return v
	}

	var totalInProgress int
	for _, status := range m.operations {
		if status == inProgress {
			totalInProgress++
		}
	}

	if m.operations[tarring] == inProgress {
		v += "Tarring...\n"
		v += fmt.Sprintf("File count: %v\n", m.tarredFileCount)
	} else if totalInProgress == 0 && (m.operations[tarring] == completed) {
		v += "Tarring complete.\n"
	}

	if m.operations[creating] == inProgress {
		v += "Creating...\n"
	} else if totalInProgress == 0 && (m.operations[creating] == completed) {
		v += fmt.Sprintf("%s created.\n", m.kind())
	}

	if m.operations[uploading] == inProgress {
		v += "Uploading...\n\n"
		v += m.uploadProgress.View() + "\n\n"
	} else if totalInProgress == 0 && (m.operations[uploading] == completed) {
		v += "Upload complete.\n"
	}

	if m.operations[waitingReady] == inProgress {
		v += fmt.Sprintf("Waiting for %v to be ready...\n", m.kind())
	} else if m.operations[waitingReady] == completed {
		v += fmt.Sprintf("%v ready.\n", m.kind())
	}

	v += helpStyle("Press \"q\" to quit")

	return v
}
