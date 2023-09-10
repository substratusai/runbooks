package cli

import (
	"context"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/substratusai/substratus/internal/cli/client"
)

type deleteModel struct {
	// Cancellation
	ctx context.Context

	// Config
	scope     string
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

func (m deleteModel) kind() string {
	return m.object.GetObjectKind().GroupVersionKind().Kind
}

func (m deleteModel) cleanupAndQuitCmd() tea.Msg {
	log.Println("Cleaning up")
	os.Remove(m.tarball.TempDir)
	return tea.Quit()
}

func (m deleteModel) Init() tea.Cmd {
	return nil
}

func (m deleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return nil, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m deleteModel) View() string {
	return ""
}
