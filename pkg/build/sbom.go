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

package build

import (
	"fmt"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// Sbom generates Software Bill Of Materials for istio repo in an SPDX readable format.
func Sbom(manifest model.Manifest) error {
	// Retrieve istio repository path to run the sbom generator
	istioRepoDir := manifest.RepoDir("istio")
	outDir := manifest.OutDir()

	// Run spdx sbom generator to generate the software bill of materials(SBOM) for istio
	log.Infof("Running spdx sbom generator for istio repository")
	if err := util.VerboseCommand("docker",
		"run", "--rm", "-v", istioRepoDir+":/repository", "-v", outDir+":/out", "spdx/spdx-sbom-generator", "--include-license-text",
		"--output-dir", "/out", "--path", "/repository").Run(); err != nil {
		return fmt.Errorf("couldn't generate sbom for istio: %v", err)
	}
	return nil
}
