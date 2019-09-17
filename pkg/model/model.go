package model

import "path"

type BuildOutput int

const (
	Docker BuildOutput = iota
	Helm
	Debian
	Archive
)

var (
	// AllBuildOutputs defines all possible release artifacts
	AllBuildOutputs = []BuildOutput{Docker, Helm, Debian, Archive}
)

// Dependency defines a git dependency for the build
type Dependency struct {
	Org    string `json:"org"`
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
	Sha    string `json:"sha,omitempty"`
}

// Ref returns the git reference of a dependency.
func (d Dependency) Ref() string {
	ref := d.Branch
	if d.Sha != "" {
		ref = d.Sha
	}
	return ref
}

// Manifest defines what is in a release
type Manifest struct {
	// Dependencies declares all git repositories used to build this release
	Dependencies []Dependency `json:"dependencies"`
	// Version specifies what version of Istio this release is
	Version string `json:"version"`
	// Docker specifies the docker hub to use in the helm charts.
	Docker string `json:"docker"`
	// Directory defines the base working directory for the release.
	// This is excluded from the final serialization
	Directory string `json:"-"`
	// BuildOutputs defines what components to build. This allows building only some components.
	BuildOutputs []BuildOutput `json:"-"`
}

// RepoDir is a helper to return the working directory for a repo
func (m Manifest) RepoDir(repo string) string {
	return path.Join(m.Directory, "work", "src", "istio.io", repo)
}

// GoOutDir is a helper to return the directory of Istio build output
func (m Manifest) GoOutDir() string {
	return path.Join(m.Directory, "work", "out", "linux_amd64", "release")
}

// WorkDir is a help to return the work directory
func (m Manifest) WorkDir() string {
	return path.Join(m.Directory, "work")
}

// SourceDir is a help to return the sources directory
func (m Manifest) SourceDir() string {
	return path.Join(m.Directory, "sources")
}

// OutDir is a help to return the out directory
func (m Manifest) OutDir() string {
	return path.Join(m.Directory, "out")
}

// ShouldBuild is a helper to determine if this manifest should build the given artifact
func (m Manifest) ShouldBuild(bo BuildOutput) bool {
	for _, b := range m.BuildOutputs {
		if bo == b {
			return true
		}
	}
	return false
}
