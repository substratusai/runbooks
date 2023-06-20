package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ModelSpec defines the desired state of Model
type ModelSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=latest
	Version string `json:"version,omitempty"`

	Source   ModelSource `json:"source"`
	Training *Training   `json:"training,omitempty"`

	Size ModelSize `json:"size,omitempty"`
}

type ModelSize struct {
	ParameterCount int64 `json:"parameterCount,omitempty"`
	ParameterBits  int   `json:"parameterBits,omitempty"`
}

type Training struct {
	DatasetName string `json:"datasetName"`
}

type ModelSource struct {
	Git       *GitSource `json:"git,omitempty"`
	ModelName string     `json:"modelName,omitempty"`

	// TODO:
	//Container
}

const (
	ModelSourceTypeGit   = "Git"
	ModelSourceTypeModel = "Model"
	//TODO:
	//ModelSourceTypeContainer = "Container"
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
	// Example: github.com/my-account/my-repo
	URL string `json:"url,omitempty"`
	// Path within the git repository referenced by url.
	Path   string `json:"path,omitempty"`
	Branch string `json:"branch,omitempty"`
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ContainerImage string             `json:"containerImage,omitempty"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	Servers        []string           `json:"servers,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Condition",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// Model is the Schema for the models API
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelSpec   `json:"spec,omitempty"`
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
