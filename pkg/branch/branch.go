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
	"io/ioutil"
	"path"
	"strconv"

	"github.com/ghodss/yaml"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// Branch will run a specific automation step. Each step may include
// multiple items which can be done before another item has a dependency
// on an intem in the given step.
// This function assumes the working directory has been setup and sources resolved.
func Branch(manifest model.Manifest, step int, dryrun bool) error {
	if err := writeManifest(manifest, manifest.OutDir()); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}

	release := manifest.Version

	switch step {
	case 1:
		if err := UpdateDependencies(manifest, dryrun); err != nil {
			return fmt.Errorf("failed to update dependencies: %v", err)
		}
	case 2:
		if err := CreateBranches(manifest, release, dryrun); err != nil {
			return fmt.Errorf("failed to create branches: %v", err)
		}
	case 3:
		// Can't do SetupProw locally as I don't have creds. Need to do manually for now.
		// Should be OK once this is running in release-builder jobs
		if err := SetupProw(manifest, release, dryrun); err != nil {
			return fmt.Errorf("failed to setup prow: %v", err)
		}
	case 4:
		if err := CreateToolImages(manifest, release, dryrun); err != nil {
			return fmt.Errorf("failed to create  tools images: %v", err)
		}
		if err := UpdateCommonFiles(manifest, release, dryrun); err != nil {
			return fmt.Errorf("failed to update common-files specification: %v", err)
		}
		if err := UpdateCodeOwners(manifest, release, dryrun); err != nil {
			return fmt.Errorf("failed to update CODEOWNERS: %v", err)
		}
		if err := StopPublishingLatest(manifest, release, dryrun); err != nil {
			return fmt.Errorf("failed to stop publishing latest: %v", err)
		}
		if err := IstioReleaseBuilderUpdates(manifest, release, dryrun); err != nil {
			return fmt.Errorf("failed to update common-files: %v", err)
		}
	case 5:
		if err := UpdateCommonFilesCommon(manifest, release, dryrun); err != nil {
			return fmt.Errorf("failed to update common-files: %v", err)
		}
	}

	// Determine if there are any changes in the repos and create PRs.
	// CreatePR() will use dryrun to determine if PRs are created
	for repo, dep := range manifest.Dependencies.Get() {
		if dep == nil {
			// Missing a dependency is not always a failure; many are optional dependencies just for
			// tagging.
			log.Infof("skipping missing dependency: %v", repo)
			continue
		}

		log.Infof("*** Checking repo %s", repo)

		prName := "Automated branching step " + strconv.Itoa(step)
		if step > 2 {
			prName = "[release-" + release + "] " + prName
		}
		if err := util.CreatePR(manifest, repo, "automatedBranchStep"+strconv.Itoa(step),
			prName, dryrun); err != nil {
			return fmt.Errorf("failed PR creation: %v", err)
		}
	}
	return nil
}

// writeManifest will output the manifest to yaml
func writeManifest(manifest model.Manifest, dir string) error {
	yml, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %v", err)
	}
	if err := ioutil.WriteFile(path.Join(dir, "manifest.yaml"), yml, 0640); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}
	return nil
}
