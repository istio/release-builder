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

	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// Archive creates the release archive that users will download. This includes the installation templates,
// istioctl, and various tools.
func Archive(manifest model.Manifest) error {
	// First, build all variants of istioctl (linux, osx, windows). gen-charts is required for manifests compiled in to istioctl.
	if err := util.RunMake(manifest, "istio", nil, "gen-charts", "istioctl-all", "istioctl.completion"); err != nil {
		return fmt.Errorf("failed to make istioctl: %v", err)
	}

	// We build archives for each arch. These contain the same thing except arch specific istioctl
	for _, arch := range []string{"linux-amd64", "linux-armv7", "linux-arm64", "osx", "win"} {
		out := path.Join(manifest.Directory, "work", "archive", arch, fmt.Sprintf("istio-%s", manifest.Version))
		if err := os.MkdirAll(out, 0750); err != nil {
			return err
		}

		// Some files we just directly copy into the release archive
		directCopies := []string{
			"LICENSE",
			"README.md",

			// Setup tools. The tools/ folder contains a bunch of extra junk, so just select exactly what we want
			"tools/dump_kubernetes.sh",
		}
		for _, file := range directCopies {
			if err := util.CopyFile(path.Join(manifest.RepoDir("istio"), file), path.Join(out, file)); err != nil {
				return err
			}
		}

		// Set up tools/certs. We filter down to only some file patterns
		includePatterns := []string{"README.md", "Makefile*", "common.mk"}
		if err := util.CopyDirFiltered(path.Join(manifest.RepoDir("istio"), "tools", "certs"), path.Join(out, "tools", "certs"), includePatterns); err != nil {
			return err
		}

		// Set up samples. We filter down to only some file patterns
		// TODO - clean this up. We probably include files we don't want and exclude files we do want.
		includePatterns = []string{"*.yaml", "*.md", "*.sh", "*.txt", "*.pem", "*.conf", "*.tpl", "*.json", "Makefile"}
		if err := util.CopyDirFiltered(path.Join(manifest.RepoDir("istio"), "samples"), path.Join(out, "samples"), includePatterns); err != nil {
			return err
		}

		manifestsDir := path.Join(out, "manifests")
		if err := os.MkdirAll(manifestsDir, 0755); err != nil {
			return err
		}
		if err := util.CopyDir(path.Join(manifest.RepoDir("istio"), "manifests", "charts"), manifestsDir); err != nil {
			return err
		}
		if err := util.CopyDir(path.Join(manifest.RepoDir("istio"), "manifests", "examples"), manifestsDir); err != nil {
			return err
		}
		if err := util.CopyDir(path.Join(manifest.RepoDir("istio"), "manifests", "profiles"), manifestsDir); err != nil {
			return err
		}

		if err := sanitizeTemplate(manifest, path.Join(out, "manifests/profiles/default.yaml")); err != nil {
			return fmt.Errorf("failed to sanitize operator charts")
		}
		if err := util.CopyDir(path.Join(manifest.RepoDir("istio"), "operator", "samples"), path.Join(out, "samples/operator")); err != nil {
			return err
		}

		// Write manifest
		if err := writeManifest(manifest, out); err != nil {
			return fmt.Errorf("failed to write manifest: %v", err)
		}

		// Copy the istioctl binary over
		istioctlBinary := fmt.Sprintf("istioctl-%s", arch)
		istioctlDest := "istioctl"
		if arch == "win" {
			istioctlBinary += ".exe"
			istioctlDest += ".exe"
		}
		if err := util.CopyFile(path.Join(manifest.RepoOutDir("istio"), istioctlBinary), path.Join(out, "bin", istioctlDest)); err != nil {
			return err
		}
		if err := os.Chmod(path.Join(out, "bin", istioctlDest), 0755); err != nil {
			return err
		}

		// Copy the istioctl completions files to the tools directory
		completionFiles := []string{"istioctl.bash", "_istioctl"}
		for _, file := range completionFiles {
			if err := util.CopyFile(path.Join(manifest.RepoOutDir("istio"), file), path.Join(out, "tools", file)); err != nil {
				return err
			}
		}

		if err := createArchive(arch, manifest, out); err != nil {
			return err
		}

		if err := createStandaloneIstioctl(arch, manifest, out); err != nil {
			return err
		}
	}
	return nil
}

func createStandaloneIstioctl(arch string, manifest model.Manifest, out string) error {
	var istioctlArchive string
	// Create a stand alone archive for istioctl
	// Windows should use zip, linux and osx tar
	if arch == "win" {
		istioctlArchive = fmt.Sprintf("istioctl-%s-%s.zip", manifest.Version, arch)
		if err := util.ZipFolder(path.Join(out, "bin", "istioctl.exe"), path.Join(out, "bin", istioctlArchive)); err != nil {
			return fmt.Errorf("failed to zip istioctl: %v", err)
		}
	} else {
		istioctlArchive = fmt.Sprintf("istioctl-%s-%s.tar.gz", manifest.Version, arch)
		icmd := util.VerboseCommand("tar", "-czf", istioctlArchive, "istioctl")
		icmd.Dir = path.Join(out, "bin")
		if err := icmd.Run(); err != nil {
			return fmt.Errorf("failed to tar istioctl: %v", err)
		}
	}
	// Copy files over to the output directory
	archivePath := path.Join(out, "bin", istioctlArchive)
	dest := path.Join(manifest.OutDir(), istioctlArchive)
	if err := util.CopyFile(archivePath, dest); err != nil {
		return fmt.Errorf("failed to package %v release archive: %v", arch, err)
	}

	// Create a SHA of the archive
	if err := util.CreateSha(dest); err != nil {
		return fmt.Errorf("failed to package %v: %v", dest, err)
	}
	return nil
}

func createArchive(arch string, manifest model.Manifest, out string) error {
	var archive string
	// Create the archive from all the above files
	// Windows should use zip, linux and osx tar
	if arch == "win" {
		archive = fmt.Sprintf("istio-%s-%s.zip", manifest.Version, arch)
		if err := util.ZipFolder(path.Join(out, "..", fmt.Sprintf("istio-%s", manifest.Version)), path.Join(out, "..", archive)); err != nil {
			return fmt.Errorf("failed to zip istioctl: %v", err)
		}
	} else {
		archive = fmt.Sprintf("istio-%s-%s.tar.gz", manifest.Version, arch)
		cmd := util.VerboseCommand("tar", "-czf", archive, fmt.Sprintf("istio-%s", manifest.Version))
		cmd.Dir = path.Join(out, "..")
		if err := cmd.Run(); err != nil {
			return err
		}
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
	return nil
}
