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

package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/rogpeppe/go-internal/modfile"

	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"

	"istio.io/pkg/log"
)

// Sources will copy all dependencies require, pulling from Github if required, and set up the working tree.
// This includes locally tagging all git repos with the version being built, so that the right version is present in binaries.
func Sources(manifest model.Manifest) error {
	for _, repo := range manifest.TopDependencies.List() {
		dependency := manifest.TopDependencies.Get(repo)
		src := path.Join(manifest.SourceDir(), repo)

		// Fetch the dependency
		if err := util.Clone(repo, *dependency, src); err != nil {
			return fmt.Errorf("failed to resolve %+v: %v", dependency, err)
		}
		log.Infof("Resolved %v", repo)

		// Also copy it to the working directory
		if err := util.CopyDir(src, manifest.RepoDir(repo)); err != nil {
			return fmt.Errorf("failed to copy dependency %v to working directory: %v", repo, err)
		}

		// Tag the repo. This allows the build process to look at the git tag for version information
		if err := TagRepo(manifest, manifest.RepoDir(repo)); err != nil {
			return fmt.Errorf("failed to tag repo %v: %v", repo, err)
		}
	}
	return nil
}

// The release expects a working directory with:
// * sources/ contains all of the sources to build from. These should not be modified
// * work/ initially contains all the sources, but may be modified during the build
// * out/ contains all final artifacts
func SetupWorkDir(dir string) error {
	if err := os.Mkdir(path.Join(dir, "sources"), 0750); err != nil {
		return fmt.Errorf("failed to set up working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(dir, "work"), 0750); err != nil {
		return fmt.Errorf("failed to set up working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(dir, "out"), 0750); err != nil {
		return fmt.Errorf("failed to set up working directory: %v", err)
	}
	return nil
}

// TagRepo tags a given git repo with the version from the manifest.
func TagRepo(manifest model.Manifest, repo string) error {
	headSha, err := GetSha(repo, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get HEAD SHA: %v", err)
	}
	currentTagSha, _ := GetSha(repo, manifest.Version)
	if currentTagSha != "" {
		if currentTagSha == headSha {
			log.Infof("Tag %v already exists, but points to the right place.", manifest.Version)
			return nil
		}
		return fmt.Errorf("tag %v already exists, retagging would move from %v to %v", manifest.Version, currentTagSha, headSha)
	}
	cmd := util.VerboseCommand("git", "tag", manifest.Version)
	cmd.Dir = repo
	return cmd.Run()
}

// GetSha returns the SHA for a given reference, or error if sha is not found
func GetSha(repo string, ref string) (string, error) {
	buf := bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Stdout = &buf
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// StandardizeManifest will convert a manifest to a fixed SHA, rather than a branch
// This allows outputting the exact version used after the build is complete
func StandardizeManifest(manifest *model.Manifest) error {
	for _, repo := range manifest.TopDependencies.List() {
		sha, err := GetSha(manifest.RepoDir(repo), "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get SHA for %v: %v", repo, err)
		}
		newDep := model.Dependency{
			Sha: strings.TrimSpace(sha),
		}
		manifest.TopDependencies.Set(repo, newDep)
	}
	if err := fetchTransitiveDependencies(manifest); err != nil {
		return fmt.Errorf("failed to get transitive dependencies: %v", err)
	}
	return nil
}

func fetchTransitiveDependencies(manifest *model.Manifest) error {
	// Add known versions
	for _, repo := range manifest.TopDependencies.List() {
		dep := manifest.TopDependencies.Get(repo)
		// For consistency with go.mod files, limit to 12 ch
		manifest.AllDependencies[repo] = dep.Sha[:12]
	}
	// Read istio go.mod
	modFile, err := ioutil.ReadFile(path.Join(manifest.RepoDir("istio"), "go.mod"))
	if err != nil {
		return err
	}
	mod, err := modfile.Parse("", modFile, nil)
	if err != nil {
		return err
	}
	for _, r := range mod.Require {
		if strings.HasPrefix(r.Mod.Path, "istio.io/") {
			ver := r.Mod.Version
			if len(strings.Split(ver, "-")) == 3 {
				// We are dealing with a pseudo version
				ver = strings.Split(ver, "-")[2]
			}
			pathsplit := strings.Split(r.Mod.Path, "/")
			if _, f := manifest.AllDependencies[pathsplit[1]]; !f {
				manifest.AllDependencies[pathsplit[1]] = ver
			}
		}
	}
	// Read istio istio.deps
	depsFile, err := ioutil.ReadFile(path.Join(manifest.RepoDir("istio"), "istio.deps"))
	if err != nil {
		return err
	}
	deps := make([]model.IstioDep, 0)
	if err := json.Unmarshal(depsFile, &deps); err != nil {
		return err
	}
	for _, d := range deps {
		if _, f := manifest.AllDependencies[d.RepoName]; !f {
			// For consistency with go.mod files, limit to 12 ch
			manifest.AllDependencies[d.RepoName] = d.LastStableSHA[:12]
		}
	}
	return nil
}
