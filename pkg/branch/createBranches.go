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
	"fmt"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// CreateBranches goes to each repo and creates the branches
func CreateBranches(manifest model.Manifest, release string, dryrun bool) error {
	log.Infof("*** Creating release branches")
	for repo, dep := range manifest.Dependencies.Get() {
		if dep == nil {
			// Missing a dependency is not always a failure; many are optional dependencies just for
			// tagging.
			log.Infof("skipping missing dependency: %v", repo)
			continue
		}
		// Skip particular repos
		if repo == "test-infra" {
			log.Infof("Skipping repo: %v", repo)
			continue
		}
		log.Infof("*** Creating a release branch %s for %s from directory: %s", release, repo, manifest.RepoDir(repo))
		cmd := util.VerboseCommand("git", "checkout", "-b", "release-"+release)
		cmd.Dir = manifest.RepoDir(repo)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to checkout release: %v", err)
		}
		if !dryrun {
			cmd = util.VerboseCommand("git", "push", "--set-upstream", "origin", "release-"+release)
			cmd.Dir = manifest.RepoDir(repo)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to push branch to repo: %v", err)
			}
		}
	}
	log.Infof("*** Release branches created")
	return nil
}
