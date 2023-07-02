package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelServerSpec defines the desired state of ModelServer
type ModelServerSpec struct {
	// ModelName is the .metadata.name of the Model to be served.
	ModelName string `json:"modelName,omitempty"`
}

// ModelServerStatus defines the observed state of ModelServer
type ModelServerStatus struct {
	// Conditions is the list of conditions that describe the current state of the ModelServer.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:resource:categories=ai
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Condition",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// The ModelServer API is used to deploy a server that exposes the capabilities of a Model
// via a HTTP interface.
type ModelServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the ModelServer.
	Spec ModelServerSpec `json:"spec,omitempty"`
	// Status is the observed state of the ModelServer.
	Status ModelServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ModelServerList contains a list of ModelServer
type ModelServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ModelServer{}, &ModelServerList{})
}
