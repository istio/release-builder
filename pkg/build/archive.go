package build

import (
	"fmt"
	"os"
	"path"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"
)

func Archive(manifest model.Manifest) error {
	if err := util.RunMake(manifest, "istio", nil, "istioctl-all", "istioctl.completion"); err != nil {
		return fmt.Errorf("failed to make istioctl: %v", err)
	}
	for _, arch := range []string{"linux", "osx", "win"} {
		out := path.Join(manifest.Directory, "work", "archive", arch, fmt.Sprintf("istio-%s", manifest.Version))
		if err := os.MkdirAll(out, 0750); err != nil {
			return err
		}

		srcToOut := func(p string) error {
			if err := util.CopyFile(path.Join(manifest.RepoDir("istio"), p), path.Join(out, p)); err != nil {
				return err
			}
			return nil
		}

		if err := srcToOut("LICENSE"); err != nil {
			return err
		}
		if err := srcToOut("README.md"); err != nil {
			return err
		}

		// Setup tools. The tools/ folder contains a bunch of extra junk, so just select exactly what we want
		if err := srcToOut("tools/convert_RbacConfig_to_ClusterRbacConfig.sh"); err != nil {
			return err
		}
		if err := srcToOut("tools/packaging/common/istio-iptables.sh"); err != nil {
			return err
		}
		if err := srcToOut("tools/dump_kubernetes.sh"); err != nil {
			return err
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

		istioctlArch := fmt.Sprintf("istioctl-%s", arch)
		if arch == "win" {
			istioctlArch += ".exe"
		}
		if err := util.CopyFile(path.Join(manifest.GoOutDir(), istioctlArch), path.Join(out, "bin", istioctlArch)); err != nil {
			return err
		}

		if arch == "win" {
			archive := fmt.Sprintf("istio-%s-%s.zip", manifest.Version, arch)
			cmd := util.VerboseCommand("zip", "-rq", archive, fmt.Sprintf("istio-%s", manifest.Version))
			cmd.Dir = path.Join(out, "..")
			if err := cmd.Run(); err != nil {
				return err
			}
		} else {
			archive := path.Join(out, "..", fmt.Sprintf("istio-%s-%s.tar.gz", manifest.Version, arch))
			cmd := util.VerboseCommand("tar", "-czf", archive, fmt.Sprintf("istio-%s", manifest.Version))
			cmd.Dir = path.Join(out, "..")
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	}
	for _, arch := range []string{"linux", "osx", "win"} {
		archive := fmt.Sprintf("istio-%s-%s.tar.gz", manifest.Version, arch)
		if arch == "win" {
			archive = fmt.Sprintf("istio-%s-%s.zip", manifest.Version, arch)
		}
		archivePath := path.Join(manifest.WorkDir(), "archive", arch, archive)
		dest := path.Join(manifest.OutDir(), archive)
		if err := util.CopyFile(archivePath, dest); err != nil {
			return fmt.Errorf("failed to package %v release archive: %v", arch, err)
		}
		if err := util.CreateSha(dest); err != nil {
			return fmt.Errorf("failed to package %v: %v", dest, err)
		}
	}
	return nil
}
