package model

type BuildOutput int

const (
	Docker BuildOutput = iota
	Helm
	Debian
	Istioctl
)
var (
	AllBuildOutputs = []BuildOutput{Docker, Helm, Debian, Istioctl}
)

type Manifest struct {
	Dependencies     []Dependency  `json:"dependencies"`
	Version          string        `json:"version"`
	WorkingDirectory string        `json:"-"`
	BuildOutputs     []BuildOutput `json:"-"`
}

func (m Manifest) ShouldBuild(bo BuildOutput) bool {
	for _, b := range m.BuildOutputs {
		if bo == b {
			return true
		}
	}
	return false
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
