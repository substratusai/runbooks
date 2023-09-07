package cli

import (
	"context"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render
	checkMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("#008000")).SetString("✓")
	// TODO: Better X mark?
	xMark = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).SetString("x")
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
