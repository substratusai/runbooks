package v1

type Container struct {
	// Git is a reference to a git repository containing model code.
	Git *GitSource `json:"git,omitempty"`
	// Image of a container that can be used to run the model.
	Image string `json:"image,omitempty"`
}

type ObjectRef struct {
	Name string `json:"name,omitempty"`

	// FUTURE: Possibly allow for cross-namespace references.
	// FUTURE: Possibly allow for cross-cluster references.
}
