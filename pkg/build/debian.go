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
	"path"

	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// Debian produces a debian package just for the sidecar
func Debian(manifest model.Manifest) error {
	if err := util.RunMake(manifest, "istio", nil, "deb/fpm"); err != nil {
		return fmt.Errorf("failed to build sidecar.deb: %v", err)
	}
	if err := util.CopyFile(path.Join(manifest.RepoOutDir("istio"), "istio-sidecar.deb"), path.Join(manifest.OutDir(), "deb", "istio-sidecar.deb")); err != nil {
		return fmt.Errorf("failed to package istio-sidecar.deb: %v", err)
	}
	if err := util.CreateSha(path.Join(manifest.OutDir(), "deb", "istio-sidecar.deb")); err != nil {
		return fmt.Errorf("failed to package istio-sidecar.deb: %v", err)
	}
	return nil
}
