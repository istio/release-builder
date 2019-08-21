package pkg

import (
	"fmt"
	"io/ioutil"

	"github.com/ghodss/yaml"
	"github.com/howardjohn/istio-release/pkg/model"
)

func ReadManifest(manifestFile string) (model.Manifest, error) {
	manifest := model.Manifest{}
	by, err := ioutil.ReadFile(manifestFile)
	if err != nil {
		return manifest, fmt.Errorf("failed to read manifest file: %v", err)
	}
	if err := yaml.Unmarshal(by, &manifest); err != nil {
		return manifest, fmt.Errorf("failed to unmarshal manifest file: %v", err)
	}
	manifest.BuildOutputs = []model.BuildOutput{model.Helm}
	return manifest, nil
}
