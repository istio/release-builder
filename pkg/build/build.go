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
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/ghodss/yaml"

	"istio.io/pkg/log"

	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// Build will create all artifacts required by the manifest
// This assumes the working directory has been setup and sources resolved.
func Build(manifest model.Manifest) error {
	if _, f := manifest.BuildOutputs[model.Docker]; f {
		if err := Docker(manifest); err != nil {
			return fmt.Errorf("failed to build Docker: %v", err)
		}
	}

	if err := SanitizeAllCharts(manifest); err != nil {
		return fmt.Errorf("failed to sanitize charts")
	}

	if _, f := manifest.BuildOutputs[model.Helm]; f {
		if err := Helm(manifest); err != nil {
			return fmt.Errorf("failed to build Helm: %v", err)
		}
	}

	if _, f := manifest.BuildOutputs[model.Debian]; f {
		if err := Debian(manifest); err != nil {
			return fmt.Errorf("failed to build Debian: %v", err)
		}
	}

	if _, f := manifest.BuildOutputs[model.Archive]; f {
		if err := Archive(manifest); err != nil {
			return fmt.Errorf("failed to build Archive: %v", err)
		}
	}

	// Bundle all sources used in the build
	cmd := util.VerboseCommand("tar", "-czf", "out/sources.tar.gz", "sources")
	cmd.Dir = path.Join(manifest.Directory)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bundle sources: %v", err)
	}

	if err := writeManifest(manifest, manifest.OutDir()); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}

	if err := writeLicense(manifest); err != nil {
		return fmt.Errorf("failed to package license file: %v", err)
	}

	return nil
}

// writeLicense copies the complete list of licenses for all dependant repos
func writeLicense(manifest model.Manifest) error {
	if err := os.Mkdir(filepath.Join(manifest.OutDir(), "licenses"), 0750); err != nil {
		return fmt.Errorf("failed to create license dir: %v", err)
	}
	for repo := range manifest.Dependencies.Get() {
		src := filepath.Join(manifest.RepoDir(repo), "licenses")
		// Just skip these, we can fail in the validation tests afterwards for repos we expect license for
		if _, err := os.Stat(src); os.IsNotExist(err) {
			log.Warnf("skipping license for %v", repo)
			continue
		}
		// Package as a tar.gz since there are hundreds of files
		cmd := util.VerboseCommand("tar", "-czf", filepath.Join(manifest.OutDir(), "licenses", repo+".tar.gz"), src)
		cmd.Dir = path.Join(manifest.Directory)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to compress license: %v", err)
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
