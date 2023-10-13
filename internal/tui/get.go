package tui

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/client"
)

type GetModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Scope     string
	Namespace string

	// Clients
	Client client.Interface

	// End times
	finalError error

	objects map[string]map[string]listedObject

	Style lipgloss.Style
}

type listedObject struct {
	object
	spinner spinner.Model
}

func newGetObjectMap() map[string]map[string]listedObject {
	return map[string]map[string]listedObject{
		"notebooks": {},
		"datasets":  {},
		"models":    {},
		"servers":   {},
	}
}

func (m *GetModel) New() GetModel {
	m.objects = newGetObjectMap()
	m.Style = appStyle
	return *m
}

func (m GetModel) Init() tea.Cmd {
	return watchCmd(m.Ctx, m.Client, m.Namespace, m.Scope)
}

func (m GetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if msg.String() == "q" {
			return m, tea.Quit
		}

	case watchMsg:
		var cmd tea.Cmd
		switch msg.Type {
		case watch.Deleted:
			delete(m.objects[msg.resource], msg.Object.(object).GetName())
		case watch.Error:
			log.Printf("Watch error: %v", msg.Object)
		default:
			o := msg.Object.(client.Object)
			name := o.GetName()
			log.Printf("Watch event: %v: %v", msg.resource, name)

			lo := m.objects[msg.resource][name]
			lo.object = msg.Object.(object)
			if msg.Type == watch.Added {
				lo.spinner = spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(activeSpinnerStyle))
				cmd = lo.spinner.Tick
			}
			m.objects[msg.resource][name] = lo
		}
		return m, cmd

	case spinner.TickMsg:
		for kind := range m.objects {
			for name := range m.objects[kind] {
				o := m.objects[kind][name]
				var cmd tea.Cmd
				if o.spinner.ID() == msg.ID {
					o.spinner, cmd = o.spinner.Update(msg)
					m.objects[kind][name] = o
					return m, cmd
				}
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.Style.Width(msg.Width)

	case error:
		m.finalError = msg
		return m, nil
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m GetModel) View() (v string) {
	defer func() {
		v = m.Style.Render(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Render("Error: "+m.finalError.Error()) + "\n"
		v += helpStyle("Press \"q\" to quit")
		return v
	}

	scopeResource, scopeName := splitScope(m.Scope)

	var total int
	for _, resource := range []string{
		"notebooks",
		"datasets",
		"models",
		"servers",
	} {
		if len(m.objects[resource]) == 0 {
			continue
		}

		if scopeResource == "" {
			v += resource + "/" + "\n"
		}

		var names []string
		for name := range m.objects[resource] {
			names = append(names, name)
			total++
		}
		sort.Strings(names)

		for _, name := range names {
			o := m.objects[resource][name]

			var indicator string
			if o.GetStatusReady() {
				indicator = checkMark.String()
			} else {
				indicator = o.spinner.View()
			}
			v += "" + indicator + " " + name + "\n"
		}
		v += "\n"
	}

	if scopeName == "" {
		v += fmt.Sprintf("\nTotal: %v\n", total)
	}

	v += helpStyle("Press \"q\" to quit")

	return v
}

type watchMsg struct {
	watch.Event
	resource string
}

type object interface {
	client.Object
	GetConditions() *[]metav1.Condition
	GetStatusReady() bool
}

func watchCmd(ctx context.Context, c client.Interface, namespace, scope string) tea.Cmd {
	pluralName := func(s string) string {
		return strings.ToLower(s) + "s"
	}

	return func() tea.Msg {
		log.Println("Starting watch")

		objs, err := scopeToObjects(scope)
		if err != nil {
			return fmt.Errorf("parsing search term: %v", err)
		}

		for _, obj := range objs {
			res, err := c.Resource(obj)
			if err != nil {
				return fmt.Errorf("resource client: %w", err)
			}

			kind := obj.GetObjectKind().GroupVersionKind().Kind
			log.Printf("Starting watch: %v", kind)

			w, err := res.Watch(ctx, namespace, obj, &metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("watch: %w", err)
			}
			go func() {
				for event := range w.ResultChan() {
					P.Send(watchMsg{Event: event, resource: pluralName(kind)})
				}
			}()
		}

		return nil
	}
}

// scopeToObjects maps a scope string to a slice of objects.
// "" --> All Substratus kinds
// "models" --> All Models
// "models/<name>" --> Single Model
func scopeToObjects(scope string) ([]client.Object, error) {
	if scope == "" {
		return []client.Object{
			&apiv1.Notebook{TypeMeta: metav1.TypeMeta{APIVersion: "substratus.ai/v1", Kind: "Notebook"}},
			&apiv1.Dataset{TypeMeta: metav1.TypeMeta{APIVersion: "substratus.ai/v1", Kind: "Dataset"}},
			&apiv1.Model{TypeMeta: metav1.TypeMeta{APIVersion: "substratus.ai/v1", Kind: "Model"}},
			&apiv1.Server{TypeMeta: metav1.TypeMeta{APIVersion: "substratus.ai/v1", Kind: "Server"}},
		}, nil
	}

	singleObj, err := scopeToObject(scope)
	if err != nil {
		return nil, err
	}

	return []client.Object{singleObj}, nil
}

func scopeToObject(scope string) (client.Object, error) {
	res, name := splitScope(scope)
	if res == "" && name == "" {
		return nil, fmt.Errorf("Invalid scope: %v", scope)
	}

	var obj client.Object
	switch res {
	case "notebooks":
		obj = &apiv1.Notebook{TypeMeta: metav1.TypeMeta{APIVersion: "substratus.ai/v1", Kind: "Notebook"}}
	case "datasets":
		obj = &apiv1.Dataset{TypeMeta: metav1.TypeMeta{APIVersion: "substratus.ai/v1", Kind: "Dataset"}}
	case "models":
		obj = &apiv1.Model{TypeMeta: metav1.TypeMeta{APIVersion: "substratus.ai/v1", Kind: "Model"}}
	case "servers":
		obj = &apiv1.Server{TypeMeta: metav1.TypeMeta{APIVersion: "substratus.ai/v1", Kind: "Server"}}
	default:
		return nil, fmt.Errorf("Invalid scope: %v", scope)
	}

	if name != "" {
		obj.SetName(name)
	}

	return obj, nil
}

func splitScope(scope string) (string, string) {
	split := strings.Split(scope, "/")
	if len(split) == 1 {
		return split[0], ""
	}
	if len(split) == 2 {
		return split[0], split[1]
	}
	return "", ""
}
