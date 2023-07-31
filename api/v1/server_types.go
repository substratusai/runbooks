package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServerSpec defines the desired state of Server
type ServerSpec struct {
	// Command to run in the container.
	Command []string `json:"command,omitempty"`

	// Image that contains model serving application and dependencies.
	Image string `json:"image,omitempty"`

	// Build specifies how to build an image.
	Build *Build `json:"build,omitempty"`

	// Resources are the compute resources required by the container.
	Resources *Resources `json:"resources,omitempty"`

	// Model references the Model object to be served.
	Model ObjectRef `json:"model,omitempty"`
}

// ServerStatus defines the observed state of Server
type ServerStatus struct {
	// Ready indicates whether the Server is ready to serve traffic. See Conditions for more details.
	//+kubebuilder:default:=false
	Ready bool `json:"ready"`

	// Conditions is the list of conditions that describe the current state of the Server.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Upload contains the status of the image build.
	Upload UploadStatus `json:"upload,omitempty"`
}

//+kubebuilder:resource:categories=ai
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"

// The Server API is used to deploy a server that exposes the capabilities of a Model
// via a HTTP interface.
type Server struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the Server.
	Spec ServerSpec `json:"spec,omitempty"`
	// Status is the observed state of the Server.
	Status ServerStatus `json:"status,omitempty"`
}

func (s *Server) GetBuild() *Build {
	return s.Spec.Build
}
func (s *Server) SetBuild(b *Build) {
	s.Spec.Build = b
}

func (s *Server) GetImage() string {
	return s.Spec.Image
}

func (s *Server) SetImage(image string) {
	s.Spec.Image = image
}

func (s *Server) GetConditions() *[]metav1.Condition {
	return &s.Status.Conditions
}

func (s *Server) GetStatusReady() bool {
	return s.Status.Ready
}

func (s *Server) SetStatusReady(r bool) {
	s.Status.Ready = r
}

func (s *Server) SetStatusUpload(b UploadStatus) {
	s.Status.Upload = b
}

func (s *Server) GetStatusUpload() UploadStatus {
	return s.Status.Upload
}

//+kubebuilder:object:root=true

// ServerList contains a list of Server
type ServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Server `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Server{}, &ServerList{})
}
