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

// CreateToolsImage update the BRANCH for the docker image name. In the postsubmit,
// new images will be created.
func CreateToolImages(manifest model.Manifest, release string, dryrun bool) error {
	log.Infof("*** Creating a new builder image")
	repo := "tools"

	sedString := "s/BRANCH=.*/BRANCH=release-" + release + "/"
	cmd := util.VerboseCommand("sed", "-i", sedString, "docker/build-tools/build-and-push.sh")
	cmd.Dir = manifest.RepoDir(repo)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update BRANCH: %v", err)
	}
	log.Infof("*** New builder image PR created in tools repo.")
	return nil
}
