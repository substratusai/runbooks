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
	"strings"
	"sync"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/kubectl/internal/cp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		return fmt.Errorf("determining user cache directory: %w", err)
	}
	toolsPath := filepath.Join(cacheDir, "substratus", "container-tools")
	if err := os.MkdirAll(toolsPath, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	const (
		// Assuming linux Nodes.
		targetOS = "linux"
	)

	nodeArch, err := c.getNodeArchForPod(ctx, podRef.Name, podRef.Namespace)
	if err != nil {
		return fmt.Errorf("getting node arch: %w", err)
	}

	if err := getContainerTools(ctx, toolsPath, targetOS); err != nil {
		return fmt.Errorf("getting container-tools: %w", err)
	}

	// TODO: Download nbwatch if it doesn't exist.

	nbWatchPath := filepath.Join(toolsPath, nodeArch, "nbwatch")
	if err := cp.ToPod(ctx, nbWatchPath, "/tmp/nbwatch", podRef, containerName); err != nil {
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
				klog.Errorf("Failed to determine relative path: %w", err)
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
		w.Close()
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

func getContainerTools(ctx context.Context, dir, targetOS string) error {
	// Check to see if tools need to be downloaded.
	versionPath := filepath.Join(dir, "version.txt")
	exists, err := fileExists(versionPath)
	if err != nil {
		return fmt.Errorf("checking if version file exists: %w", err)
	}
	if exists {
		version, err := os.ReadFile(versionPath)
		if err != nil {
			return fmt.Errorf("reading version file: %w", err)
		}
		versionStr := strings.TrimSpace(string(version))
		if versionStr == Version {
			klog.V(1).Infof("Version (%q) matches for container-tools, skipping download.", Version)
			return nil
		} else {
			klog.V(1).Infof("Version (%q) does not match version.txt: %q", Version, versionStr)
		}
	}

	// Remove existing files.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing existing files: %w", err)
	}

	for _, arch := range []string{"amd64", "arm64"} {
		archDir := filepath.Join(dir, arch)
		if err := os.MkdirAll(archDir, 0755); err != nil {
			return fmt.Errorf("recreating directory: %w", err)
		}
		if err := getContainerToolsRelease(ctx, archDir, targetOS, arch); err != nil {
			return fmt.Errorf("getting container-tools: %w", err)
		}
	}

	if err := os.WriteFile(versionPath, []byte(Version), 0644); err != nil {
		return fmt.Errorf("writing version file: %w", err)
	}

	return nil
}

func getContainerToolsRelease(ctx context.Context, dir, targetOS, targetArch string) error {
	releaseURL := fmt.Sprintf("https://github.com/substratusai/substratus/releases/download/v%s/container-tools-%s-%s.tar.gz", Version, targetOS, targetArch)
	klog.V(1).Infof("Downloading: %s", releaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", releaseURL, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading release: %s", resp.Status)
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

func (c *Client) getNodeArchForPod(ctx context.Context, podName, podNamespace string) (string, error) {
	pod, err := c.Interface.CoreV1().Pods(podNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting pod: %w", err)
	}
	nodeName := pod.Spec.NodeName

	node, err := c.Interface.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting node: %w", err)
	}

	arch, ok := node.Labels["kubernetes.io/arch"]
	if !ok {
		return "", fmt.Errorf("node %s has no kubernetes.io/arch label", nodeName)
	}

	return arch, nil
}
