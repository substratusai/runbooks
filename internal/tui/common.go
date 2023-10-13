package tui

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cli/utils"
	"github.com/substratusai/substratus/internal/client"
)

var (
	P       *tea.Program
	LogFile *os.File
	HTTPC   = &http.Client{Timeout: 30 * time.Second}
)

func init() {
	// Log to a file. Useful in debugging since you can't really log to stdout.
	var err error
	LogFile, err = tea.LogToFile("/tmp/sub.log", "")
	if err != nil {
		panic(err)
	}
}

type Namespace struct {
	Specified  string
	Contextual string
}

func (n Namespace) Set(obj client.Object) {
	if n.Specified != "" {
		obj.SetNamespace(n.Specified)
	} else if obj.GetNamespace() == "" {
		ns := "default"
		if n.Contextual != "" {
			ns = n.Contextual
		}
		obj.SetNamespace(ns)
	}
}

type status int

const (
	notStarted = status(0)
	inProgress = status(1)
	completed  = status(2)
)

type localURLMsg string

type (
	tarballCompleteMsg *client.Tarball
	fileTarredMsg      string
)

func prepareTarballCmd(ctx context.Context, dir string) tea.Cmd {
	return func() tea.Msg {
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

type appliedMsg struct {
	client.Object
	index int
	err   error
}

type applyInput struct {
	client.Object
	index int
}

func applyCmd(ctx context.Context, res *client.Resource, in *applyInput) tea.Cmd {
	return func() tea.Msg {
		if err := res.Apply(in.Object, true); err != nil {
			return appliedMsg{index: in.index, err: err}
		}
		return appliedMsg{Object: in.Object, index: in.index}
	}
}

type createdWithUploadMsg struct {
	client.Object
}

func createWithUploadCmd(ctx context.Context, res *client.Resource, obj client.Object, tarball *client.Tarball, increment, replace bool) tea.Cmd {
	return func() tea.Msg {
		if err := specifyUpload(obj, tarball); err != nil {
			return fmt.Errorf("specifying upload: %w", err)
		}

		if increment {
			list, err := res.List(obj.GetNamespace(), obj.GetObjectKind().GroupVersionKind().Version, &metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("listing: %w", err)
			}

			var version int
			switch list := list.(type) {
			case *apiv1.ModelList:
				version, err = nextModelVersion(list, obj.GetName())
				if err != nil {
					return fmt.Errorf("next model version: %w", err)
				}
			case *apiv1.DatasetList:
				version, err = nextDatasetVersion(list, obj.GetName())
				if err != nil {
					return fmt.Errorf("next dataset version: %w", err)
				}
			default:
				return fmt.Errorf("unrecognized list type: %T", list)
			}

			log.Printf("Next version: %v", version)

			obj.SetName(fmt.Sprintf("%v-%v", obj.GetName(), version))
		}

		if _, err := res.Create(obj.GetNamespace(), true, obj); err != nil {
			if replace && apierrors.IsAlreadyExists(err) {
				if _, err := res.Delete(obj.GetNamespace(), obj.GetName()); err != nil {
					return fmt.Errorf("replacing: delete: %w", err)
				}
				if _, err := res.Create(obj.GetNamespace(), true, obj); err != nil {
					return fmt.Errorf("replacing: creating: %w", err)
				}
			} else {
				return fmt.Errorf("creating: %w", err)
			}
		}

		return createdWithUploadMsg{Object: obj}
	}
}

type objectReadyMsg struct {
	client.Object
}

type objectUpdateMsg struct {
	client.Object
}

func waitReadyCmd(ctx context.Context, res *client.Resource, obj client.Object) tea.Cmd {
	return func() tea.Msg {
		if err := res.WaitReady(ctx, obj, func(updatedObj client.Object) {
			P.Send(objectUpdateMsg{Object: updatedObj})
		}); err != nil {
			return fmt.Errorf("waiting to be ready: %w", err)
		}
		return objectReadyMsg{Object: obj}
	}
}

func nextModelVersion(list *apiv1.ModelList, name string) (int, error) {
	var highestVersion int
	re := regexp.MustCompile(name + `-(\d+)`)
	for _, item := range list.Items {
		match := re.FindStringSubmatch(item.Name)
		if len(match) != 2 {
			continue
		}
		v, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("version label to int: %w", err)
		}
		if v > highestVersion {
			highestVersion = v
		}
	}

	return highestVersion + 1, nil
}

func nextDatasetVersion(list *apiv1.DatasetList, name string) (int, error) {
	var highestVersion int
	re := regexp.MustCompile(name + `-(\d+)`)
	for _, item := range list.Items {
		match := re.FindStringSubmatch(item.Name)
		if len(match) != 2 {
			continue
		}
		v, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, fmt.Errorf("version label to int: %w", err)
		}
		if v > highestVersion {
			highestVersion = v
		}
	}

	return highestVersion + 1, nil
}

type suspendedMsg struct {
	error error
}

func suspendCmd(ctx context.Context, res *client.Resource, obj client.Object) tea.Cmd {
	return func() tea.Msg {
		log.Println("Suspending")
		_, err := res.Patch(obj.GetNamespace(), obj.GetName(), types.MergePatchType, []byte(`{"spec": {"suspend": true} }`), &metav1.PatchOptions{})
		if err != nil {
			log.Printf("Error suspending: %v", err)
			return suspendedMsg{error: err}
		}
		return suspendedMsg{}
	}
}

type deletedMsg struct {
	name  string
	error error
}

func deleteCmd(ctx context.Context, res *client.Resource, obj client.Object) tea.Cmd {
	return func() tea.Msg {
		name := obj.GetName()

		log.Println("Deleting")
		_, err := res.Delete(obj.GetNamespace(), obj.GetName())
		if err != nil {
			log.Printf("Error deleting: %v", err)
			return deletedMsg{name: name, error: err}
		}

		return deletedMsg{name: name}
	}
}

type readManifestMsg struct {
	obj client.Object
}

func readManifest(path string) tea.Cmd {
	return func() tea.Msg {
		log.Println("Reading manifest")

		manifest, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		var obj client.Object
		obj, err = client.Decode(manifest)
		if err != nil {
			return fmt.Errorf("decoding: %w", err)
		}

		return readManifestMsg{
			obj: obj,
		}
	}
}
