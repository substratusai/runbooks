package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec defines the desired state of Model
type ModelSpec struct {
	// Container that contains model code and dependencies.
	Container Container `json:"container"`

	// Loader should be set to run a loading job. Cannot also be set with Trainer.
	Loader *ModelLoader `json:"loader,omitempty"`

	// Trainer should be set to run a training job. Cannot also be set with Loader.
	Trainer *ModelTrainer `json:"trainer,omitempty"`
}

func (m *Model) GetContainer() *Container {
	return &m.Spec.Container
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

type ModelLoader struct {
	// Params will be passed into the loading process as environment variables.
	// Environment variable name will be `"PARAM_" + uppercase(key)`.
	Params map[string]string `json:"params,omitempty"`
}

type ModelTrainer struct {
	// BaseModel should be set in order to mount another model to be
	// used for transfer learning.
	BaseModel *ObjectRef `json:"baseModel,omitempty"`

	// Dataset to mount for training.
	Dataset ObjectRef `json:"datasetName"`

	//+kubebuilder:default:=3
	// Epochs is the total number of iterations that should be run through the training data.
	// Increasing this number will increase training time.
	// The EPOCHS environment variable will be set during training.
	Epochs int64 `json:"epochs,omitempty"`

	//+kubebuilder:default:=1000000000000
	// DataLimit is the maximum number of training records to use. In the case of JSONL, this would be the total number of lines
	// to train with. Increasing this number will increase training time.
	// The DATA_LIMIT environment variable will be set during training.
	DataLimit int64 `json:"dataLimit,omitempty"`

	//+kubebuilder:default:=1
	// BatchSize is the number of training records to use per (forward and backward) pass through the model.
	// Increasing this number will increase the memory requirements of the training process.
	// The BATCH_SIZE environment variable will be set during training.
	BatchSize int64 `json:"batchSize,omitempty"`

	// Params will be passed into the loading process as environment variables.
	// Environment variable name will be `"PARAM_" + uppercase(key)`.
	// For standard parameters like Epochs, use the well-defined Trainer fields.
	Params map[string]string `json:"params,omitempty"`
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	// Ready indicates that the Model is ready to use. See Conditions for more details.
	Ready bool `json:"ready,omitempty"`

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
