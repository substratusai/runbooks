package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +structType=atomic
type Build struct {
	// Git is a reference to a git repository that will be built within the cluster.
	// Built image will be set in the .spec.image field.
	Git *BuildGit `json:"git,omitempty"`
	// Upload can be set to request to start an upload flow where the client is
	// responsible for uploading a local directory that is to be built in the cluster.
	Upload *BuildUpload `json:"upload,omitempty"`
}

// +structType=atomic
type BuildUpload struct {
	// MD5Checksum is the md5 checksum of the tar'd repo root requested to be uploaded and built.
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:MinLength=32
	// +kubebuilder:validation:Pattern="^[a-fA-F0-9]{32}$"
	MD5Checksum string `json:"md5Checksum"`

	// RequestID is the ID of the request to build the image.
	// Changing this ID to a new value can be used to get a new signed URL
	// (useful when a URL has expired).
	RequestID string `json:"requestID"`
}

// +structType=atomic
type BuildGit struct {
	// URL to the git repository to build.
	// Example: https://github.com/my-username/my-repo
	URL string `json:"url"`
	// Path within the git repository referenced by url.
	Path string `json:"path,omitempty"`

	// Tag is the git tag to use. Choose either tag or branch.
	// This tag will be pulled only at build time and not monitored
	// for changes.
	Tag string `json:"tag,omitempty"`
	// Branch is the git branch to use. Choose either branch or tag.
	// This branch will be pulled only at build time and not monitored
	// for changes.
	Branch string `json:"branch,omitempty"`
}

type UploadStatus struct {
	// SignedURL is a short lived HTTPS URL.
	// The client is expected to send a PUT request to this URL
	// containing a tar'd docker build context.
	// Content-Type of "application/octet-stream" should be used.
	SignedURL string `json:"signedURL,omitempty"`

	// RequestID is the request id that corresponds to this status.
	// Clients should check that this matches the request id that they
	// set in the upload spec before uploading.
	RequestID string `json:"requestID,omitempty"`

	// Expiration is the time at which the signed URL expires.
	Expiration metav1.Time `json:"expiration,omitempty"`

	// StoredMD5Checksum is the md5 checksum of the file that the controller
	// observed in storage.
	StoredMD5Checksum string `json:"storedMD5Checksum,omitempty"`
}

type ObjectRef struct {
	// Name of Kubernetes object.
	Name string `json:"name"`

	// FUTURE: Possibly allow for cross-namespace references.
	// FUTURE: Possibly allow for cross-cluster references.
}

type Resources struct {
	//+kubebuilder:default:=2
	// CPU resources.
	CPU int64 `json:"cpu,omitempty"`

	//+kubebuilder:default:=10
	// Disk size in Gigabytes.
	Disk int64 `json:"disk,omitempty"`

	//+kubebuilder:default:=10
	// Memory is the amount of RAM in Gigabytes.
	Memory int64 `json:"memory,omitempty"`

	// GPU resources.
	GPU *GPUResources `json:"gpu,omitempty"`
}

type GPUType string

const (
	GPUTypeNvidiaA100 = GPUType("nvidia-a100")
	GPUTypeNvidiaT4   = GPUType("nvidia-t4")
	GPUTypeNvidiaL4   = GPUType("nvidia-l4")
)

type GPUResources struct {
	// Type of GPU.
	Type GPUType `json:"type,omitempty"`
	// Count is the number of GPUs.
	Count int64 `json:"count,omitempty"`
}

type ArtifactsStatus struct {
	URL string `json:"url,omitempty"`
}
