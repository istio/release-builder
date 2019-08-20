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
	for _, s := range log.Scopes() {
		s.SetLogCallers(true)
	}
	manifest, err := readManifest("")
	if err != nil {
		log.Fatalf("failed to unmarshal manifest: %v", err)
	}

	if manifest.WorkingDirectory == "" {
		manifest.WorkingDirectory = setupWorkDir()

	}
	if err := pkg.Sources(manifest); err != nil {
		log.Fatalf("failed to fetch sources: %v", err)
	}
	log.Infof("Fetched all sources, setup working directory at %v", path.Join(manifest.WorkingDirectory, "work"))

	if err := pkg.Build(manifest); err != nil {
		log.Fatalf("failed to build: %v", err)
	}
	log.Infof("Build complete")

	if err := pkg.Package(manifest); err != nil {
		log.Fatalf("failed to package: %v", err)
	}

	log.Infof("Built release at %v", manifest.WorkingDirectory)
}

func readManifest(manifestFile string) (model.Manifest, error) {

	return model.Manifest{
		//WorkingDirectory: "/tmp/istio-release365668469",
		Version: "1.3.0",
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
