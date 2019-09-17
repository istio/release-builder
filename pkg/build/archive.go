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
	"os"
	"path"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"
)

// Archive creates the release archive that users will download. This includes the installation templates,
// istioctl, and various tools.
func Archive(manifest model.Manifest) error {
	// First, build all variants of istioctl (linux, osx, windows)
	if err := util.RunMake(manifest, "istio", nil, "istioctl-all", "istioctl.completion"); err != nil {
		return fmt.Errorf("failed to make istioctl: %v", err)
	}

	// We build archives for each arch. These contain the same thing except arch specific istioctl
	for _, arch := range []string{"linux", "osx", "win"} {
		out := path.Join(manifest.Directory, "work", "archive", arch, fmt.Sprintf("istio-%s", manifest.Version))
		if err := os.MkdirAll(out, 0750); err != nil {
			return err
		}

		// Some files we just directly copy into the release archive
		directCopies := []string{
			"LICENSE",
			"README.md",

			// Setup tools. The tools/ folder contains a bunch of extra junk, so just select exactly what we want
			"tools/convert_RbacConfig_to_ClusterRbacConfig.sh",
			"tools/packaging/common/istio-iptables.sh",
			"tools/dump_kubernetes.sh",
		}
		for _, file := range directCopies {
			if err := util.CopyFile(path.Join(manifest.RepoDir("istio"), file), path.Join(out, file)); err != nil {
				return err
			}
			return nil
		}

		// Set up install and samples. We filter down to only some file patterns
		// TODO - clean this up. We probably include files we don't want and exclude files we do want.
		includePatterns := []string{"*.yaml", "*.md", "cleanup.sh", "*.txt", "*.pem", "*.conf", "*.tpl", "*.json"}
		if err := util.CopyDirFiltered(path.Join(manifest.RepoDir("istio"), "samples"), path.Join(out, "samples"), includePatterns); err != nil {
			return err
		}
		if err := util.CopyDirFiltered(path.Join(manifest.RepoDir("istio"), "install"), path.Join(out, "install"), includePatterns); err != nil {
			return err
		}

		// Copy the istioctl binary over
		istioctlBinary := fmt.Sprintf("istioctl-%s", arch)
		if arch == "win" {
			istioctlBinary += ".exe"
		}
		if err := util.CopyFile(path.Join(manifest.GoOutDir(), istioctlBinary), path.Join(out, "bin", istioctlBinary)); err != nil {
			return err
		}

		// Create the archive from all the above files
		archive := path.Join(out, "..", fmt.Sprintf("istio-%s-%s.tar.gz", manifest.Version, arch))
		cmd := util.VerboseCommand("tar", "-czf", archive, fmt.Sprintf("istio-%s", manifest.Version))
		cmd.Dir = path.Join(out, "..")

		// Windows should use zip instead
		if arch == "win" {
			archive = fmt.Sprintf("istio-%s-%s.zip", manifest.Version, arch)
			cmd = util.VerboseCommand("zip", "-rq", archive, fmt.Sprintf("istio-%s", manifest.Version))
		}

		if err := cmd.Run(); err != nil {
			return err
		}

		// Copy files over to the output directory
		archivePath := path.Join(manifest.WorkDir(), "archive", arch, archive)
		dest := path.Join(manifest.OutDir(), archive)
		if err := util.CopyFile(archivePath, dest); err != nil {
			return fmt.Errorf("failed to package %v release archive: %v", arch, err)
		}

		// Create a SHA of the archive
		if err := util.CreateSha(dest); err != nil {
			return fmt.Errorf("failed to package %v: %v", dest, err)
		}
	}
	return nil
}
