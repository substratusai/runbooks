package client

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/kubectl/internal/cp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

func (c *Client) SyncFilesFromNotebook(ctx context.Context, nb *apiv1.Notebook) error {
	podRef := podForNotebook(nb)
	const containerName = "notebook"

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("determining user cache dir: %w", err)
	}
	binPath := filepath.Join(cacheDir, "substratus", "container-tools", "nbwatch")

	const (
		// TODO: Detect OS and Arch:
		targetOS   = "Linux"
		targetArch = "x86_64"
	)

	if err := getNBWatch(binPath, targetOS, targetArch); err != nil {
		return fmt.Errorf("getting nbwatch: %w", err)
	}

	// TODO: Download nbwatch if it doesn't exist.

	if err := cp.ToPod(ctx, binPath, "/tmp/nbwatch", podRef, containerName); err != nil {
		return fmt.Errorf("cp nbwatch to pod: %w", err)
	}

	r, w := io.Pipe()

	// TODO: Instead of processing events line-by-line, decode them line-by-line
	// immediately and append them to a channel with deduplication.

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			klog.V(2).Info("File sync loop: Done.")
		}()

		klog.V(2).Info("Reading events...")

		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			eventLine := scanner.Bytes()
			var event NBWatchEvent
			if err := json.Unmarshal(eventLine, &event); err != nil {
				klog.Errorf("Failed to unmarshal nbevent: %w", err)
			}

			relPath, err := filepath.Rel("/content/src", event.Path)
			if err != nil {
				klog.Errorf("Failed to determining relative path: %w", err)
				continue
			}

			localDir := "src"
			localPath := filepath.Join(localDir, relPath)

			// Possible: CREATE, REMOVE, WRITE, RENAME, CHMOD
			if event.Op == "WRITE" || event.Op == "CREATE" {
				// NOTE: A long-running port-forward might be more performant here.
				if err := cp.FromPod(ctx, event.Path, localPath, podRef, containerName); err != nil {
					klog.Errorf("Sync: failed to copy: %w", err)
				}
			} else if event.Op == "REMOVE" || event.Op == "RENAME" {
				if err := os.Remove(localPath); err != nil {
					klog.Errorf("Sync: failed to remove: %w", err)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			klog.Error("Error reading from buffer:", err)
			return
		}
		klog.V(2).Info("Done reading events.")
	}()

	if err := c.exec(ctx, podRef, "/tmp/nbwatch", nil, w, os.Stderr); err != nil {
		return fmt.Errorf("exec: nbwatch: %w", err)
	}

	klog.V(2).Info("Waiting for file sync loop to finish...")
	wg.Wait()

	return nil
}

func (c *Client) exec(ctx context.Context, podRef types.NamespacedName,
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
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
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

func getNBWatch(dir, targetOS, targetArch string) error {
	releaseURL := fmt.Sprintf("https://github.com/substratusai/substratus/releases/download/%s/container-tools-%s-%s.tar.gz", Version, targetOS, targetArch)
	klog.V(1).Infof("Downloading: %s", releaseURL)
	resp, err := http.Get(releaseURL)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		dest := filepath.Join(dir, hdr.Name)
		klog.V(1).Infof("Writing %s", dest)
		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return fmt.Errorf("creating file: %w", err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			return fmt.Errorf("writing file from tar: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("closing file: %w", err)
		}
	}

	return nil
}
