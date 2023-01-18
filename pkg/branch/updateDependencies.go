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

package branch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// UpdateDependencies runs commands in the istio/istio repo: ./bin/update_deps.sh and
// make gen.
func UpdateDependencies(manifest model.Manifest, dryrun bool) error {
	log.Infof("*** Updating the istio.istio dependencies in the master branch before branching.")
	release := "master" // This is being done before branching
	repo := "istio"

	cmd := util.VerboseCommand("./bin/update_deps.sh")
	cmd.Stdin = os.Stdin
	cmd.Dir = manifest.RepoDir(repo)
	env := []string{"UPDATE_BRANCH=" + release}
	cmd.Env = append(os.Environ(), env...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update dependencies during update_deps: %v", err)
	}

	// Also update the go-control plane
	cmd = util.VerboseCommand("go", "get", "github.com/envoyproxy/go-control-plane@main")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update dependencies during update_deps: %v", err)
	}

	// release_builder sets VERSION to the value in the manifest (ex: 1.9) and
	// for this command we want the version unset (or what's in the Makefile.core.mk).
	var out bytes.Buffer
	grepCmd := exec.Command("grep", "export VERSION", "Makefile.core.mk")
	grepCmd.Stdout = &out
	grepCmd.Dir = manifest.RepoDir(repo)
	err := grepCmd.Run()
	if err != nil {
		return fmt.Errorf("grep error: %v", err)
	}
	makefileVersion := strings.TrimSpace(strings.Split(out.String(), "?=")[1])

	env = []string{"VERSION=" + makefileVersion}
	if err := util.RunMake(manifest, repo, env, "gen"); err != nil {
		return fmt.Errorf("failed to update dependencies in make: %v", err)
	}

	log.Infof("*** istio.istio dependencies in the master branch updated.")
	return nil
}
