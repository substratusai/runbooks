package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// DatasetSpec defines the desired state of Dataset.
type DatasetSpec struct {
	// Command to run in the container.
	Command []string `json:"command,omitempty"`

	// Environment variables in the container
	Env map[string]intstr.IntOrString `json:"env,omitempty"`

	// Image that contains dataset loading code and dependencies.
	Image *string `json:"image,omitempty"`

	// Build specifies how to build an image.
	Build *Build `json:"build,omitempty"`

	// Resources are the compute resources required by the container.
	Resources *Resources `json:"resources,omitempty"`

	// Params will be passed into the loading process as environment variables.
	Params map[string]intstr.IntOrString `json:"params,omitempty"`
}

func (d *Dataset) GetParams() map[string]intstr.IntOrString {
	return d.Spec.Params
}

func (d *Dataset) GetBuild() *Build {
	return d.Spec.Build
}
func (d *Dataset) SetBuild(b *Build) {
	d.Spec.Build = b
}

func (d *Dataset) SetImage(image string) {
	d.Spec.Image = ptr.To(image)
}
func (d *Dataset) GetImage() string {
	if d.Spec.Image == nil {
		return ""
	}
	return *d.Spec.Image
}

func (d *Dataset) GetConditions() *[]metav1.Condition {
	return &d.Status.Conditions
}

func (d *Dataset) GetStatusReady() bool {
	return d.Status.Ready
}

func (d *Dataset) SetStatusReady(r bool) {
	d.Status.Ready = r
}

func (d *Dataset) GetStatusArtifacts() ArtifactsStatus {
	return d.Status.Artifacts
}

func (d *Dataset) SetStatusUpload(us UploadStatus) {
	d.Status.BuildUpload = us
}

func (d *Dataset) GetStatusUpload() UploadStatus {
	return d.Status.BuildUpload
}

// DatasetStatus defines the observed state of Dataset.
type DatasetStatus struct {
	// Ready indicates that the Dataset is ready to use. See Conditions for more details.
	//+kubebuilder:default:=false
	Ready bool `json:"ready"`

	// Conditions is the list of conditions that describe the current state of the Dataset.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Artifacts status.
	Artifacts ArtifactsStatus `json:"artifacts,omitempty"`

	// BuildUpload contains the status of the build context upload.
	BuildUpload UploadStatus `json:"buildUpload,omitempty"`
}

//+kubebuilder:resource:categories=ai,shortName=data
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"

// The Dataset API is used to describe data that can be referenced for training Models.
//
//   - Datasets pull in remote data sources using containerized data loaders.
//
//   - Users can specify their own ETL logic by referencing a repository from a Dataset.
//
//   - Users can leverage pre-built data loader integrations with various sources.
//
//   - Training typically requires a large dataset. The Dataset API pulls a dataset once and stores it in a bucket, which is mounted directly into training Jobs.
//
//   - The Dataset API allows users to query ready-to-use datasets (`kubectl get datasets`).
//
//   - The Dataset API allows Kubernetes RBAC to be applied as a mechanism for controlling access to data.
type Dataset struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the Dataset.
	Spec DatasetSpec `json:"spec,omitempty"`
	// Status is the observed state of the Dataset.
	Status DatasetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DatasetList contains a list of Dataset
type DatasetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dataset `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dataset{}, &DatasetList{})
}
