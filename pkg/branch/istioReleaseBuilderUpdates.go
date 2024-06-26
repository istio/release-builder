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

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// IstioReleaseBuilderUpdates updates master to use next release base image, and build
// dev images based on next release.
func IstioReleaseBuilderUpdates(manifest model.Manifest, release string, dryrun bool) error {
	log.Infof("*** Updating release-builder to use branch " + release)
	repo := "release-builder"

	sedString := "$!N;/test-infra/! s/\\(.*\\n\\)\\(.*branch: \\)master/\\1\\2release-" + release + "/;P;D"
	cmd := util.VerboseCommand("sed", "-i", sedString, "example/manifest.yaml",
		"release/build.sh", "test/publish.sh", "release/build-base-images.sh")
	cmd.Dir = manifest.RepoDir(repo)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command: %v", err)
	}

	log.Infof("*** release-builder to use branch " + release)
	return nil
}
