package publish

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"
)

func Docker(manifest model.Manifest, hub string) error {
	dockerArchives, err := ioutil.ReadDir(path.Join(manifest.OutDir(), "docker"))
	if err != nil {
		return fmt.Errorf("failed to read docker output of release: %v", err)
	}
	for _, f := range dockerArchives {
		if !strings.HasSuffix(f.Name(), "tar.gz") {
			return fmt.Errorf("invalid image found in docker folder: %v", f.Name())
		}
		imageName, variant := getImageNameVariant(f.Name())
		if err := util.VerboseCommand("docker", "load", "-i", path.Join(manifest.OutDir(), "docker", f.Name())).Run(); err != nil {
			return fmt.Errorf("failed to load docker image %v: %v", f.Name(), err)
		}

		currentTag := fmt.Sprintf("istio/%s:%s%s", imageName, manifest.Version, variant)
		newTag := fmt.Sprintf("%s/%s:%s%s", hub, imageName, manifest.Version, variant)
		if err := util.VerboseCommand("docker", "tag", currentTag, newTag).Run(); err != nil {
			return fmt.Errorf("failed to load docker image %v: %v", currentTag, err)
		}

		if err := util.VerboseCommand("docker", "push", newTag).Run(); err != nil {
			return fmt.Errorf("failed to push docker image %v: %v", newTag, err)
		}
	}
	return nil
}

func getImageNameVariant(fname string) (string, string) {
	imageName := strings.Split(fname, ".")[0]
	if match, _ := filepath.Match("*-distroless", imageName); match {
		return strings.TrimSuffix(imageName, "-distroless"), "-distroless"
	}
	return imageName, ""
}
