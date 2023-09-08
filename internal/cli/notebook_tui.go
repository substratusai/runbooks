package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cli/client"
)

// A notebookModel can be more or less any type of data. It holds all the data for a
// program, so often it's a struct. For this simple example, however, all
// we'll need is a simple integer.
type notebookModel struct {
	// Cancellation
	ctx context.Context

	// Config
	path          string
	namespace     string
	noOpenBrowser bool

	// Clients
	client   client.Interface
	resource *client.Resource
	k8s      *kubernetes.Clientset

	// Original Object (could be a Dataset, Model, or Server)
	object client.Object

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

	// Watch Pods
	pods map[string]podWatchMsg

	// End times
	quitting   bool
	goodbye    string
	finalError error
}

type operation string

const (
	tarring        = operation("Tarring")
	applying       = operation("Applying")
	creating       = operation("Creating")
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

func (m notebookModel) cleanupAndQuitCmd() tea.Msg {
	log.Println("Cleaning up")
	os.Remove(m.tarball.TempDir)
	return tea.Quit()
}

func (m notebookModel) Init() tea.Cmd {
	log.Println("Init")
	m.operations[tarring] = inProgress
	return prepareTarballCmd(m.ctx, m.path)
}

type (
	operationMsg struct {
		operation
		status
	}

	localURLMsg string
)

func (m notebookModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if m.quitting {
			switch msg.String() {
			case "esc":
				m.quitting = false
				return m, nil
			case "s":
				return m, notebookSuspendCmd(context.Background(), m.resource, m.notebook)
			case "d":
				return m, notebookDeleteCmd(context.Background(), m.resource, m.notebook)
			}
		} else {
			if msg.String() == "q" {
				m.quitting = true
				return m, nil
			}
		}

	case notebookSuspendedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Notebook suspended."
		}
		return m, m.cleanupAndQuitCmd

	case notebookDeletedMsg:
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
		m.operations[uploading] = inProgress
		return m, uploadTarballCmd(m.ctx, m.resource, m.notebook.DeepCopy(), m.tarball)

	case uploadTarballProgressMsg:
		return m, m.uploadProgress.SetPercent(float64(msg))

	case tarballUploadedMsg:
		m.operations[uploading] = completed
		m.notebook = msg.Object.(*apiv1.Notebook)
		m.operations[waitingReady] = inProgress
		return m, tea.Batch(
			watchPods(m.ctx, m.client, m.notebook.DeepCopy()),
			waitReadyCmd(m.ctx, m.resource, m.notebook.DeepCopy()),
		)

	case podWatchMsg:
		m.pods[msg.Pod.Name] = msg
		return m, nil

	case objectReadyMsg:
		m.operations[waitingReady] = completed
		m.notebook = msg.Object.(*apiv1.Notebook)
		m.operations[syncingFiles] = inProgress
		return m, tea.Batch(
			notebookSyncFilesCmd(m.ctx, m.client, m.notebook.DeepCopy(), m.path),
			noteookPortForwardCmd(m.ctx, m.client, m.notebook.DeepCopy()),
		)

	case notebookFileSyncMsg:
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

	case notebookPortForwardReadyMsg:
		return m, notebookOpenInBrowser(m.notebook.DeepCopy())

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

	case error:
		m.finalError = msg
		m.quitting = true
		return m, nil
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m notebookModel) View() string {
	pad := strings.Repeat(" ", padding)
	v := "\n"

	if m.finalError != nil {
		v += pad + errorStyle("Error: "+m.finalError.Error()) + "\n"
		v += "\n" + pad + helpStyle("Press \"q\" to quit") + "\n\n"
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

	var podNames []string
	for name := range m.pods {
		podNames = append(podNames, name)
	}
	sort.Strings(podNames)
	for _, name := range podNames {
		p := m.pods[name]
		if p.Type == watch.Deleted {
			continue
		}
		v += pad + "> " + p.Pod.Labels["role"] + ": " + string(p.Pod.Status.Phase) + "\n"
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

	v += "\n" + pad + helpStyle("Press \"q\" to quit") + "\n"

	return v
}

type notebookFileSyncMsg struct {
	file     string
	complete bool
	error    error
}

func notebookSyncFilesCmd(ctx context.Context, c client.Interface, nb *apiv1.Notebook, dir string) tea.Cmd {
	return func() tea.Msg {
		if err := c.SyncFilesFromNotebook(ctx, nb, dir, func(file string, complete bool, syncErr error) {
			p.Send(notebookFileSyncMsg{
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

type notebookPortForwardReadyMsg struct{}

func noteookPortForwardCmd(ctx context.Context, c client.Interface, nb *apiv1.Notebook) tea.Cmd {
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
				p.Send(notebookPortForwardReadyMsg{})
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

func notebookOpenInBrowser(nb *apiv1.Notebook) tea.Cmd {
	return func() tea.Msg {
		// TODO(nstogner): Grab token from Notebook status.
		url := "http://localhost:8888?token=default"
		log.Printf("Opening browser to %s\n", url)
		browser.OpenURL(url)
		return localURLMsg(url)
	}
}

type notebookSuspendedMsg struct {
	error error
}

func notebookSuspendCmd(ctx context.Context, res *client.Resource, nb *apiv1.Notebook) tea.Cmd {
	return func() tea.Msg {
		log.Println("Suspending notebook")
		_, err := res.Patch(nb.Namespace, nb.Name, types.MergePatchType, []byte(`{"spec": {"suspend": true} }`), &metav1.PatchOptions{})
		if err != nil {
			log.Printf("Error suspending notebook: %v", err)
			return notebookSuspendedMsg{error: err}
		}
		return notebookSuspendedMsg{}
	}
}

type notebookDeletedMsg struct {
	error error
}

func notebookDeleteCmd(ctx context.Context, res *client.Resource, nb *apiv1.Notebook) tea.Cmd {
	return func() tea.Msg {
		log.Println("Deleting notebook")
		_, err := res.Delete(nb.Namespace, nb.Name)
		if err != nil {
			log.Printf("Error deleting notebook: %v", err)
			return notebookDeletedMsg{error: err}
		}
		return notebookDeletedMsg{}
	}
}
