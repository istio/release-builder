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
	"os"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// UpdateCodeOwners goes to each repo and updates CODEOWNERS to just be the
// release managers.
func UpdateCodeOwners(manifest model.Manifest, release string, dryrun bool) error {
	log.Infof("*** Updating CODEOWNERS")
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

		log.Infof("***Updating CODEOWNERS %s from directory: %s", repo, manifest.RepoDir(repo))

		cmd := util.VerboseCommand("echo", "* @istio/release-managers-1-8")
		cmd.Dir = manifest.RepoDir(repo)
		outFile, err := os.Create(cmd.Dir + "/CODEOWNERS")
		if err != nil {
			return fmt.Errorf("failed using CODEOWNERS: %v", err)
		}
		defer outFile.Close()
		cmd.Stdout = outFile
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run echo command: %v", err)
		}
	}
	log.Infof("*** CODEOWNERS updated")
	return nil
}
