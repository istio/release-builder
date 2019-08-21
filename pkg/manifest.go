package pkg

import "github.com/howardjohn/istio-release/pkg/model"

func ReadManifest(manifestFile string) (model.Manifest, error) {

	return model.Manifest{
		//Directory: "/tmp/istio-release365668469",
		Version: "master-20190820-09-16",
		Dependencies: []model.Dependency{
			{
				Org:    "istio",
				Repo:   "istio",
				Branch: "master",
			},
			{
				Org:  "istio",
				Repo: "cni",
				Sha:  "master",
			},
		},
		BuildOutputs: []model.BuildOutput{model.Helm},
		//BuildOutputs: model.AllBuildOutputs,
	}, nil
	//by, err := ioutil.ReadFile(manifestFile)
	//if err != nil {
	//	log.Fatalf("failed to read manifest file: %v", err, )
	//}
	//manifest := pkg.Manifest{}
	//err := yaml.Unmarshal(by, &manifest)
	//return manifest, err
}
