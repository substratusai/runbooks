package v1

// TODO add validation to ensure only git, image or local is specified
type Container struct {
	Git   *GitSource `json:"git,omitempty"`
	Image string     `json:"image,omitempty"`
}

type GitSource struct {
	URL string `json:"url"`
	// Git refspec, for example, refs/heads/my-branch
	Branch string `json:"branch,omitempty"`
	Path   string `json:"path,omitempty"`
}
