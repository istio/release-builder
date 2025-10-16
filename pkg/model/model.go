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
	"fmt"
	"path"
	"runtime"
)

type (
	BuildOutput    int
	AutoDependency string
)

const (
	Docker BuildOutput = iota
	Helm
	Debian
	Rpm
	Archive
	Grafana
	Scanner

	// Deps will resolve by looking at the istio.deps file in istio/istio
	Deps string = "deps"
	// Modules will resolve by looking at the go.mod file in istio/istio
	Modules string = "modules"
	// ProxyWorkspace will resolve by looking at the WORKSPACE file in istio/proxy.
	// This should only be used to resolve Envoy dep SHA.
	ProxyWorkspace string = "proxy_workspace"
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
	// If true, go version semantic will be used for tagging the git repo, e.g. v1.2.3.
	GoVersionEnabled bool `json:"goversionenabled,omitempty"`
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
	Istio          *Dependency `json:"istio"`
	Api            *Dependency `json:"api"` //nolint: revive, stylecheck
	Proxy          *Dependency `json:"proxy"`
	Ztunnel        *Dependency `json:"ztunnel"`
	ClientGo       *Dependency `json:"client-go"`
	TestInfra      *Dependency `json:"test-infra"`
	Tools          *Dependency `json:"tools"`
	Envoy          *Dependency `json:"envoy"`
	Enhancements   *Dependency `json:"enhancements"`
	ReleaseBuilder *Dependency `json:"release-builder"`
	CommonFiles    *Dependency `json:"common-files"`
}

func (i *IstioDependencies) Get() map[string]*Dependency {
	return map[string]*Dependency{
		"istio":           i.Istio,
		"api":             i.Api,
		"proxy":           i.Proxy,
		"ztunnel":         i.Ztunnel,
		"client-go":       i.ClientGo,
		"test-infra":      i.TestInfra,
		"tools":           i.Tools,
		"envoy":           i.Envoy,
		"release-builder": i.ReleaseBuilder,
		"common-files":    i.CommonFiles,
		"enhancements":    i.Enhancements,
	}
}

// MarshalJSON writes the dependencies, exposing just the SHA
func (i IstioDependencies) MarshalJSON() ([]byte, error) {
	deps := make(map[string]Dependency)
	for repo, dep := range i.Get() {
		if dep == nil {
			continue
		}
		deps[repo] = Dependency{Sha: dep.Sha, GoVersionEnabled: dep.GoVersionEnabled}
	}
	return json.Marshal(deps)
}

func (i *IstioDependencies) Set(repo string, dependency Dependency) {
	dp := i.Get()[repo]
	*dp = dependency
}

type DockerOutput string

const (
	// DockerOutputTar outputs docker images to tar files on disk
	DockerOutputTar DockerOutput = "tar"
	// DockerOutputContext loads docker images into the local docker context
	DockerOutputContext DockerOutput = "context"
)

// Manifest defines what is in a release
type InputManifest struct {
	// Dependencies declares all git repositories used to build this release
	Dependencies IstioDependencies `json:"dependencies"`
	// Version specifies what version of Istio this release is
	Version string `json:"version"`
	// Docker specifies the docker hub to use in the helm charts.
	Docker string `json:"docker"`
	// DockerOutput specifies where docker images are written.
	DockerOutput DockerOutput `json:"dockerOutput"`
	// Architectures defines the architectures to build for.
	// Note: this impacts only docker and deb/rpm; istioctl is always built in additional platforms.
	// Example: []string{"linux/amd64", "linux/arm64"}.
	Architectures []string `json:"architectures"`
	// Directory defines the base working directory for the release.
	// This is excluded from the final serialization
	Directory string `json:"directory"`
	// ProxyOverride specifies a URL to an Envoy binary to use instead of the default proxy
	// The binary will be pulled from `$proxyOverride/envoy-alpha-SHA.tar.gz`
	ProxyOverride string `json:"proxyOverride"`
	// BuildOutputs defines what components to build. This allows building only some components.
	BuildOutputs []string `json:"outputs"`
	// GrafanaDashboards defines a mapping of dashboard name -> ID of the dashboard on grafana.com
	GrafanaDashboards map[string]int `json:"dashboards"`
	// BillOfMaterials flag determines if a Bill of Materials should be produced
	// by the build.
	SkipGenerateBillOfMaterials bool `json:"skipGenerateBillOfMaterials"`
}

// Manifest defines what is in a release
type Manifest struct {
	// Dependencies declares all git repositories used to build this release
	Dependencies IstioDependencies `json:"dependencies"`
	// Version specifies what version of Istio this release is
	Version string `json:"version"`
	// Docker specifies the docker hub to use in the helm charts.
	Docker string `json:"docker"`
	// DockerOutput specifies where docker images are written.
	DockerOutput DockerOutput `json:"dockerOutput"`
	// Architectures defines the architectures to build for.
	// Note: this impacts only docker and deb/rpm; istioctl is always built in additional platforms.
	// Example: []string{"linux/amd64", "linux/arm64"}.
	Architectures []string `json:"architectures"`
	// Directory defines the base working directory for the release.
	// This is excluded from the final serialization
	Directory string `json:"-"`
	// ProxyOverride specifies a URL to an Envoy binary to use instead of the default proxy
	// The binary will be pulled from `$proxyOverride/envoy-alpha-SHA.tar.gz`
	ProxyOverride string `json:"-"`
	// BuildOutputs defines what components to build. This allows building only some components.
	BuildOutputs map[BuildOutput]struct{} `json:"-"`
	// GrafanaDashboards defines a mapping of dashboard name -> ID of the dashboard on grafana.com
	// Note: this tool is not yet smart enough to create dashboards that do not already exist, it can only update dashboards.
	GrafanaDashboards map[string]int `json:"dashboards"`
	// BillOfMaterials flag determines if a Bill of Materials should be produced
	// by the build.
	SkipGenerateBillOfMaterials bool `json:"skipGenerateBillOfMaterials"`
}

// RepoDir is a helper to return the working directory for a repo
func (m Manifest) RepoDir(repo string) string {
	return path.Join(m.Directory, "work", "src", "istio.io", repo)
}

// GoOutDir is a helper to return the directory of Istio build output
func (m Manifest) GoOutDir() string {
	return path.Join(m.Directory, "work", "out", fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH), "release")
}

// RepoOutDir is a helper to return the directory of Istio build output for repos the place outputs inside the repo
func (m Manifest) RepoOutDir(repo string) string {
	return path.Join(m.Directory, "work", "src", "istio.io", repo, "out", fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH), "release")
}

// RepoOutDir is a helper to return the directory of Istio build arch output for repos the place outputs inside the repo
func (m Manifest) RepoArchOutDir(repo string, arch string) string {
	return path.Join(m.Directory, "work", "src", "istio.io", repo, "out", "linux_"+arch, "release")
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
