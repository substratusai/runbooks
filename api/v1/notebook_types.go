package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NotebookSpec defines the desired state of Notebook
type NotebookSpec struct {
	// Suspend should be set to true to stop the notebook (Pod) from running.
	Suspend bool `json:"suspend,omitempty"`

	Container Container `json:"container,omitempty"`

	// Resources are the compute resources required by the container.
	Resources *Resources `json:"resources,omitempty"`

	// Model to load into the notebook container.
	Model *ObjectRef `json:"model,omitempty"`

	// Dataset to load into the notebook container.
	Dataset *ObjectRef `json:"dataset,omitempty"`
}

func (n *Notebook) GetContainer() *Container {
	return &n.Spec.Container
}

func (n *Notebook) GetConditions() *[]metav1.Condition {
	return &n.Status.Conditions
}

func (n *Notebook) StatusReady() bool {
	return n.Status.Ready
}

func (n *Notebook) GetStatusReady() bool {
	return n.Status.Ready
}
func (n *Notebook) SetStatusReady(r bool) {
	n.Status.Ready = r
}

// NotebookStatus defines the observed state of Notebook
type NotebookStatus struct {
	// Ready indicates that the Notebook is ready to serve. See Conditions for more details.
	Ready bool `json:"ready"`

	// Conditions is the list of conditions that describe the current state of the Notebook.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:resource:categories=ai
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"

// The Notebook API can be used to quickly spin up a development environment backed by high performance compute.
//
//   - Notebooks integrate with the Model and Dataset APIs allow for quick iteration.
//
//   - Notebooks can be synced to local directories to streamline developer experiences using Substratus kubectl plugins.
type Notebook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the observed state of the Notebook.
	Spec NotebookSpec `json:"spec,omitempty"`
	// Status is the observed state of the Notebook.
	Status NotebookStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NotebookList contains a list of Notebook
type NotebookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Notebook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Notebook{}, &NotebookList{})
}
