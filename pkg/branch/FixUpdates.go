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

// FixUpdates will fixup a problem where the commonfiles update didn't work
// correctly. Should be able to fix the problem and remove this
func FixUpdates(manifest model.Manifest, release string, dryrun bool) error {
	log.Infof("*** Updating common-files")
	for repo, dep := range manifest.Dependencies.Get() {
		if dep == nil {
			// Missing a dependency is not always a failure; many are optional dependencies just for
			// tagging.
			log.Infof("skipping missing dependency: %v", repo)
			continue
		}
		// Skip particular repos
		if repo == "common-files" || repo == "test-infra" || repo == "envoy" {
			log.Infof("Skipping repo: %v", repo)
			continue
		}

		log.Infof("***Updating the common-files for %s from directory: %s", repo, manifest.RepoDir(repo))

		env := []string{"UPDATE_BRANCH=release-" + release}
		if err := util.RunMake(manifest, repo, env, "update-common"); err != nil {
			return fmt.Errorf("failed to update common-files in make: %v", err)
		}
	}
	log.Infof("*** make update-common updated")
	return nil
}
