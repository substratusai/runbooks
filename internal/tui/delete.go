package tui

import (
	"context"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/substratusai/substratus/internal/client"
)

type DeleteModel struct {
	// Cancellation
	Ctx context.Context

	// Config
	Scope     string
	Namespace Namespace

	// Clients
	Client client.Interface

	resource *client.Resource
	toDelete []client.Object
	deleted  []string

	Style lipgloss.Style

	// End times
	goodbye    string
	finalError error
}

func (m *DeleteModel) New() DeleteModel {
	m.Style = appStyle
	if m.toDelete == nil {
		m.toDelete = []client.Object{}
	}
	return *m
}

type deleteInitMsg struct{}

func (m DeleteModel) Init() tea.Cmd {
	//if len(m.Objects) == 0 {
	//	return listCmd(m.Ctx, m.Resource, m.Scope)
	//} else {
	//	cmds := make([]tea.Cmd, 0, len(m.Objects))
	//	for _, obj := range m.Objects {
	//		cmds = append(cmds, deleteCmd(m.Ctx, m.Resource, obj))
	//	}
	//	return tea.Batch(cmds...)
	//}

	return func() tea.Msg { return deleteInitMsg{} }
}

func (m DeleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case deleteInitMsg:
		log.Println("running init")
		obj, err := scopeToObject(m.Scope)
		if err != nil {
			m.finalError = fmt.Errorf("scope to object: %w", err)
			return m, nil
		}
		m.Namespace.Set(obj)

		res, err := m.Client.Resource(obj)
		if err != nil {
			m.finalError = fmt.Errorf("resource client: %w", err)
			return m, nil
		}
		m.resource = res

		return m, getDeletionTargetsCmd(m.Ctx, m.resource, obj)

	case tea.KeyMsg:
		log.Println("Received key msg:", msg.String())
		if msg.String() == "q" {
			return m, tea.Quit
		}

	case deletionListMsg:
		var cmds []tea.Cmd
		m.toDelete = msg.items
		for _, obj := range msg.items {
			log.Printf("to-delete: %v", obj.GetName())
			// TODO: Implement a confirmation flow.
			cmds = append(cmds, deleteCmd(m.Ctx, m.resource, obj))
		}
		return m, tea.Sequence(cmds...)

	case deletedMsg:
		if msg.error != nil {
			m.finalError = msg.error
		} else {
			m.deleted = append(m.deleted, msg.name)
		}
		if len(m.deleted) == len(m.toDelete) {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.Style.Width(msg.Width)
	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m DeleteModel) View() (v string) {
	defer func() {
		v += helpStyle("Press \"q\" to quit")
		v = m.Style.Render(v)
	}()

	if m.finalError != nil {
		v += errorStyle.Width(m.Style.GetWidth()-m.Style.GetHorizontalMargins()-10).Render("Error: "+m.finalError.Error()) + "\n"
		return
	}

	if m.goodbye != "" {
		v += m.goodbye + "\n"
		return
	}

	for _, name := range m.deleted {
		v += checkMark.String() + " " + name + ": deleted\n"
	}

	return
}

type deletionListMsg struct {
	items []client.Object
}

func getDeletionTargetsCmd(ctx context.Context, res *client.Resource, obj client.Object) tea.Cmd {
	log.Printf("getting deletion targets: %v/%v", obj.GetNamespace(), obj.GetName())
	return func() tea.Msg {
		if obj.GetName() != "" {
			fetched, err := res.Get(obj.GetNamespace(), obj.GetName())
			if err != nil {
				return fmt.Errorf("get: %w", err)
			}
			return deletionListMsg{items: []client.Object{fetched.(client.Object)}}
		} else {
			items, err := res.List(obj.GetNamespace(), "", &metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("list: %w", err)
			}
			log.Printf("%T", items)
			panic("NOT IMPLEMENTED")
		}
	}
}
