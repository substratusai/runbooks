package cli

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cli/client"
	"github.com/substratusai/substratus/internal/cli/utils"
)

// NewClient is a dirty hack to allow the client to be mocked out in tests.
var NewClient = client.NewClient

var p *tea.Program

const (
	maxWidth = 80
	padding  = 2
)

var (
	appStyle = lipgloss.NewStyle().
			PaddingTop(1).
			PaddingRight(1).
			PaddingBottom(1).
			PaddingLeft(2).
			Render

	podStyle = lipgloss.NewStyle().PaddingLeft(2).Render

	// https://coolors.co/palette/264653-2a9d8f-e9c46a-f4a261-e76f51
	//
	logStyle = lipgloss.NewStyle().PaddingLeft(1).Border(lipgloss.NormalBorder(), false, false, false, true) /*lipgloss.Border{
		TopLeft:    "| ",
		BottomLeft: "| ",
		Left:       "| ",
	})*/

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).
			MarginTop(1).
			MarginBottom(1).
			Render

	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e76f51")).Render
	checkMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("#2a9d8f")).SetString("✓")
	// TODO: Better X mark?
	xMark = lipgloss.NewStyle().Foreground(lipgloss.Color("#e76f51")).SetString("x")
)

type (
	tarballCompleteMsg *client.Tarball
	fileTarredMsg      string
)

func prepareTarballCmd(ctx context.Context, dir string) tea.Cmd {
	return func() tea.Msg {
		log.Println("Preparing tarball")
		tarball, err := client.PrepareImageTarball(ctx, dir, func(file string) {
			log.Println("tarred", file)
			p.Send(fileTarredMsg(file))
		})
		if err != nil {
			log.Println("Error", err)
			return fmt.Errorf("preparing tarball: %w", err)
		}
		return tarballCompleteMsg(tarball)
	}
}

type (
	tarballUploadedMsg struct {
		client.Object
	}
	uploadTarballProgressMsg float64
)

func uploadTarballCmd(ctx context.Context, res *client.Resource, obj client.Object, tarball *client.Tarball) tea.Cmd {
	return func() tea.Msg {
		log.Println("Uploading tarball")
		err := res.Upload(ctx, obj, tarball, func(percentage float64) {
			log.Printf("Upload percentage: %v", percentage)
			p.Send(uploadTarballProgressMsg(percentage))
		})
		if err != nil {
			log.Println("Upload failed", err)
			return fmt.Errorf("uploading: %w", err)
		}
		log.Println("Upload completed")
		return tarballUploadedMsg{Object: obj}
	}
}

func specifyUpload(obj client.Object, tarball *client.Tarball) error {
	if err := client.ClearImage(obj); err != nil {
		return fmt.Errorf("clearing image in spec: %w", err)
	}
	if err := client.SetUploadContainerSpec(obj, tarball, utils.NewUUID()); err != nil {
		return fmt.Errorf("setting upload in spec: %w", err)
	}
	return nil
}

type appliedWithUploadMsg struct {
	client.Object
}

func applyWithUploadCmd(ctx context.Context, res *client.Resource, obj client.Object, tarball *client.Tarball) tea.Cmd {
	return func() tea.Msg {
		if err := specifyUpload(obj, tarball); err != nil {
			return fmt.Errorf("specifying upload: %w", err)
		}
		if err := res.Apply(obj, true); err != nil {
			return fmt.Errorf("applying: %w", err)
		}
		return appliedWithUploadMsg{Object: obj}
	}
}

type createdWithUploadMsg struct {
	client.Object
}

func createWithUploadCmd(ctx context.Context, res *client.Resource, obj client.Object, tarball *client.Tarball) tea.Cmd {
	return func() tea.Msg {
		if err := specifyUpload(obj, tarball); err != nil {
			return fmt.Errorf("specifying upload: %w", err)
		}

		lowerKind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
		if obj.GetLabels() == nil {
			obj.SetLabels(map[string]string{})
		}
		obj.GetLabels()[lowerKind] = obj.GetName()

		list, err := res.List(obj.GetNamespace(), obj.GetObjectKind().GroupVersionKind().Version, &metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				lowerKind: obj.GetName(),
			}).String(),
		})
		if err != nil {
			return fmt.Errorf("listing: %w", err)
		}

		var version int
		switch list := list.(type) {
		case *apiv1.ModelList:
			version, err = nextModelVersion(list)
			if err != nil {
				return fmt.Errorf("next model version: %w", err)
			}
		case *apiv1.DatasetList:
			version, err = nextDatasetVersion(list)
			if err != nil {
				return fmt.Errorf("next dataset version: %w", err)
			}
		default:
			return fmt.Errorf("unrecognized list type: %T", list)
		}

		obj.SetName(fmt.Sprintf("%v.v%v", obj.GetName(), version))
		obj.GetLabels()["version"] = fmt.Sprintf("%v", version)
		if _, err := res.Create(obj.GetNamespace(), true, obj); err != nil {
			return fmt.Errorf("creating: %w", err)
		}
		return createdWithUploadMsg{Object: obj}
	}
}

type objectReadyMsg struct {
	client.Object
}

func waitReadyCmd(ctx context.Context, res *client.Resource, obj client.Object) tea.Cmd {
	return func() tea.Msg {
		if err := res.WaitReady(ctx, obj); err != nil {
			return fmt.Errorf("waiting to be ready: %w", err)
		}
		return objectReadyMsg{Object: obj}
	}
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
					p.Send(podWatchMsg{Type: event.Type, Pod: pod})
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
			p.Send(podLogsMsg{role: pod.Labels["role"], name: pod.Name, logs: logs})
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	}
}

func nextModelVersion(list *apiv1.ModelList) (int, error) {
	if len(list.Items) == 0 {
		return 0, nil
	}

	var sortErr error
	sort.Slice(list.Items, func(i, j int) bool {
		vi, err := strconv.Atoi(list.Items[i].GetLabels()["version"])
		if err != nil {
			sortErr = err
		}
		vj, err := strconv.Atoi(list.Items[j].GetLabels()["version"])
		if err != nil {
			sortErr = err
		}
		return vi > vj
	})
	if sortErr != nil {
		return 0, sortErr
	}

	v, err := strconv.Atoi(list.Items[0].GetLabels()["version"])
	if err != nil {
		return 0, err
	}

	return v + 1, nil
}

func nextDatasetVersion(list *apiv1.DatasetList) (int, error) {
	if len(list.Items) == 0 {
		return 0, nil
	}

	var sortErr error
	sort.Slice(list.Items, func(i, j int) bool {
		vi, err := strconv.Atoi(list.Items[i].GetLabels()["version"])
		if err != nil {
			sortErr = err
		}
		vj, err := strconv.Atoi(list.Items[j].GetLabels()["version"])
		if err != nil {
			sortErr = err
		}
		return vi < vj
	})
	if sortErr != nil {
		return 0, sortErr
	}

	v, err := strconv.Atoi(list.Items[0].GetLabels()["version"])
	if err != nil {
		return 0, err
	}

	return v + 1, nil
}
