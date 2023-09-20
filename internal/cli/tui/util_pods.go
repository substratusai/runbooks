package tui

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	log.Println("podsModel.Init()")
	return watchPods(m.Ctx, m.Client, m.Object.DeepCopyObject().(client.Object))
}

func (m podsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case podWatchMsg:
		pi := m.pods[msg.Pod.Labels["role"]][msg.Pod.Name]
		pi.lastEvent = msg.Type
		pi.pod = msg.Pod.DeepCopy()

		containerName := pi.pod.Annotations["kubectl.kubernetes.io/default-container"]

		var cmd tea.Cmd
		if !pi.logsStarted {
			for _, status := range pi.pod.Status.ContainerStatuses {
				if status.Name == containerName && status.Ready {
					log.Printf("Getting logs for Pod container: %v", status.Name)
					cmd = getLogs(m.Ctx, m.K8s, pi.pod, containerName)
					pi.logsStarted = true
					pi.logsViewport = viewport.New(m.width-10, 7)
					pi.logsViewport.Style = logStyle
					break
				} else {
					log.Printf("Skipping logs for container: %v (Ready = %v)", status.Name, status.Ready)
				}
			}
		}

		m.pods[msg.Pod.Labels["role"]][msg.Pod.Name] = pi
		return m, cmd

	case podLogsMsg:
		pi := m.pods[msg.role][msg.name]
		pi.logs += msg.logs + "\n"
		pi.logsViewport.SetContent(lipgloss.NewStyle().Width(m.width - 20).Render(pi.logs) /*wordwrap.String(pi.logs, m.width-14)*/)
		pi.logsViewport.GotoBottom()
		m.pods[msg.role][msg.name] = pi
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		log.Printf("!!!Width: %v", m.width)
		for role := range m.pods {
			for name := range m.pods[role] {
				pi := m.pods[role][name]
				if pi.logsViewport.Width > 0 {
					pi.logsViewport.Width = msg.Width - 10
					pi.logsViewport.SetContent(lipgloss.NewStyle().Width(m.width - 20).Render(pi.logs))
					pi.logsViewport.Style = logStyle
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

type podWatchMsg struct {
	Type watch.EventType
	Pod  *corev1.Pod
}

func watchPods(ctx context.Context, c client.Interface, obj client.Object) tea.Cmd {
	return func() tea.Msg {
		log.Println("Starting Pod watch")

		pods, err := c.Resource(&corev1.Pod{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}})
		if err != nil {
			return fmt.Errorf("pods client: %w", err)
		}

		w, err := pods.Watch(ctx, obj.GetNamespace(), nil, &metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind): obj.GetName(),
				//"role": role,
			}).String(),
		})
		if err != nil {
			return fmt.Errorf("watch: %w", err)
		}
		go func() {
			for event := range w.ResultChan() {
				switch event.Type {
				case watch.Added, watch.Modified, watch.Deleted:
					pod := event.Object.(*corev1.Pod)
					log.Printf("Pod event: %s: %s", pod.Name, event.Type)
					P.Send(podWatchMsg{Type: event.Type, Pod: pod})
				}
			}
		}()

		return nil
	}
}

type podLogsMsg struct {
	role string
	name string
	logs string
}

func getLogs(ctx context.Context, k8s *kubernetes.Clientset, pod *corev1.Pod, container string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("Starting to get logs for pod: %v", pod.Name)
		req := k8s.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container:  container,
			Follow:     true,
			Timestamps: false,
		})
		logs, err := req.Stream(ctx)
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			logs := scanner.Text()
			log.Printf("Pod logs for: %v: %q", pod.Name, logs)
			P.Send(podLogsMsg{role: pod.Labels["role"], name: pod.Name, logs: logs})
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	}
}
