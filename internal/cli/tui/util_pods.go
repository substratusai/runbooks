package tui

import (
	"context"
	"sort"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/substratusai/substratus/internal/cli/client"
)

type podsModel struct {
	// Cancellation
	Ctx context.Context

	// Clients
	Client   client.Interface
	Resource *client.Resource
	K8s      *kubernetes.Clientset

	Object client.Object

	watchingPods status

	// Watch Pods
	// map[role][podName]
	pods map[string]map[string]podInfo

	// Size
	width int

	// End times
	finalError error
}

type podInfo struct {
	lastEvent watch.EventType
	pod       *corev1.Pod

	logs         string
	logsStarted  bool
	logsViewport viewport.Model
}

func (m *podsModel) New() podsModel {
	m.watchingPods = inProgress

	m.pods = map[string]map[string]podInfo{
		"build": {},
		"run":   {},
	}

	return *m
}

func (m podsModel) Init() tea.Cmd {
	m.watchingPods = inProgress
	return watchPods(m.Ctx, m.Client, m.Object.DeepCopyObject().(client.Object))
}

func (m podsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case podWatchMsg:
		pi := m.pods[msg.Pod.Labels["role"]][msg.Pod.Name]
		pi.lastEvent = msg.Type
		pi.pod = msg.Pod.DeepCopy()

		var cmd tea.Cmd
		if !pi.logsStarted {
			const containerName = "builder"
			for _, status := range pi.pod.Status.ContainerStatuses {
				if status.Name == containerName && status.Ready {
					cmd = getLogs(m.Ctx, m.K8s, pi.pod, containerName)
					pi.logsStarted = true
					pi.logsViewport = viewport.New(m.width-10, 7)
					pi.logsViewport.Style = logStyle
					break
				}
			}
		}

		m.pods[msg.Pod.Labels["role"]][msg.Pod.Name] = pi
		return m, cmd

	case podLogsMsg:
		pi := m.pods[msg.role][msg.name]
		pi.logs += msg.logs + "\n"
		pi.logsViewport.SetContent(wordwrap.String(pi.logs, m.width-14))
		pi.logsViewport.GotoBottom()
		m.pods[msg.role][msg.name] = pi
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		for role := range m.pods {
			for name := range m.pods[role] {
				pi := m.pods[role][name]
				if pi.logsViewport.Width > 0 {
					pi.logsViewport.Width = msg.Width - 10
					m.pods[role][name] = pi
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
func (m podsModel) View() (v string) {
	if m.watchingPods == inProgress {
		roles := []string{"build", "run"}

		var vv string
		for _, role := range roles {
			var pods []podInfo
			for _, p := range m.pods[role] {
				pods = append(pods, p)
			}
			sort.Slice(pods, func(i, j int) bool {
				return pods[i].pod.CreationTimestamp.Before(&pods[j].pod.CreationTimestamp)
			})
			for _, p := range pods {
				if p.lastEvent == watch.Deleted {
					continue
				}
				vv += "> " + p.pod.Labels["role"] + ": " + string(p.pod.Status.Phase) + "\n"
				if p.pod.Status.Phase != corev1.PodSucceeded {
					vv += "\n" + p.logsViewport.View() + "\n"
				}
			}
		}

		// Further indent this section.
		v += podStyle(vv)

	}

	return v
}
