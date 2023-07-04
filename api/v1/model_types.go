package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec defines the desired state of Model
type ModelSpec struct {
	// Source is a reference to the source of the model.
	Source ModelSource `json:"source"`
	// Training should be set to run a training job.
	Training *ModelTraining `json:"training,omitempty"`
	// Size describes different size dimensions of the underlying model.
	Size ModelSize `json:"size"`
	// Compute describes the compute requirements and preferences of the model.
	Compute ModelCompute `json:"compute"`
}

type ModelCompute struct {
	// +kubebuilder:validation:MinItems=1
	// Types is a list of supported compute types for this Model. This list should be
	// ordered by preference, with the most preferred type first.
	Types []ComputeType `json:"types"`
}

// +kubebuilder:validation:Enum=CPU;GPU
type ComputeType string

const (
	ComputeTypeCPU ComputeType = "CPU"
	ComputeTypeGPU ComputeType = "GPU"
	//ComputeTypeTPU ComputeType = "TPU"
)

type ModelSize struct {
	// ParameterCount is the number of parameters in the underlying model.
	ParameterCount int64 `json:"parameterCount,omitempty"`
	// ParameterBits is the number of bits per parameter in the underlying model. Common values would be 8, 16, 32.
	ParameterBits int `json:"parameterBits,omitempty"`
}

type ModelTraining struct {
	// DatasetName is the .metadata.name of the Dataset to use for training.
	DatasetName string `json:"datasetName"`
	// Params is a list of hyperparameters to use for training.
	Params ModelTrainingParams `json:"params"`
}

type ModelTrainingParams struct {
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

type ModelSource struct {
	// Git is a reference to a git repository containing model code.
	Git *GitSource `json:"git,omitempty"`
	// ModelName is the .metadata.name of another Model that this Model should be based on.
	ModelName string `json:"modelName,omitempty"`
}

const (
	ModelSourceTypeGit   = "Git"
	ModelSourceTypeModel = "Model"
)

func (s ModelSource) Type() string {
	if s.Git != nil {
		return ModelSourceTypeGit
	} else if s.ModelName != "" {
		return ModelSourceTypeModel
	}
	return ""
}

type GitSource struct {
	// URL to the git repository.
	// Example: https://github.com/substratusai/model-falcon-40b
	URL string `json:"url,omitempty"`
	// Path within the git repository referenced by url.
	Path string `json:"path,omitempty"`
	// Branch is the git branch to use.
	Branch string `json:"branch,omitempty"`
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	// ContainerImage is reference to the container image that was built for this Model.
	ContainerImage string `json:"containerImage,omitempty"`
	// Servers is the list of servers that are currently running this Model. Soon to be deprecated.
	Servers []string `json:"servers,omitempty"`
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
