package notebook

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cli/client"
	"github.com/substratusai/substratus/internal/cli/utils"
)

const (
	maxWidth = 80
	padding  = 2
)

// A model can be more or less any type of data. It holds all the data for a
// program, so often it's a struct. For this simple example, however, all
// we'll need is a simple integer.
type model struct {
	// Cancellation
	ctx context.Context

	// Config
	path          string
	namespace     string
	noOpenBrowser bool

	// Clients
	client   client.Interface
	resource *client.Resource

	// Current Notebook
	notebook *apiv1.Notebook

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

	// Ready to open browser
	localURL string

	// End times
	quitting   bool
	goodbye    string
	finalError error
}

type operation string

const (
	tarring        = operation("Tarring")
	applying       = operation("Applying")
	uploading      = operation("Uploading")
	waitingReady   = operation("WaitingReady")
	syncingFiles   = operation("SyncingFiles")
	portForwarding = operation("PortForwarding")
)

type status int

const (
	notStarted = status(0)
	inProgress = status(1)
	completed  = status(2)
)

func (m model) cleanupAndQuitCmd() tea.Msg {
	log.Println("Cleaning up")
	os.Remove(m.tarball.TempDir)
	return tea.Quit()
}

func (m model) Init() tea.Cmd {
	m.operations[tarring] = inProgress
	return prepareTarballCmd(m.ctx, m.path)
}

type (
	fileTarredMsg        string
	tarballCompleteMsg   *client.Tarball
	appliedWithUploadMsg struct {
		client.Object
	}
	tarballUploadedMsg struct {
		client.Object
	}
	uploadProgressMsg float64

	objectReadyMsg struct {
		client.Object
	}

	deletedMsg struct {
		error error
	}
	suspendedMsg struct {
		error error
	}

	fileSyncMsg struct {
		file     string
		complete bool
		error    error
	}

	portForwardReadyMsg struct{}

	operationMsg struct {
		operation
		status
	}

	localURLMsg string
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if m.quitting {
			switch msg.String() {
			case "esc":
				m.quitting = false
				return m, nil
			case "s":
				return m, suspendNotebookCmd(context.Background(), m.resource, m.notebook)
			case "d":
				return m, deleteNotebookCmd(context.Background(), m.resource, m.notebook)
			}
		} else {
			if msg.String() == "q" {
				m.quitting = true
				return m, nil
			}
			if m.operations[applying] == completed {
				if msg.String() == "a" {
					// TODO.
					return m, nil
				}
			}
		}

	case suspendedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Notebook suspended."
		}
		return m, m.cleanupAndQuitCmd

	case deletedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Notebook deleted."
		}
		return m, m.cleanupAndQuitCmd

	case fileTarredMsg:
		m.tarredFileCount++
		return m, nil

	case tarballCompleteMsg:
		m.operations[tarring] = completed
		m.tarball = msg
		m.operations[applying] = inProgress
		return m, applyWithUploadCmd(m.ctx, m.resource, m.notebook.DeepCopy(), m.tarball)

	case appliedWithUploadMsg:
		m.operations[applying] = completed
		m.notebook = msg.Object.(*apiv1.Notebook)
		log.Println("Hey")
		return m, uploadTarballCmd(m.ctx, m.resource, m.notebook.DeepCopy(), m.tarball)

	case uploadProgressMsg:
		m.operations[uploading] = inProgress
		return m, m.uploadProgress.SetPercent(float64(msg))

	case tarballUploadedMsg:
		m.operations[uploading] = completed
		m.notebook = msg.Object.(*apiv1.Notebook)
		m.operations[waitingReady] = inProgress
		return m, waitReadyCmd(m.ctx, m.resource, m.notebook.DeepCopy())

	case objectReadyMsg:
		m.operations[waitingReady] = completed
		m.notebook = msg.Object.(*apiv1.Notebook)
		m.operations[syncingFiles] = inProgress
		return m, tea.Batch(
			syncFilesCmd(m.ctx, m.client, m.notebook.DeepCopy(), m.path),
			portForwardNotebookCmd(m.ctx, m.client, m.notebook.DeepCopy()),
		)

	case fileSyncMsg:
		if msg.complete {
			m.currentSyncingFile = ""
		} else {
			m.currentSyncingFile = msg.file
		}
		if msg.error != nil {
			m.lastSyncFailure = msg.error
		} else {
			m.lastSyncFailure = nil
		}
		return m, nil

	case portForwardReadyMsg:
		return m, openNotebookInBrowser(m.notebook.DeepCopy())

	case localURLMsg:
		m.localURL = string(msg)
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
	}
	return m, nil
}

var (
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render
)

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m model) View() string {
	pad := strings.Repeat(" ", padding)
	v := "\n"

	if m.finalError != nil {
		v += pad + errorStyle("Error: "+m.finalError.Error()) + "\n\n"
		return v
	}

	if m.goodbye != "" {
		v += pad + m.goodbye + "\n\n"
		return v
	}

	if m.quitting {
		v += pad + "Quitting...\n"
		v += "\n" + pad + helpStyle("Press \"s\" to suspend, \"d\" to delete, \"ESC\" to cancel") + "\n"
		return v
	}

	if m.operations[tarring] == inProgress {
		v += pad + "Tarring...\n"
		v += pad + fmt.Sprintf("File count: %v\n", m.tarredFileCount)
	} else if m.operations[tarring] == completed {
		v += pad + "Tarring complete.\n"
	}

	if m.operations[applying] == inProgress {
		v += pad + "Applying...\n"
	} else if m.operations[applying] == completed {
		v += pad + "Notebook applied.\n"
	}

	if m.operations[uploading] == inProgress {
		v += pad + "Uploading...\n\n"
		v += pad + m.uploadProgress.View() + "\n\n"
	} else if m.operations[uploading] == completed {
		v += pad + "Upload complete.\n"
	}

	if m.operations[waitingReady] == inProgress {
		v += pad + "Waiting for notebook to be ready...\n"
	} else if m.operations[waitingReady] == completed {
		v += pad + "Notebook ready.\n"
	}

	if m.operations[syncingFiles] == inProgress {
		if m.currentSyncingFile != "" {
			v += pad + fmt.Sprintf("Syncing from notebook: %v\n", m.currentSyncingFile)
		} else {
			v += pad + "Watching for files to sync...\n"
		}
		if m.lastSyncFailure != nil {
			v += "\n"
			v += pad + errorStyle("Sync failed: "+m.lastSyncFailure.Error()) + "\n\n"
		}
	} else if m.operations[syncingFiles] == completed {
		v += pad + "Done syncing files.\n"
	}

	if m.operations[portForwarding] == inProgress {
		v += pad + "Port-forwarding...\n"
	} else if m.operations[portForwarding] == completed {
		v += pad + "Done port-forwarding.\n"
	}

	if m.localURL != "" {
		v += "\n"
		v += pad + fmt.Sprintf("Notebook URL: %v\n", m.localURL)
	}

	if m.localURL != "" {
		v += "\n" + pad + helpStyle("Press \"q\" to quit, \"a\" to apply") + "\n"
	} else {
		v += "\n" + pad + helpStyle("Press \"q\" to quit") + "\n"
	}

	return v
}

func prepareTarballCmd(ctx context.Context, dir string) tea.Cmd {
	return func() tea.Msg {
		log.Println("Preparing tarball")
		tarball, err := client.PrepareImageTarball(ctx, dir, func(file string) {
			log.Println("tarred", file)
			p.Send(fileTarredMsg(file))
		})
		if err != nil {
			log.Println("Error", err)
			return fmt.Errorf("preparing tarball: %w", err)
		}
		return tarballCompleteMsg(tarball)
	}
}

func applyWithUploadCmd(ctx context.Context, res *client.Resource, obj client.Object, tarball *client.Tarball) tea.Cmd {
	return func() tea.Msg {
		if err := client.ClearImage(obj); err != nil {
			return fmt.Errorf("clearing image in spec: %w", err)
		}
		if err := client.SetUploadContainerSpec(obj, tarball, utils.NewUUID()); err != nil {
			return fmt.Errorf("setting upload in spec: %w", err)
		}
		if err := res.Apply(obj, true); err != nil {
			return fmt.Errorf("applying: %w", err)
		}
		return appliedWithUploadMsg{Object: obj}
	}
}

func uploadTarballCmd(ctx context.Context, res *client.Resource, obj *apiv1.Notebook, tarball *client.Tarball) tea.Cmd {
	return func() tea.Msg {
		log.Println("Uploading tarball")
		err := res.Upload(ctx, obj, tarball, func(percentage float64) {
			log.Printf("Upload percentage: %v", percentage)
			p.Send(uploadProgressMsg(percentage))
		})
		if err != nil {
			log.Println("Upload failed", err)
			return fmt.Errorf("uploading: %w", err)
		}
		log.Println("Upload completed")
		return tarballUploadedMsg{Object: obj}
	}
}

func waitReadyCmd(ctx context.Context, res *client.Resource, obj client.Object) tea.Cmd {
	return func() tea.Msg {
		if err := res.WaitReady(ctx, obj); err != nil {
			return fmt.Errorf("waiting to be ready: %w", err)
		}
		return objectReadyMsg{Object: obj}
	}
}

func syncFilesCmd(ctx context.Context, c client.Interface, nb *apiv1.Notebook, dir string) tea.Cmd {
	return func() tea.Msg {
		if err := c.SyncFilesFromNotebook(ctx, nb, dir, func(file string, complete bool, syncErr error) {
			p.Send(fileSyncMsg{
				file:     file,
				complete: complete,
				error:    syncErr,
			})
		}); err != nil {
			if !errors.Is(err, context.Canceled) {
				return err
			}
		}
		return operationMsg{operation: syncingFiles, status: completed}
	}
}

func portForwardNotebookCmd(ctx context.Context, c client.Interface, nb *apiv1.Notebook) tea.Cmd {
	return func() tea.Msg {
		p.Send(operationMsg{operation: portForwarding, status: inProgress})
		defer p.Send(operationMsg{operation: portForwarding, status: completed})

		const maxRetries = 3
		for i := 0; i < maxRetries; i++ {
			portFwdCtx, cancelPortFwd := context.WithCancel(ctx)
			defer cancelPortFwd() // Avoid a context leak
			runtime.ErrorHandlers = []func(err error){
				func(err error) {
					// Cancel a broken port forward to attempt to restart the port-forward.
					log.Printf("Port-forward error: %v", err)
					cancelPortFwd()
				},
			}

			// portForward will close the ready channel when it returns.
			// so we only use the outer ready channel once. On restart of the portForward,
			// we use a new channel.
			ready := make(chan struct{})
			go func() {
				log.Println("Waiting for port-forward to be ready")
				<-ready
				log.Println("Port-forward ready")
				p.Send(portForwardReadyMsg{})
			}()

			if err := c.PortForwardNotebook(portFwdCtx, false, nb, ready); err != nil {
				log.Printf("Port-forward returned an error: %v", err)
			}

			// Check if the command's context is cancelled, if so,
			// avoid restarting the port forward.
			if err := ctx.Err(); err != nil {
				log.Printf("Context done, not attempting to restart port-forward: %v", err.Error())
				return nil
			}

			cancelPortFwd() // Avoid a build up of contexts before returning.
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			log.Printf("Restarting port forward (index = %v), after backoff: %s", i, backoff)
			time.Sleep(backoff)
		}
		log.Println("Done trying to port-forward")

		return operationMsg{operation: portForwarding, status: completed}
	}
}

func openNotebookInBrowser(nb *apiv1.Notebook) tea.Cmd {
	return func() tea.Msg {
		// TODO(nstogner): Grab token from Notebook status.
		url := "http://localhost:8888?token=default"
		log.Printf("Opening browser to %s\n", url)
		browser.OpenURL(url)
		return localURLMsg(url)
	}
}

func suspendNotebookCmd(ctx context.Context, res *client.Resource, nb *apiv1.Notebook) tea.Cmd {
	return func() tea.Msg {
		log.Println("Suspending notebook")
		_, err := res.Patch(nb.Namespace, nb.Name, types.MergePatchType, []byte(`{"spec": {"suspend": true} }`), &metav1.PatchOptions{})
		if err != nil {
			log.Printf("Error suspending notebook: %v", err)
			return suspendedMsg{error: err}
		}
		return suspendedMsg{}
	}
}

func deleteNotebookCmd(ctx context.Context, res *client.Resource, nb *apiv1.Notebook) tea.Cmd {
	return func() tea.Msg {
		log.Println("Deleting notebook")
		_, err := res.Delete(nb.Namespace, nb.Name)
		if err != nil {
			log.Printf("Error deleting notebook: %v", err)
			return deletedMsg{error: err}
		}
		return deletedMsg{}
	}
}
