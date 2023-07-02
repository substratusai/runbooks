package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatasetSpec defines the desired state of Dataset.
type DatasetSpec struct {
	// Filename is the name of the file when it is downloaded.
	Filename string `json:"filename"`
	// Source if a reference to the code that is doing the data sourcing.
	Source DatasetSource `json:"source,omitempty"`
}

// DatasetSource if a reference to the code that is doing the data sourcing.
type DatasetSource struct {
	// Git is a reference to the git repository that contains the data loading code.
	Git *GitSource `json:"git,omitempty"`
}

// DatasetStatus defines the observed state of Dataset.
type DatasetStatus struct {
	// URL points to the underlying data storage (bucket URL).
	URL string `json:"url,omitempty"`

	// Conditions is the list of conditions that describe the current state of the Dataset.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:resource:categories=ai
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Condition",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

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
