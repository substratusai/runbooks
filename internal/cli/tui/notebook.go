package tui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cli/client"
)

// A NotebookModel can be more or less any type of data. It holds all the data for a
// program, so often it's a struct. For this simple example, however, all
// we'll need is a simple integer.
type NotebookModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Path          string
	Namespace     string
	NoOpenBrowser bool

	// Clients
	Client   client.Interface
	Resource *client.Resource
	K8s      *kubernetes.Clientset

	// Original Object (could be a Dataset, Model, or Server)
	// Object client.Object

	// Current Notebook
	Notebook *apiv1.Notebook

	upload    uploadModel
	readiness readinessModel

	pods podsModel

	// File syncing
	syncingFiles       status
	currentSyncingFile string
	lastSyncFailure    error

	// Ready to open browser
	portForwarding status
	localURL       string

	width int

	// End times
	quitting   bool
	goodbye    string
	finalError error
}

func (m NotebookModel) cleanupAndQuitCmd() tea.Msg {
	m.upload.cleanup()
	return tea.Quit()
}

func (m *NotebookModel) New() NotebookModel {
	m.upload = (&uploadModel{
		Ctx:       m.Ctx,
		Client:    m.Client,
		Resource:  m.Resource,
		Object:    m.Notebook,
		Path:      m.Path,
		Namespace: m.Namespace,
		Mode:      uploadModeApply,
	}).New()
	m.readiness = (&readinessModel{
		Ctx:      m.Ctx,
		Object:   m.Notebook,
		Client:   m.Client,
		Resource: m.Resource,
	}).New()
	m.pods = (&podsModel{
		Ctx:      m.Ctx,
		Client:   m.Client,
		Resource: m.Resource,
		K8s:      m.K8s,
		Object:   m.Notebook,
	}).New()

	return *m
}

func (m NotebookModel) Init() tea.Cmd {
	return m.upload.Init()
}

type (
	operationMsg struct {
		operation
		status
	}

	localURLMsg string
)

func (m NotebookModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	if m.pods.watchingPods != notStarted {
		mdl, cmd := m.pods.Update(msg)
		m.pods = mdl.(podsModel)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if m.quitting {
			switch msg.String() {
			case "esc":
				if m.finalError == nil {
					m.quitting = false
				}

			// "Leave be" results in issues where a build will eventually replace the Notebook
			//  and the command will error out due to a failure on the previous notebook nbwatch
			//  command... revisit later.
			//
			// case "l":
			//	cmds = append(cmds, m.cleanupAndQuitCmd)
			case "s":
				cmds = append(cmds, notebookSuspendCmd(context.Background(), m.Resource, m.Notebook))
			case "d":
				cmds = append(cmds, notebookDeleteCmd(context.Background(), m.Resource, m.Notebook))
			}
		} else {
			if msg.String() == "q" {
				m.quitting = true
			}
		}

	case notebookSuspendedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Notebook suspended."
		}
		cmds = append(cmds, m.cleanupAndQuitCmd)

	case notebookDeletedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Notebook deleted."
		}
		cmds = append(cmds, m.cleanupAndQuitCmd)

	case tarballUploadedMsg:
		m.Notebook = msg.Object.(*apiv1.Notebook)
		m.readiness.Object = msg.Object
		m.pods.Object = msg.Object

		cmds = append(cmds,
			m.readiness.Init(),
			m.pods.Init(),
		)

	case objectReadyMsg:
		m.Notebook = msg.Object.(*apiv1.Notebook)
		m.syncingFiles = inProgress
		cmds = append(cmds,
			notebookSyncFilesCmd(m.Ctx, m.Client, m.Notebook.DeepCopy(), m.Path),
			noteookPortForwardCmd(m.Ctx, m.Client, m.Notebook.DeepCopy()),
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

	case notebookPortForwardReadyMsg:
		cmds = append(cmds, notebookOpenInBrowser(m.Notebook.DeepCopy()))

	case localURLMsg:
		m.localURL = string(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case error:
		log.Printf("Error message: %v", msg)
		m.finalError = msg
		m.quitting = true
	}

	return m, tea.Batch(cmds...)
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m NotebookModel) View() (v string) {
	defer func() {
		v = appStyle(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Width(m.width-10).Render("Error: "+m.finalError.Error()) + "\n"
		v += helpStyle("Press \"s\" to suspend, \"d\" to delete")
		// v += helpStyle("Press \"l\" to leave be, \"s\" to suspend, \"d\" to delete")
		return v
	}

	if m.goodbye != "" {
		v += m.goodbye + "\n"
		return v
	}

	if m.quitting {
		v += "Quitting...\n"
		v += helpStyle("Press \"s\" to suspend, \"d\" to delete, \"ESC\" to cancel")
		// v += helpStyle("Press \"l\" to leave be, \"s\" to suspend, \"d\" to delete, \"ESC\" to cancel")
		return v
	}

	v += m.upload.View()
	if m.readiness.waiting != notStarted {
		v += m.readiness.View()
	}
	if m.pods.watchingPods != notStarted {
		v += m.pods.View()
		v += "\n"
	}

	if m.syncingFiles == inProgress {
		if m.currentSyncingFile != "" {
			v += fmt.Sprintf("Syncing from notebook: %v\n", m.currentSyncingFile)
		} else {
			v += "Watching for files to sync...\n"
		}
		if m.lastSyncFailure != nil {
			v += "\n"
			v += errorStyle.Render("Sync failed: "+m.lastSyncFailure.Error()) + "\n\n"
		}
	}

	if m.portForwarding == inProgress {
		v += "Port-forwarding...\n"
	}

	if m.localURL != "" && m.portForwarding == inProgress {
		v += "\n"
		v += fmt.Sprintf("Notebook URL: %v\n", m.localURL)
	}

	v += helpStyle("Press \"q\" to quit")

	return v
}

type notebookFileSyncMsg struct {
	file     string
	complete bool
	error    error
}

func notebookSyncFilesCmd(ctx context.Context, c client.Interface, nb *apiv1.Notebook, dir string) tea.Cmd {
	return func() tea.Msg {
		if err := c.SyncFilesFromNotebook(ctx, nb, dir, LogFile, func(file string, complete bool, syncErr error) {
			P.Send(notebookFileSyncMsg{
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
		P.Send(operationMsg{operation: portForwarding, status: inProgress})
		defer P.Send(operationMsg{operation: portForwarding, status: completed})

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
				P.Send(notebookPortForwardReadyMsg{})
			}()

			if err := c.PortForwardNotebook(portFwdCtx, LogFile, nb, ready); err != nil {
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
