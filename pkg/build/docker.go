package build

import (
	"fmt"
	"path"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"
)

// Docker builds all docker images and outputs them as tar.gz files
// docker.save in the repos does most of the work, we just need to call this and copy the files over
func Docker(manifest model.Manifest) error {
	// Build both default and distroless variants
	if err := util.RunMake(manifest, "istio", []string{"DOCKER_BUILD_VARIANTS=default distroless"}, "docker.save"); err != nil {
		return fmt.Errorf("failed to create docker archives: %v", err)
	}
	if err := util.RunMake(manifest, "cni", nil, "docker.save"); err != nil {
		return fmt.Errorf("failed to create cni docker archives: %v", err)
	}
	if err := util.CopyDir(path.Join(manifest.GoOutDir(), "docker"), path.Join(manifest.OutDir(), "docker")); err != nil {
		return fmt.Errorf("failed to package docker images: %v", err)
	}
	return nil
}
