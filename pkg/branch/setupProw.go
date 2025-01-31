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

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// SetupProw goes to the test-infra repo and runs the commands to generate the
// config files for the new release.
func SetupProw(manifest model.Manifest, release string, dryrun bool) error {
	log.Infof("*** Updating prow config for new branches.")
	repo := manifest.RepoDir("test-infra")
	prowGenInputDir := path.Join(repo, "prow/config/jobs")
	prowGenOutputDir := path.Join(repo, "prow/cluster/jobs")

	branchCmd := util.VerboseCommand("go", "run", "./cmd/prowgen/main.go", "--skip-gar-tagging", "--input-dir="+prowGenInputDir,
		"branch", release)
	branchCmd.Dir = path.Join(repo, "tools/prowgen")
	if err := branchCmd.Run(); err != nil {
		return fmt.Errorf("failed to generate new prow config: %v", err)
	}

	writeCmd := util.VerboseCommand("go", "run", "./cmd/prowgen/main.go",
		"--input-dir="+prowGenInputDir, "--output-dir="+prowGenOutputDir, "write")
	writeCmd.Dir = path.Join(repo, "tools/prowgen")
	if err := writeCmd.Run(); err != nil {
		return fmt.Errorf("failed to write new prow config: %v", err)
	}

	privateJobsProwConfigDir := path.Join(repo, "prow/config/istio-private_jobs")
	privateCmd := util.VerboseCommand("go", "run", "main.go", "--input-dir="+privateJobsProwConfigDir, "branch", release)
	privateCmd.Dir = path.Join(repo, "tools/generate-transform-jobs")
	if err := privateCmd.Run(); err != nil {
		return fmt.Errorf("failed to generate new private prow config: %v", err)
	}

	log.Infof("*** Prow config for new branches updated.")
	return nil
}
