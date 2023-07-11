package v1

type Container struct {
	// Git is a reference to a git repository that will be built within the cluster.
	// Built image will be set in the Image field.
	Git *GitSource `json:"git,omitempty"`
	// Image of a container.
	Image string `json:"image,omitempty"`
}

type GitSource struct {
	// URL to the git repository.
	// Example: https://github.com/my-username/my-repo
	URL string `json:"url"`
	// Path within the git repository referenced by url.
	Path string `json:"path,omitempty"`
	// Branch is the git branch to use.
	Branch string `json:"branch,omitempty"`
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
	GPUTypeNvidiaTeslaT4 = GPUType("nvidia-tesla-t4")
	GPUTypeNvidiaL4      = GPUType("nvidia-l4")
)

type GPUResources struct {
	// Type of GPU.
	Type GPUType `json:"type,omitempty"`
	// Count is the number of GPUs.
	Count int64 `json:"count,omitempty"`
}
