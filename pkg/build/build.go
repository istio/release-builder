package build

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/ghodss/yaml"
	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"
)

func Build(manifest model.Manifest) error {
	if manifest.ShouldBuild(model.Docker) {
		if err := Docker(manifest); err != nil {
			return fmt.Errorf("failed to build Docker: %v", err)
		}
	}

	if manifest.ShouldBuild(model.Helm) {
		if err := Helm(manifest); err != nil {
			return fmt.Errorf("failed to build Helm: %v", err)
		}
	}

	if manifest.ShouldBuild(model.Debian) {
		if err := Debian(manifest); err != nil {
			return fmt.Errorf("failed to build Debian: %v", err)
		}
	}

	if manifest.ShouldBuild(model.Archive) {
		if err := Archive(manifest); err != nil {
			return fmt.Errorf("failed to build Archive: %v", err)
		}
	}

	// Bundle sources
	cmd := util.VerboseCommand("tar", "-czf", "out/sources.tar.gz", "sources")
	cmd.Dir = path.Join(manifest.Directory)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bundle sources: %v", err)
	}

	// Manifest
	if err := writeManifest(manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}

	// Full license
	if err := writeLicense(manifest); err != nil {
		return fmt.Errorf("failed to package license file: %v", err)
	}

	return nil
}

func writeLicense(manifest model.Manifest) interface{} {
	cmd := util.VerboseCommand("go", "run", "tools/license/get_dep_licenses.go")
	cmd.Dir = manifest.RepoDir("istio")
	o, err := os.Create(path.Join(manifest.OutDir(), "LICENSES"))
	if err != nil {
		return err
	}
	cmd.Stdout = o
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func writeManifest(manifest model.Manifest) error {
	// TODO we should replace indirect refs with SHA (in other part of code)
	yml, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %v", err)
	}
	if err := ioutil.WriteFile(path.Join(manifest.OutDir(), "manifest.yaml"), yml, 0640); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}
	return nil
}
