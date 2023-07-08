package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec defines the desired state of Model
type ModelSpec struct {
	// Container specifies the runtime container to use for loader or trainer
	Container Container `json:"container"`
	// Loader should be set to load a model from an external source
	Loader *ModelLoader `json:"loader,omitempty"`
	// Training should be set to run a training job.
	Trainer *ModelTrainer `json:"training,omitempty"`
}

type ModelLoader struct {
	// Params is a list of hyperparameters to use for training.
	// TODO discuss if Params should be map[string]string
	Params ModelLoaderParams `json:"params"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ModelLoaderParams struct {
	// Name to use inside the loader. In case of HuggingFace loader this would be <HF_ORG>/<HF_REPO>
	Name string `json:"epochs,omitempty"`
}

type ModelTrainer struct {
	// SourceModel to use as a base model for training job
	SourceModel *ModelSource `json:"sourceModel,omitempty"`
	// DatasetName is the .metadata.name of the Dataset to use for training.
	DatasetName string `json:"datasetName"`
	// Params is a list of hyperparameters to use for training.
	Params ModelTrainerParams `json:"params"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ModelSource struct {
	// Name is the .metadata.name of another Model that this Model should be based on.
	Name string `json:"name,omitempty"`
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
	// URL points to the underlying data storage (bucket URL).
	URL string `json:"url,omitempty"`
	// ContainerImage is reference to the container image that was built for this Model.
	ContainerImage string `json:"containerImage,omitempty"`
	// Conditions is the list of conditions that describe the current state of the Model.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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
