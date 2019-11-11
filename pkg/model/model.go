// Copyright Istio Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"encoding/json"
	"path"
)

type BuildOutput int
type AutoDependency string

const (
	Docker BuildOutput = iota
	Helm
	Debian
	Archive

	// Deps will resolve by looking at the istio.deps file in istio/istio
	Deps string = "deps"
	// Modules will resolve by looking at the go.mod file in istio/istio
	Modules string = "modules"
)

// Dependency defines a git dependency for the build
type Dependency struct {
	// Git repository to pull from. Required if branch or sha is set
	Git string `json:"git,omitempty"`
	// Checkout the git branch
	Branch string `json:"branch,omitempty"`
	// Checkout the git SHA
	Sha string `json:"sha,omitempty"`
	// Copy the local path. Note this still needs to be a git repo.
	LocalPath string `json:"localpath,omitempty"`
	// Auto will fetch the SHA to use based on other repos. Currently this supports reading
	// istio.deps from istio/istio only.
	Auto string `json:"auto,omitempty"`
}

// Ref returns the git reference of a dependency.
func (d Dependency) Ref() string {
	ref := d.Branch
	if d.Sha != "" {
		ref = d.Sha
	}
	return ref
}

// Dependencies for the build
type IstioDependencies struct {
	Istio        *Dependency `json:"istio"`
	Cni          *Dependency `json:"cni"`
	Operator     *Dependency `json:"operator"`
	Api          *Dependency `json:"api"` //nolint: golint, stylecheck
	Proxy        *Dependency `json:"proxy"`
	Pkg          *Dependency `json:"pkg"`
	ClientGo     *Dependency `json:"client-go"`
	GogoGenproto *Dependency `json:"gogo-genproto"`
	TestInfra    *Dependency `json:"test-infra"`
	Tools        *Dependency `json:"tools"`
}

func (i *IstioDependencies) Get() map[string]*Dependency {
	return map[string]*Dependency{
		"istio":         i.Istio,
		"cni":           i.Cni,
		"operator":      i.Operator,
		"api":           i.Api,
		"proxy":         i.Proxy,
		"pkg":           i.Pkg,
		"client-go":     i.ClientGo,
		"gogo-genproto": i.GogoGenproto,
		"test-infra":    i.TestInfra,
		"tools":         i.Tools,
	}
}

// MarshalJSON writes the dependencies, exposing just the SHA
func (i IstioDependencies) MarshalJSON() ([]byte, error) {
	deps := make(map[string]Dependency)
	for repo, dep := range i.Get() {
		if dep == nil {
			continue
		}
		deps[repo] = Dependency{Sha: dep.Sha}
	}
	return json.Marshal(deps)
}

func (i *IstioDependencies) Set(repo string, dependency Dependency) {
	dp := i.Get()[repo]
	*dp = dependency
}

// Manifest defines what is in a release
type InputManifest struct {
	// Dependencies declares all git repositories used to build this release
	Dependencies IstioDependencies `json:"dependencies"`
	// Version specifies what version of Istio this release is
	Version string `json:"version"`
	// Docker specifies the docker hub to use in the helm charts.
	Docker string `json:"docker"`
	// Directory defines the base working directory for the release.
	// This is excluded from the final serialization
	Directory string `json:"directory"`
	// ProxyOverride specifies a path to an Envoy binary to use instead of the default proxy
	ProxyOverride string `json:"proxyOverride"`
	// BuildOutputs defines what components to build. This allows building only some components.
	BuildOutputs []string `json:"outputs"`
}

// Manifest defines what is in a release
type Manifest struct {
	// Dependencies declares all git repositories used to build this release
	Dependencies IstioDependencies `json:"dependencies"`
	// Version specifies what version of Istio this release is
	Version string `json:"version"`
	// Docker specifies the docker hub to use in the helm charts.
	Docker string `json:"docker"`
	// Directory defines the base working directory for the release.
	// This is excluded from the final serialization
	Directory string `json:"-"`
	// ProxyOverride specifies a path to an Envoy binary to use instead of the default proxy
	ProxyOverride string `json:"proxyOverride"`
	// BuildOutputs defines what components to build. This allows building only some components.
	BuildOutputs map[BuildOutput]struct{} `json:"-"`
	// BuildInfoFileName stores the path for BUILDINFO, used by build scripts
	BuildInfoFileName string `json:"-"`
}

// RepoDir is a helper to return the working directory for a repo
func (m Manifest) RepoDir(repo string) string {
	return path.Join(m.Directory, "work", "src", "istio.io", repo)
}

// GoOutDir is a helper to return the directory of Istio build output
func (m Manifest) GoOutDir() string {
	return path.Join(m.Directory, "work", "out", "linux_amd64", "release")
}

// RepoOutDir is a helper to return the directory of Istio build output for repos the place outputs inside the repo
func (m Manifest) RepoOutDir(repo string) string {
	return path.Join(m.Directory, "work", "src", "istio.io", repo, "out", "linux_amd64", "release")
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

// IstioDep identifies a external dependency of Istio.
type IstioDep struct {
	Comment       string `json:"_comment,omitempty"`
	Name          string `json:"name,omitempty"`
	RepoName      string `json:"repoName,omitempty"`
	LastStableSHA string `json:"lastStableSHA,omitempty"`
}
