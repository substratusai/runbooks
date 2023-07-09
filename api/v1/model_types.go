package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec defines the desired state of Model
type ModelSpec struct {
	// Container that contains model code and dependencies.
	Container Container `json:"container"`

	// Loader should be set to run a loading job.
	Loader *ModelLoader `json:"loader,omitempty"`

	// Trainer should be set to run a training job.
	Trainer *ModelTrainer `json:"trainer,omitempty"`
}

func (m *Model) GetContainer() Container {
	return m.Spec.Container
}
func (m *Model) SetContainer(c Container) {
	m.Spec.Container = c
}

func (m *Model) GetConditions() *[]metav1.Condition {
	return &m.Status.Conditions
}

type ModelLoader struct {
	Params map[string]string `json:"params"`
}

type ModelTrainer struct {
	BaseModel *ObjectRef `json:"baseModel,omitempty"`

	// Dataset to mount for training.
	Dataset ObjectRef `json:"datasetName"`

	// Params is a list of hyperparameters to use for training.
	Params ModelTrainerParams `json:"params"`
}

type ModelTrainerParams struct {
	//+kubebuilder:default:=3
	// Epochs is the total number of iterations that should be run through the training data.
	// Increasing this number will increase training time.
	Epochs int64 `json:"epochs,omitempty"`
	//+kubebuilder:default:=1000000000000
	// DataLimit is the maximum number of training records to use. In the case of JSONL, this would be the total number of lines
	// to train with. Increasing this number will increase training time.
	DataLimit int64 `json:"dataLimit,omitempty"`
	//+kubebuilder:default:=1
	// BatchSize is the number of training records to use per (forward and backward) pass through the model.
	// Increasing this number will increase the memory requirements of the training process.
	BatchSize int64 `json:"batchSize,omitempty"`
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	// Servers is the list of servers that are currently running this Model. Soon to be deprecated.
	Servers []string `json:"servers,omitempty"`

	// Conditions is the list of conditions that describe the current state of the Model.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// URL of model artifacts.
	URL string `json:"url,omitempty"`
}

//+kubebuilder:resource:categories=ai
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Condition",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// The Model API is used to build and train machine learning models.
//
//   - Base models can be built from a Git repository.
//
//   - Models can be trained by combining a base Model with a Dataset.
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
