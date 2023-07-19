package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ModelSpec defines the desired state of Model
type ModelSpec struct {
	// Command to run in the container.
	Command []string `json:"command,omitempty"`

	// Image that contains model code and dependencies.
	Image Image `json:"image"`

	// Resources are the compute resources required by the container.
	Resources *Resources `json:"resources,omitempty"`

	// BaseModel should be set in order to mount another model to be
	// used for transfer learning.
	BaseModel *ObjectRef `json:"baseModel,omitempty"`

	// Dataset to mount for training.
	TrainingDataset *ObjectRef `json:"trainingDataset,omitempty"`

	// Parameters are passing into the model training/loading container as environment variables.
	// Environment variable name will be `"PARAM_" + uppercase(key)`.
	Params map[string]intstr.IntOrString `json:"params,omitempty"`
}

func (m *Model) GetImage() *Image {
	return &m.Spec.Image
}

func (m *Model) GetConditions() *[]metav1.Condition {
	return &m.Status.Conditions
}

func (m *Model) GetStatusReady() bool {
	return m.Status.Ready
}

func (m *Model) SetStatusReady(r bool) {
	m.Status.Ready = r
}

func (m *Model) GetArtifactsStatus() ArtifactsStatus {
	return m.Status.Artifacts
}

func (m *Model) SetStatusImage(us ImageStatus) {
	m.Status.Image = us
}

func (m *Model) GetStatusImage() ImageStatus {
	return m.Status.Image
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	// Ready indicates that the Model is ready to use. See Conditions for more details.
	//+kubebuilder:default:=false
	Ready bool `json:"ready"`

	// Conditions is the list of conditions that describe the current state of the Model.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Artifacts status.
	Artifacts ArtifactsStatus `json:"artifacts,omitempty"`

	// Image contains the status of the image. Upload URL is reported here.
	Image ImageStatus `json:"image,omitempty"`
}

//+kubebuilder:resource:categories=ai
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"

// The Model API is used to build and train machine learning models.
//
//   - Base models can be built from a Git repository.
//
//   - Models can be trained by combining a base Model with a Dataset.
//
//   - Model artifacts are persisted in cloud buckets.
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the Model.
	Spec ModelSpec `json:"spec,omitempty"`
	// Status is the observed state of the Model.
	Status ModelStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ModelList contains a list of Model
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Model{}, &ModelList{})
}
