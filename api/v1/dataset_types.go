package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatasetSpec defines the desired state of Dataset
type DatasetSpec struct {
	Filename string        `json:"filename"`
	Source   DatasetSource `json:"source,omitempty"`
}

type DatasetSource struct {
	Git *GitSource `json:"git,omitempty"`
}

// DatasetStatus defines the observed state of Dataset
type DatasetStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	URL string `json:"url,omitempty"`
}

//+kubebuilder:resource:categories=ai
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Condition",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// Dataset is the Schema for the datasets API
type Dataset struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatasetSpec   `json:"spec,omitempty"`
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
