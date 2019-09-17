package build

import (
	"fmt"
	"path"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"
)

// Debian produces a debian package just for the sidecar
func Debian(manifest model.Manifest) error {
	if err := util.RunMake(manifest, "istio", nil, "sidecar.deb"); err != nil {
		return fmt.Errorf("failed to build sidecar.deb: %v", err)
	}
	if err := util.CopyFile(path.Join(manifest.GoOutDir(), "istio-sidecar.deb"), path.Join(manifest.OutDir(), "deb", "istio-sidecar.deb")); err != nil {
		return fmt.Errorf("failed to package istio-sidecar.deb: %v", err)
	}
	if err := util.CreateSha(path.Join(manifest.OutDir(), "deb", "istio-sidecar.deb")); err != nil {
		return fmt.Errorf("failed to package istio-sidecar.deb: %v", err)
	}
	return nil
}
