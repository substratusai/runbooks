package client

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/apimachinery/pkg/types"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"k8s.io/klog/v2"
)

var httpClient = &http.Client{}

type Tarball struct {
	TempDir     string
	Path        string
	MD5Checksum string
}

func PrepareImageTarball(ctx context.Context, buildPath string) (*Tarball, error) {
	exists, err := fileExists(filepath.Join(buildPath, "Dockerfile"))
	if err != nil {
		return nil, fmt.Errorf("checking if Dockerfile exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("path does not contain Dockerfile: %s", buildPath)
	}

	tmpDir, err := os.MkdirTemp("/tmp", "substratus-kubectl-upload")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	tarPath := filepath.Join(tmpDir, "/archive.tar.gz")
	err = tarGz(ctx, buildPath, tarPath)
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

func SetUploadContainerSpec(obj Object, tb *Tarball, requestID string) error {
	type buildable interface {
		GetBuild() *apiv1.Build
		SetBuild(*apiv1.Build)
	}

	bObj, ok := obj.(buildable)
	if !ok {
		return fmt.Errorf("object not compatible")
	}

	b := bObj.GetBuild()
	if b == nil {
		b = &apiv1.Build{}
	}
	b.Git = nil
	b.Upload = &apiv1.BuildUpload{
		MD5Checksum: tb.MD5Checksum,
		RequestID:   requestID,
	}
	bObj.SetBuild(b)

	return nil
}

func ClearImage(obj Object) error {
	type clearable interface {
		SetImage(string)
	}

	bObj, ok := obj.(clearable)
	if !ok {
		return fmt.Errorf("object not compatible")
	}

	bObj.SetImage("")

	return nil
}

func (r *Resource) Apply(obj Object, force bool) error {
	applyManifest, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	// Server-side apply.
	if _, err := r.Patch(obj.GetNamespace(), obj.GetName(), types.ApplyPatchType, applyManifest, &metav1.PatchOptions{
		Force: ptr.To(force),
	}); err != nil {
		return err
	}

	return nil
}

func (r *Resource) Upload(ctx context.Context, obj Object, tb *Tarball) error {
	// NOTE: The r.Helper.WatchSingle() method does not support passing a context, calling the code
	// below instead (it was pulled from the Helper implementation).
	watcher, err := r.RESTClient.Get().
		NamespaceIfScoped(obj.GetNamespace(), r.NamespaceScoped).
		Resource(r.Resource).
		VersionedParams(&metav1.ListOptions{
			ResourceVersion: obj.GetResourceVersion(),
			Watch:           true,
			FieldSelector:   fields.OneTermEqualSelector("metadata.name", obj.GetName()).String(),
		}, metav1.ParameterCodec).
		Watch(ctx)
	if err != nil {
		return err
	}

	var uploadURL string

loop:
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Added, watch.Modified:
			o := event.Object.(interface {
				GetStatusUpload() apiv1.UploadStatus
				GetBuild() *apiv1.Build
			})
			status := o.GetStatusUpload()
			spec := o.GetBuild().Upload
			if status.StoredMD5Checksum == tb.MD5Checksum {
				// This is an edge-case where the controller found a matching upload
				// that already existed in storage.
				klog.V(1).Infof("upload already exists in storage with md5 checksum: %s, skipping upload", status.StoredMD5Checksum)
				return nil
			}
			if status.SignedURL != "" && status.RequestID == spec.RequestID {
				uploadURL = status.SignedURL
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

	// Trigger the controller to requeue the object.
	// Nothing special about this annotation.
	uploadTS := time.Now().UTC().Format(time.RFC3339)
	if _, err := r.Patch(obj.GetNamespace(), obj.GetName(), types.MergePatchType, []byte(fmt.Sprintf(`{ "metadata": {"annotations": { "upload-timestamp": %q } } }`, uploadTS)), &metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patching upload timestamp: %w", err)
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

func tarGz(ctx context.Context, src, dst string) error {
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
		klog.V(4).Infof("Tarring: %v", path)
		if err := ctx.Err(); err != nil {
			return err
		}

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
		header.Name = filepath.Join(header.Name)

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

func fileExists(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
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

	klog.V(2).Infof("uploading tarball to: %s", url)
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
	klog.V(1).Info("successfully uploaded tarball")
	return nil
}
