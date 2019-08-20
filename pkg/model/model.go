package model

type Manifest struct {
	Dependencies     []Dependency `json:"dependencies"`
	Version          string       `json:"version"`
	WorkingDirectory string       `json:"-"`
}

type Dependency struct {
	Org    string `json:"org"`
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
	Sha    string `json:"sha,omitempty"`
}

func (d Dependency) Ref() string {
	ref := d.Branch
	if d.Sha != "" {
		ref = d.Sha
	}
	return ref
}
