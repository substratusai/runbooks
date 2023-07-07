package v1

// TODO add validation to ensure only git, image or local is specified
type Container struct {
	Git   GitSource `json:"git"`
	Image string    `json:"image"`
}

type GitSource struct {
	Url string `json:"url"`
	// refs/heads/my-branch
	Refspec string `json:"refspec,omitempty"`
	Path    string `json:"path,omitempty"`
}
