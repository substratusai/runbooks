package tui

import (
	"context"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/substratusai/substratus/internal/client"
)

type applyObjectKey struct {
	schema.GroupVersionKind
	types.NamespacedName
}

type applyObject struct {
	object  client.Object
	status  status
	error   error
	spinner spinner.Model
}

type ApplyModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Namespace     Namespace
	Filename      string
	NoOpenBrowser bool

	// Clients
	Client client.Interface
	K8s    *kubernetes.Clientset

	objects []applyObject

	applying status

	Style lipgloss.Style

	// End times
	quitting   bool
	finalError error
}

func (m *ApplyModel) New() ApplyModel {
	m.Style = appStyle

	return *m
}

func (m ApplyModel) Init() tea.Cmd {
	return findManifests(m.Filename, false)
}

func (m ApplyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("MSG: %T", msg)

	var cmds []tea.Cmd

	apply := func(o client.Object, idx int) {
		res, err := m.Client.Resource(o)
		if err != nil {
			m.finalError = fmt.Errorf("resource client: %w", err)
			return
		}

		cmds = append(cmds, applyCmd(m.Ctx, res, &applyInput{
			Object: o.DeepCopyObject().(client.Object),
			index:  idx,
		}))
	}
	switch msg := msg.(type) {
	case manifestsFoundMsg:
		m.applying = inProgress
		m.objects = []applyObject{}
		for _, o := range msg.manifests {
			o = o.DeepCopyObject().(client.Object)
			m.Namespace.Set(o)
			s := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(activeSpinnerStyle))
			m.objects = append(m.objects, applyObject{
				object:  o,
				status:  inProgress,
				spinner: s,
			})
			cmds = append(cmds, s.Tick)
			apply(o, 0)
		}
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		for k, o := range m.objects {
			if o.spinner.ID() == msg.ID {
				var cmd tea.Cmd
				o.spinner, cmd = o.spinner.Update(msg)
				m.objects[k] = o
				return m, cmd
			}
		}

	case appliedMsg:
		ao := m.objects[msg.index]
		ao.status = completed
		ao.error = msg.err
		m.objects[msg.index] = ao

		if msg.index == len(m.objects)-1 {
			m.applying = completed
			return m, tea.Quit
		}
		apply(m.objects[msg.index+1].object, msg.index+1)
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if msg.String() == "q" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.Style.Width(msg.Width)

	case error:
		log.Printf("Error message: %v", msg)
		m.finalError = msg
		return m, tea.Quit
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m ApplyModel) View() (v string) {
	defer func() {
		v = m.Style.Render(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Width(m.Style.GetWidth()-m.Style.GetHorizontalMargins()-10).Render("Error: "+m.finalError.Error()) + "\n"
		return
	}

	for _, o := range m.objects {
		var indicator string
		if o.status != completed {
			indicator = o.spinner.View()
		} else {
			if o.error != nil {
				indicator = xMark.String()
			} else {
				indicator = checkMark.String()
			}
		}
		gvk := o.object.GetObjectKind().GroupVersionKind()
		v += fmt.Sprintf("%s %v: %v\n",
			indicator, gvk.Kind,
			o.object.GetName(),
		)
	}

	if m.applying == inProgress {
		v += "\nApplying...\n"
		v += helpStyle("Press \"q\" to quit")
	}

	return v
}
