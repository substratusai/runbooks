package cli

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

	objects map[string]map[string]object
}

func newListObjectMap() map[string]map[string]object {
	return map[string]map[string]object{
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
		switch msg.Type {
		case watch.Deleted:
			delete(m.objects[msg.resource], msg.Object.(object).GetName())
		case watch.Error:
			log.Printf("Watch error: %v", msg.Object)
		default:
			o := msg.Object.(client.Object)
			name := o.GetName()
			log.Printf("Watch event: %v: %v", msg.resource, name)
			m.objects[msg.resource][name] = msg.Object.(object)
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
func (m lsModel) View() string {
	pad := strings.Repeat(" ", padding)
	v := "\n"

	if m.finalError != nil {
		v += pad + errorStyle("Error: "+m.finalError.Error()) + "\n"
		v += "\n" + pad + helpStyle("Press \"q\" to quit") + "\n\n"
		return v
	}

	var total int
	for _, resource := range []string{"notebooks", "datasets", "models", "servers"} {
		if len(m.objects[resource]) == 0 {
			continue
		}

		v += pad + resource + "/" + "\n"

		nameMap := m.objects[resource]
		var names []string
		for name := range nameMap {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			o := m.objects[resource][name]
			v += pad + pad
			if o.GetStatusReady() {
				v += checkMark.String()
			} else {
				// TODO: Spinner for in-progress, X for failure
				v += xMark.String()
			}
			v += " " + name
			v += "\n"
			total++
		}

		v += "\n"
	}
	v += pad + fmt.Sprintf("Total: %v", total) + "\n"

	v += "\n" + pad + helpStyle("Press \"q\" to quit") + "\n"

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
