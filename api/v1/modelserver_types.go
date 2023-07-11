package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelServerSpec defines the desired state of ModelServer
type ModelServerSpec struct {
	// Container that contains model serving application and dependencies.
	Container Container `json:"container"`

	// Resources are the compute resources required by the container.
	Resources *Resources `json:"resources,omitempty"`

	// Model references the Model object to be served.
	Model ObjectRef `json:"model,omitempty"`
}

// ModelServerStatus defines the observed state of ModelServer
type ModelServerStatus struct {
	// Ready indicates whether the ModelServer is ready to serve traffic. See Conditions for more details.
	Ready bool `json:"ready"`

	// Conditions is the list of conditions that describe the current state of the ModelServer.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:resource:categories=ai
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"

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

func (s *ModelServer) GetContainer() *Container {
	return &s.Spec.Container
}

func (s *ModelServer) GetConditions() *[]metav1.Condition {
	return &s.Status.Conditions
}

func (s *ModelServer) GetStatusReady() bool {
	return s.Status.Ready
}

func (s *ModelServer) SetStatusReady(r bool) {
	s.Status.Ready = r
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
