package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cli/client"
	"github.com/substratusai/substratus/internal/cli/utils"
)

func init() {
	// Log to a file. Useful in debugging since you can't really log to stdout.
	var err error
	LogFile, err = tea.LogToFile("/tmp/sub.log", "")
	if err != nil {
		panic(err)
	}
}

var LogFile *os.File

var P *tea.Program

const (
	maxWidth = 60
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

	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e76f51"))

	activeSpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E9C46A"))
	checkMark          = lipgloss.NewStyle().Foreground(lipgloss.Color("#2a9d8f")).SetString("✓")
	// TODO: Better X mark?
	xMark = lipgloss.NewStyle().Foreground(lipgloss.Color("#e76f51")).SetString("x")
)

type (
	tarballCompleteMsg *client.Tarball
	fileTarredMsg      string
)

func prepareTarballCmd(ctx context.Context, dir string) tea.Cmd {
	return func() tea.Msg {
		P.Send(operationMsg{operation: tarring, status: inProgress})
		defer P.Send(operationMsg{operation: tarring, status: completed})

		log.Println("Preparing tarball")
		tarball, err := client.PrepareImageTarball(ctx, dir, func(file string) {
			log.Println("tarred", file)
			P.Send(fileTarredMsg(file))
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
			P.Send(uploadTarballProgressMsg(percentage))
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
		P.Send(operationMsg{operation: creating, status: inProgress})
		defer P.Send(operationMsg{operation: creating, status: completed})

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

		log.Printf("Next version: %v", version)

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

func nextModelVersion(list *apiv1.ModelList) (int, error) {
	var highestVersion int
	for _, item := range list.Items {
		v, err := strconv.Atoi(item.GetLabels()["version"])
		if err != nil {
			return 0, fmt.Errorf("version label to int: %w", err)
		}
		if v > highestVersion {
			highestVersion = v
		}
	}

	return highestVersion + 1, nil
}

func nextDatasetVersion(list *apiv1.DatasetList) (int, error) {
	var highestVersion int
	for _, item := range list.Items {
		v, err := strconv.Atoi(item.GetLabels()["version"])
		if err != nil {
			return 0, fmt.Errorf("version label to int: %w", err)
		}
		if v > highestVersion {
			highestVersion = v
		}
	}

	return highestVersion + 1, nil
}
