package tui

import (
	"context"
	"errors"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/client"
)

type NotebookModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Path          string
	Namespace     Namespace
	NoOpenBrowser bool

	// Clients
	Client client.Interface
	K8s    *kubernetes.Clientset

	// Current notebook
	notebook *apiv1.Notebook
	resource *client.Resource

	// Proceses
	upload    uploadModel
	readiness readinessModel
	pods      podsModel

	// File syncing
	syncingFiles       status
	currentSyncingFile string
	lastSyncFailure    error

	// Ready to open browser
	portForwarding status
	localURL       string

	// End times
	quitting   bool
	goodbye    string
	finalError error

	Style lipgloss.Style
}

func (m NotebookModel) cleanupAndQuitCmd() tea.Msg {
	m.upload.cleanup()
	return tea.Quit()
}

func (m *NotebookModel) New() NotebookModel {
	m.upload = (&uploadModel{
		Ctx:    m.Ctx,
		Client: m.Client,
		Path:   m.Path,
		Mode:   uploadModeApply,
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

func (m NotebookModel) Init() tea.Cmd {
	return readManifest(m.Ctx, m.Client, m.Path, m.Namespace)
}

func (m NotebookModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	log.Printf("MSG: %T", msg)
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
	case readManifestMsg:
		nb, err := client.NotebookForObject(msg.obj)
		if err != nil {
			m.finalError = fmt.Errorf("determining notebook: %w", err)
		}
		nb.Spec.Suspend = ptr.To(false)
		m.notebook = nb

		m.resource = msg.res
		m.upload.Object = m.notebook
		m.upload.Resource = m.resource
		cmds = append(cmds, m.upload.Init())

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
				cmds = append(cmds, suspendCmd(context.Background(), m.resource, m.notebook))
			case "d":
				cmds = append(cmds, deleteCmd(context.Background(), m.resource, m.notebook))
			}
		} else {
			if msg.String() == "q" {
				m.quitting = true
			}
		}

	case suspendedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Notebook suspended."
		}
		cmds = append(cmds, m.cleanupAndQuitCmd)

	case deletedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.goodbye = "Notebook deleted."
		}
		cmds = append(cmds, m.cleanupAndQuitCmd)

	case tarballUploadedMsg:
		m.notebook = msg.Object.(*apiv1.Notebook)

		m.readiness.Object = m.notebook
		m.readiness.Resource = m.resource
		m.pods.Object = m.notebook
		m.pods.Resource = m.resource
		cmds = append(cmds,
			m.readiness.Init(),
			m.pods.Init(),
		)

	case objectReadyMsg:
		m.notebook = msg.Object.(*apiv1.Notebook)
		m.syncingFiles = inProgress
		cmds = append(cmds,
			notebookSyncFilesCmd(m.Ctx, m.Client, m.notebook.DeepCopy(), m.Path),
			portForwardCmd(m.Ctx, m.Client, client.PodForNotebook(m.notebook)),
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

	case portForwardReadyMsg:
		cmds = append(cmds, notebookOpenInBrowser(m.notebook.DeepCopy()))

	case localURLMsg:
		m.localURL = string(msg)

	case tea.WindowSizeMsg:
		m.Style.Width(msg.Width)
		innerWidth := m.Style.GetWidth() - m.Style.GetHorizontalPadding()
		m.upload.Style = lipgloss.NewStyle().Width(innerWidth)
		m.readiness.Style = lipgloss.NewStyle().Width(innerWidth)
		m.pods.SetStyle(logStyle.Width(innerWidth))

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
		v = m.Style.Render(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Width(m.Style.GetWidth()-m.Style.GetHorizontalMargins()-10).Render("Error: "+m.finalError.Error()) + "\n"
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
	v += m.readiness.View()
	v += m.pods.View()

	if m.syncingFiles == inProgress {
		v += "\n"
		if m.currentSyncingFile != "" {
			v += fmt.Sprintf("Syncing from notebook: %v\n", m.currentSyncingFile)
		} else {
			v += "Watching for files to sync...\n"
		}
		if m.lastSyncFailure != nil {
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
		return nil
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
