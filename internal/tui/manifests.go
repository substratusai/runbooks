package tui

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/client"
)

type manifestsModel struct {
	Path     string
	Filename string

	// Kinds is a list of manifest kinds to include in results,
	// ordered by preference.
	Kinds []string

	reading status

	Style lipgloss.Style
}

// New initializes all internal fields.
func (m *manifestsModel) New() manifestsModel {
	return *m
}

func (m manifestsModel) Active() bool {
	return m.reading == inProgress
}

func (m manifestsModel) Init() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg { return manifestsInitMsg{} },
		findSubstratusManifests(m.Path, m.Filename),
	)
}

type manifestsInitMsg struct{}

func (m manifestsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case manifestsInitMsg:
		log.Println("Initializing manifests")
		m.reading = inProgress

	case readManifestMsg:
		m.reading = completed

		return m, func() tea.Msg { return manifestSelectedMsg{obj: msg.obj} }

	case substratusManifestsMsg:
		m.reading = completed

		var n int
		var single client.Object
		for _, k := range m.Kinds {
			items := msg.manifests[k]
			if single == nil && len(items) > 0 {
				single = items[0]
			}
			n += len(items)
		}

		log.Printf("Found (filtered) manifests: %v", n)

		if n == 0 {
			return m, func() tea.Msg { return fmt.Errorf("No substratus Server kinds found in *.yaml") }
		} else if n == 1 {
			return m, func() tea.Msg { return manifestSelectedMsg{obj: single} }
		} else {
			// TODO: Selector
			return m, func() tea.Msg { return manifestSelectedMsg{obj: single} }
		}

	}

	return m, nil
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m manifestsModel) View() (v string) {
	defer func() {
		if v != "" {
			v = m.Style.Render(v)
		}
	}()

	if m.reading == inProgress {
		v += "Reading manifests..."
	}

	return
}

type manifestSelectedMsg struct {
	obj client.Object
}

type substratusManifestsMsg struct {
	manifests map[string][]client.Object
}

func findSubstratusManifests(path, filename string) tea.Cmd {
	return func() tea.Msg {
		msg := substratusManifestsMsg{
			manifests: map[string][]client.Object{},
		}

		if filename != "" {
			manifest, err := os.ReadFile(filename)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			if err := manifestToObjects(manifest, msg.manifests); err != nil {
				return fmt.Errorf("reading manifests in file: %v: %w", filename, err)
			}
		} else {
			if path == "" {
				var err error
				path, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			matches, err := filepath.Glob("*.yaml")
			if err != nil {
				return err
			}
			for _, p := range matches {
				manifest, err := os.ReadFile(p)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}

				if err := manifestToObjects(manifest, msg.manifests); err != nil {
					return fmt.Errorf("reading manifests in file: %v: %w", p, err)
				}
			}
		}
		return msg
	}
}

func manifestToObjects(manifest []byte, m map[string][]client.Object) error {
	split := bytes.Split(manifest, []byte("---\n"))
	for _, doc := range split {
		if strings.TrimSpace(string(doc)) == "" {
			continue
		}

		obj, err := client.Decode(doc)
		if err != nil {
			return fmt.Errorf("decoding: %w", err)
		}
		switch t := obj.(type) {
		case *apiv1.Model, *apiv1.Dataset, *apiv1.Server, *apiv1.Notebook:
			kind := t.GetObjectKind().GroupVersionKind().Kind
			if m[kind] == nil {
				m[kind] = make([]client.Object, 0)
			}
			m[kind] = append(m[kind], obj)
			// case *apiv1.Model:
			//	msg.models = append(msg.models, t)
			// case *apiv1.Server:
			//	msg.servers = append(msg.servers, t)
			// case *apiv1.Dataset:
			//	msg.datasets = append(msg.datasets, t)
			// case *apiv1.Notebook:
			//	msg.notebooks = append(msg.notebooks, t)
		}
	}

	return nil
}
