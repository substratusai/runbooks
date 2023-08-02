package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/kubectl/internal/cp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

func CopySrcToNotebook(ctx context.Context, baseDir string, nb *apiv1.Notebook) error {
	return cp.ToPod(ctx, filepath.Join(baseDir, "src"), "/content/", podForNotebook(nb), "notebook")
}

func CopySrcFromNotebook(ctx context.Context, baseDir string, nb *apiv1.Notebook) error {
	return cp.FromPod(ctx, "/content/src", filepath.Join(baseDir, "src"), podForNotebook(nb), "notebook")
}

func (c *Client) SyncFilesFromNotebook(ctx context.Context, nb *apiv1.Notebook) error {
	podRef := podForNotebook(nb)
	const containerName = "notebook"

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("determining user cache dir: %w", err)
	}
	binPath := filepath.Join(cacheDir, "substratus", "containertools", "nbwatch")

	// TODO: Download nbwatch if it doesn't exist.

	if err := cp.ToPod(ctx, binPath, "/tmp/nbwatch", podRef, containerName); err != nil {
		return fmt.Errorf("cp nbwatch to pod: %w", err)
	}

	r, w := io.Pipe()

	// TODO: Instead of processing events line-by-line, decode them line-by-line
	// immediately and append them to a channel with deduplication.

	go func() {
		klog.V(1).Info("Reading events...")

		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			eventLine := scanner.Bytes()
			var event NBWatchEvent
			if err := json.Unmarshal(eventLine, &event); err != nil {
				klog.Error(err)
			}

			relPath, err := filepath.Rel("/content/src", event.Path)
			if err != nil {
				klog.Error(err)
				continue
			}

			localDir := "src"
			localPath := filepath.Join(localDir, relPath)

			// Possible: CREATE, REMOVE, WRITE, RENAME, CHMOD
			if event.Op == "WRITE" || event.Op == "CREATE" {
				// NOTE: A long-running port-forward might be more performant here.
				if err := cp.FromPod(ctx, event.Path, localPath, podRef, containerName); err != nil {
					klog.Error(err)
				}
			} else if event.Op == "REMOVE" || event.Op == "RENAME" {
				if err := os.Remove(localPath); err != nil {
					klog.Error(err)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			klog.Error("Error reading from buffer:", err)
			return
		}
		klog.V(1).Info("Done reading events.")
	}()

	if err := c.Exec(podRef, "/tmp/nbwatch", nil, w, os.Stderr); err != nil {
		return fmt.Errorf("exec: nbwatch: %w", err)
	}

	return nil
}

func (c *Client) Exec(podRef types.NamespacedName,
	command string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	req := c.Interface.CoreV1().RESTClient().Post().Resource("pods").Name(podRef.Name).
		Namespace(podRef.Namespace).SubResource("exec")
	option := &corev1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	if stdin == nil {
		option.Stdin = false
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", req.URL())
	if err != nil {
		return err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return err
	}

	return nil
}

type NBWatchEvent struct {
	Index int64  `json:"index"`
	Path  string `json:"path"`
	Op    string `json:"op"`
}
