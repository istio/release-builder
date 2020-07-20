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
// A prereq for this is that the common-files relase branch has been updated with a
// new UPDATE_BRANCH and image in it's files.
func SetupProw(manifest model.Manifest, dryrun bool) error {
	log.Infof("*** Updating prow config for new branches.")
	repo := "test-infra"

	cmd := util.VerboseCommand("go run generate.go branch 1.8")
	cmd.Dir = manifest.RepoDir(repo) + "prow/config/cmd"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate new prow config: %v", err)
	}
	log.Infof("*** Prow config for new branches updated.")
	return nil
}
