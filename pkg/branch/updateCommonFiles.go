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

// UpdateCommonFiles goes to each repo and runs the command to update the common files.
// A prereq for this is that the common-files release branch has been updated with a
// new UPDATE_BRANCH and image in it's files.
func UpdateCommonFiles(manifest model.Manifest, release string, dryrun bool) error {
	log.Infof("*** Updating common-files UPDATE_BRANCH")
	for repo, dep := range manifest.Dependencies.Get() {
		if dep == nil {
			// Missing a dependency is not always a failure; many are optional dependencies just for
			// tagging.
			log.Infof("skipping missing dependency: %v", repo)
			continue
		}
		// Skip particular repos
		if repo == "common-files" || repo == "envoy" || repo == "test-infra" || repo == "enhancements" {
			log.Infof("Skipping repo: %v", repo)
			continue
		}

		log.Infof("***Updating the common-files for %s from directory: %s", repo, manifest.RepoDir(repo))

		sedString := "s/UPDATE_BRANCH ?=.*/UPDATE_BRANCH ?= \"release-" + release + "\"/"
		cmd := util.VerboseCommand("sed", "-i", sedString, "common/Makefile.common.mk")
		cmd.Dir = manifest.RepoDir(repo)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run command: %v", err)
		}
	}
	log.Infof("*** common-files UPDATE_BRANCH updated")
	return nil
}
