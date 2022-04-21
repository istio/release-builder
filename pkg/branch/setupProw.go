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
	"path"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// SetupProw goes to the test-infra repo and runs the commands to generate the
// config files for the new release.
func SetupProw(manifest model.Manifest, release string, dryrun bool) error {
	log.Infof("*** Updating prow config for new branches.")
	repo := "test-infra"

	cmd := util.VerboseCommand("go", "run", "./cmd/prowgen/main.go", "branch", release)
	cmd.Dir = path.Join(manifest.RepoDir(repo), "tools/prowgen")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate new prow config: %v", err)
	}

	privateCmd := util.VerboseCommand("go", "run", "main.go", "branch", release)
	cmd.Dir = path.Join(manifest.RepoDir(repo), "tools/generate-transform-jobs")
	if err := privateCmd.Run(); err != nil {
		return fmt.Errorf("failed to generate new private prow config: %v", err)
	}

	log.Infof("*** Prow config for new branches updated.")
	return nil
}
