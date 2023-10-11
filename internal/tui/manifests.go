package tui

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/client"
)

type manifestsModel struct {
	Path           string
	Filename       string
	SubstratusOnly bool

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
	path := m.Path
	if m.Filename != "" {
		path = m.Filename
	}
	return tea.Sequence(
		func() tea.Msg { return manifestsInitMsg{} },
		findManifests(path, m.SubstratusOnly),
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

	case manifestsFoundMsg:
		m.reading = completed

		var n int
		var single client.Object
		byKind := groupObjectsByKind(msg.manifests)
		for _, k := range m.Kinds {
			items := byKind[k]
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

type manifestsFoundMsg struct {
	manifests []client.Object
}

func groupObjectsByKind(objs []client.Object) map[string][]client.Object {
	g := make(map[string][]client.Object)
	for _, o := range objs {
		kind := o.GetObjectKind().GroupVersionKind().Kind
		g[kind] = append(g[kind], o)
	}
	return g
}

func findManifests(path string, substratusOnly bool) tea.Cmd {
	return func() tea.Msg {
		manifests, err := resolveManifests(path, substratusOnly)
		if err != nil {
			return fmt.Errorf("resolving manifests: %w", err)
		}

		var all []client.Object
		for _, manifest := range manifests {
			objs, err := manifestToObjects(manifest, substratusOnly)
			if err != nil {
				return fmt.Errorf("manifest to objects: %w", err)
			}
			all = append(all, objs...)
		}

		if len(all) == 0 {
			return fmt.Errorf("No manifests found: %v", path)
		}

		return manifestsFoundMsg{
			manifests: all,
		}
	}
}

func resolveManifests(path string, substratusOnly bool) ([][]byte, error) {
	typ, err := determinePathType(path)
	if err != nil {
		return nil, fmt.Errorf("determining path type: %w", err)
	}

	switch typ {
	case pathHTTP:
		resp, err := HTTPC.Get(path)
		if err != nil {
			return nil, fmt.Errorf("http: %w", err)
		}
		defer resp.Body.Close()

		// Check server response
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("http: bad status: %s", resp.Status)
		}

		manifest, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("http: reading: %w", err)
		}
		return [][]byte{manifest}, nil

	case pathFile:
		manifest, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}
		return [][]byte{manifest}, nil
	case pathDir:
		glob := filepath.Join(path, "*.yaml")
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}

		var all [][]byte
		for _, p := range matches {
			manifest, err := os.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("reading file: %w", err)
			}
			all = append(all, manifest)
		}
		return all, nil

	default:
		return nil, fmt.Errorf("unrecognized path type: %s", typ)
	}
}

type pathType string

const (
	pathFile = "file"
	pathDir  = "dir"
	pathHTTP = "http"
)

func determinePathType(path string) (pathType, error) {
	if strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") {
		return pathHTTP, nil
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if fileInfo.IsDir() {
		return pathDir, nil
	}

	return pathFile, nil
}

func manifestToObjects(manifest []byte, substratusOnly bool) ([]client.Object, error) {
	var m []client.Object
	split := bytes.Split(manifest, []byte("---\n"))
	for _, doc := range split {
		if strings.TrimSpace(string(doc)) == "" {
			continue
		}

		obj, err := client.Decode(doc)
		if err != nil {
			return nil, fmt.Errorf("decoding: %w", err)
		}
		if obj == nil {
			log.Printf("ignoring nil object: %v", doc)
			continue
		}

		if substratusOnly {
			switch obj.(type) {
			case *apiv1.Model, *apiv1.Dataset, *apiv1.Server, *apiv1.Notebook:
				m = append(m, obj)
			}
		} else {
			m = append(m, obj)
		}
	}

	return m, nil
}
