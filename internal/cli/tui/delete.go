package tui

import (
	"context"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/substratusai/substratus/internal/cli/client"
)

type DeleteModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Scope     string
	Namespace string

	// Clients
	Client   client.Interface
	Resource *client.Resource

	// Original Object (could be a Dataset, Model, or Server)
	Object client.Object

	// Tarring
	tarredFileCount int
	tarball         *client.Tarball

	// Keeping track of whats happening
	operations map[operation]status

	// File syncing
	currentSyncingFile string
	lastSyncFailure    error

	// End times
	finalError error
}

func (m DeleteModel) kind() string {
	return m.Object.GetObjectKind().GroupVersionKind().Kind
}

func (m DeleteModel) cleanupAndQuitCmd() tea.Msg {
	log.Println("Cleaning up")
	os.Remove(m.tarball.TempDir)
	return tea.Quit()
}

func (m DeleteModel) Init() tea.Cmd {
	m.operations = map[operation]status{}
	return nil
}

func (m DeleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return nil, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m DeleteModel) View() string {
	return ""
}
