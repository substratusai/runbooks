package cli

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cli/client"
)

type lsModel struct {
	// Cancellation
	ctx context.Context

	// Config
	scope     string
	namespace string

	// Clients
	client client.Interface

	// End times
	finalError error

	objects map[string]map[string]listedObject
}

type listedObject struct {
	object
	spinner spinner.Model
}

func newListObjectMap() map[string]map[string]listedObject {
	return map[string]map[string]listedObject{
		"notebooks": {},
		"datasets":  {},
		"models":    {},
		"servers":   {},
	}
}

func (m lsModel) Init() tea.Cmd {
	return watchCmd(m.ctx, m.client, m.namespace, m.scope)
}

func (m lsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case error:
		m.finalError = msg
		return m, nil
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m lsModel) View() (v string) {
	defer func() {
		v = appStyle(v)
	}()

	if m.finalError != nil {
		v += errorStyle("Error: "+m.finalError.Error()) + "\n"
		v += helpStyle("Press \"q\" to quit")
		return v
	}

	var total int
	for _, resource := range []string{"notebooks", "datasets", "models", "servers"} {
		if len(m.objects[resource]) == 0 {
			continue
		}

		v += resource + "/" + "\n"

		var names []string
		for name := range m.objects[resource] {
			names = append(names, name)
		}
		sort.Strings(names)

		type objectVersions struct {
			unversionedName string
			versions        []listedObject
		}

		var groups []objectVersions

		var lastUnversionedName string
		for _, name := range names {
			o := m.objects[resource][name]
			lowerKind := strings.TrimSuffix(resource, "s")
			unversionedName := o.GetLabels()[lowerKind]

			if unversionedName != lastUnversionedName {
				groups = append(groups, objectVersions{
					unversionedName: unversionedName,
					versions:        []listedObject{o},
				})
			} else {
				groups[len(groups)-1].versions = append(groups[len(groups)-1].versions, o)
			}

			lastUnversionedName = unversionedName
			total++
		}

		for _, g := range groups {
			// TODO: Table view
			v += "  " + g.unversionedName + "   "

			var displayVersions []string
			for _, o := range g.versions {
				version := o.GetLabels()["version"]

				var indicator string
				if o.GetStatusReady() {
					indicator = checkMark.String()
				} else if c := meta.FindStatusCondition(*o.GetConditions(), apiv1.ConditionComplete); c != nil && c.Reason == apiv1.ReasonJobFailed {
					indicator = xMark.String()
				} else {
					indicator = o.spinner.View()
				}
				displayVersions = append(displayVersions, fmt.Sprintf("%v v%v", indicator, version))
			}
			slices.Reverse(displayVersions)
			v += strings.Join(displayVersions, "  ") + "\n"
		}

		v += "\n"
	}
	v += fmt.Sprintf("Total: %v", total) + "\n"

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
					p.Send(watchMsg{Event: event, resource: pluralName(kind)})
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

	split := strings.Split(scope, "/")
	var obj client.Object
	switch split[0] {
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

	if len(split) == 2 {
		obj.SetName(split[1])
	} else if len(split) > 2 {
		return nil, fmt.Errorf("Invalid scope: %v", scope)
	}

	return []client.Object{obj}, nil
}
