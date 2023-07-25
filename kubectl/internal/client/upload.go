package client

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"k8s.io/klog/v2"
)

var httpClient = &http.Client{}

type Tarball struct {
	TempDir     string
	Path        string
	MD5Checksum string
}

func PrepareImageTarball(buildPath string) (*Tarball, error) {
	if !fileExists(filepath.Join(buildPath, "Dockerfile")) {
		return nil, fmt.Errorf("path does not contain Dockerfile: %s", buildPath)
	}

	tmpDir, err := os.MkdirTemp("/tmp", "substratus-kubectl-upload")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	tarPath := filepath.Join(tmpDir, "/archive.tar.gz")
	err = tarGz(buildPath, tarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create a tar.gz of the directory: %w", err)
	}

	checksum, err := calculateMD5(tarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate the checksum: %w", err)
	}

	return &Tarball{
		Path:        tarPath,
		MD5Checksum: checksum,
		TempDir:     tmpDir,
	}, nil
}

func SetUploadContainerSpec(obj client.Object, tb *Tarball) error {
	return setContainerUpload(obj, tb.MD5Checksum)
}

func (c *Client) Apply(obj client.Object) error {
	applyManifest, err := Encode(obj)
	if err != nil {
		return err
	}

	// Server-side apply.
	if _, err := c.resource.Patch(obj.GetNamespace(), obj.GetName(), types.ApplyPatchType, applyManifest, &metav1.PatchOptions{
		FieldManager: FieldManager,
	}); err != nil {
		return err
	}

	return nil
}

func (c *Client) Upload(obj client.Object, tb *Tarball) error {
	watcher, err := c.resource.Watch(obj.GetNamespace(), obj.GetName(), &metav1.ListOptions{})
	if err != nil {
		return err
	}

	var uploadURL string

loop:
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Added, watch.Modified:
			// NOTE: event.Object is a *unstructured.Unstructured,
			// not the original Object type.
			// imgStatus := event.Object.(interface{ GetStatusImage() *apiv1.ImageStatus }).GetStatusImage()

			uploadURL, _, _ = unstructured.NestedString(event.Object.(*unstructured.Unstructured).Object, "status", "image", "uploadURL")
			md5Checksum, _, _ := unstructured.NestedString(event.Object.(*unstructured.Unstructured).Object, "status", "image", "md5Checksum")
			if uploadURL != "" && md5Checksum == tb.MD5Checksum {
				watcher.Stop()
				break loop
			}
		case watch.Error:
			// Cast the event.Object to metav1.Status and print its message
			if status, ok := event.Object.(*metav1.Status); ok {
				return fmt.Errorf("watch error occurred: %s", status.Message)
			}
			// TODO(bjb): occasionally this watch errors with:
			// watch error occurred: an error on the server ("unable to decode an event from the watch stream: http2: response body closed") has prevented the request from succeeding
			return errors.New("unknown watch error occurred")
		case watch.Deleted:
			return fmt.Errorf("object deleted before upload completed")
		default:
			return errors.New("unhandled event type")
		}
	}

	if err := uploadTarball(tb, uploadURL); err != nil {
		return fmt.Errorf("uploading tarball: %w", err)
	}

	return nil
}

func setContainerUpload(obj client.Object, md5 string) error {
	type imager interface {
		GetImage() *apiv1.Image
	}

	imgObj, ok := obj.(imager)
	if !ok {
		return fmt.Errorf("object not compatible image uploads")
	}

	img := imgObj.GetImage()
	img.Git = nil
	img.Name = ""
	img.Upload = &apiv1.ImageUpload{
		Md5Checksum: md5,
	}

	return nil
}

func calculateMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func tarGz(src, dst string) error {
	tarFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create tarFile: %w", err)
	}
	defer tarFile.Close()

	gzWriter := gzip.NewWriter(tarFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// TODO(bjb): #121 read .dockerignore if it exists, exclude those files
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk the tempdir path: %w", err)
		}

		// Skip the root directory
		if path == src {
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("failed to read file headers: %w", err)
		}

		// Use relative filepath to ensure the root directory is not included in tarball
		relativePath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to determine relative path: %w", err)
		}

		// clean up the file name to avoid including preceding "./" or "/"
		header.Name = strings.TrimPrefix(relativePath, string(filepath.Separator))

		// Add "/workspace" to the beginning of the header.Name
		header.Name = filepath.Join("workspace", header.Name)

		// Skip if it is not a regular file or a directory
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to prepare a tarfile header: %w", err)
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file during compression: %w", err)
			}
			defer file.Close()
			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to copy file contents: %w", err)
			}
		}

		return nil
	})
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func uploadTarball(tarball *Tarball, url string) error {
	data, err := hex.DecodeString(tarball.MD5Checksum)
	if err != nil {
		return fmt.Errorf("failed to decode hex checksum: %w", err)
	}
	encodedMd5Checksum := base64.StdEncoding.EncodeToString(data)

	file, err := os.Open(tarball.Path)
	if err != nil {
		return fmt.Errorf("tar upload: %w", err)
	}
	defer file.Close()

	klog.Infof("uploading tarball to: %s", url)
	req, err := http.NewRequest(http.MethodPut, url, file)
	if err != nil {
		return fmt.Errorf("tar upload: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-MD5", encodedMd5Checksum)

	// Send the request
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("tar upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}
	fmt.Println("successfully uploaded tarball")
	return nil
}
