package main

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/howardjohn/istio-release/pkg"
	"github.com/howardjohn/istio-release/pkg/model"

	"istio.io/pkg/log"
)

func setupWorkDir() string {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "istio-release")
	if err != nil {
		log.Fatalf("failed to create working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(tmpdir, "sources"), 0750); err != nil {
		log.Fatalf("failed to set up working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(tmpdir, "work"), 0750); err != nil {
		log.Fatalf("failed to set up working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(tmpdir, "out"), 0750); err != nil {
		log.Fatalf("failed to set up working directory: %v", err)
	}
	return tmpdir
}

func main() {
	manifest, err := readManifest("")
	if err != nil {
		log.Fatalf("failed to unmarshal manifest: %v", err)
	}

	if manifest.Directory == "" {
		manifest.Directory = setupWorkDir()

	}
	if err := pkg.Sources(manifest); err != nil {
		log.Fatalf("failed to fetch sources: %v", err)
	}
	log.Infof("Fetched all sources, setup working directory at %v", manifest.WorkDir())

	if err := pkg.Build(manifest); err != nil {
		log.Fatalf("failed to build: %v", err)
	}

	log.Infof("Built release at %v", manifest.OutDir())
}

func readManifest(manifestFile string) (model.Manifest, error) {

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
		//BuildOutputs: []model.BuildOutput{model.Helm},
		BuildOutputs: model.AllBuildOutputs,
	}, nil
	//by, err := ioutil.ReadFile(manifestFile)
	//if err != nil {
	//	log.Fatalf("failed to read manifest file: %v", err, )
	//}
	//manifest := pkg.Manifest{}
	//err := yaml.Unmarshal(by, &manifest)
	//return manifest, err
}
