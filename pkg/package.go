package pkg

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/ghodss/yaml"
	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/util"
)

func Package(manifest model.Manifest) error {
	out := path.Join(manifest.WorkingDirectory, "out")

	// Docker
	istioOut := path.Join(manifest.WorkingDirectory, "work", "out", "linux_amd64", "release")
	if err := util.CopyDir(path.Join(istioOut, "docker"), path.Join(out, "docker")); err != nil {
		return fmt.Errorf("failed to package docker images: %v", err)
	}

	// Helm
	if err := util.CopyDir(path.Join(manifest.WorkingDirectory, "work", "helm", "packages"), path.Join(out, "charts")); err != nil {
		return fmt.Errorf("failed to package helm chart: %v", err)
	}

	// Sidecar Debian
	if err := util.CopyFile(path.Join(istioOut, "istio-sidecar.deb"), path.Join(out, "deb", "istio-sidecar.deb")); err != nil {
		return fmt.Errorf("failed to package istio-sidecar.deb: %v", err)
	}
	if err := util.CreateSha(path.Join(out, "deb", "istio-sidecar.deb")); err != nil {
		return fmt.Errorf("failed to package istio-sidecar.deb: %v", err)
	}

	// Istioctl
	for _, arch := range []string{"linux", "osx", "win"} {
		archive := fmt.Sprintf("istio-%s-%s.tar.gz", manifest.Version, arch)
		if arch == "win" {
			archive = fmt.Sprintf("istio-%s-%s.zip", manifest.Version, arch)
		}
		archivePath := path.Join(manifest.WorkingDirectory, "work", "archive", arch, archive)
		if err := util.CopyFile(archivePath, path.Join(out, archive)); err != nil {
			return fmt.Errorf("failed to package %v release archive: %v", arch, err)
		}
	}

	// Manifest
	if err := writeManifest(manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}

	return nil
}

func writeManifest(manifest model.Manifest) error {
	// TODO we should replace indirect refs with SHA (in other part of code)
	yml, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %v", err)
	}
	if err := ioutil.WriteFile(path.Join(manifest.WorkingDirectory, "out", "manifest.yaml"), yml, 0640); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}
	return nil
}
