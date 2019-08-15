package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/howardjohn/istio-release/util"
)

const (
	manifestFile = "release/manifest.yaml"
)

func exit(err error, context ...string) {
	fmt.Printf("%s: %s", strings.Join(context, ": "), err.Error())
	os.Exit(1)
}

func setupWorkDir() string {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "istio-release")
	if err != nil {
		exit(err, "failed to create working directory")
	}
	if err := os.Mkdir(path.Join(tmpdir, "sources"), 0750); err != nil {
		exit(err, "failed to set up working directory")
	}
	if err := os.Mkdir(path.Join(tmpdir, "work"), 0750); err != nil {
		exit(err, "failed to set up working directory")
	}
	if err := os.Mkdir(path.Join(tmpdir, "out"), 0750); err != nil {
		exit(err, "failed to set up working directory")
	}
	return tmpdir
}

func main() {
	tmpdir := setupWorkDir()
	fmt.Println("Working directory:", tmpdir)

	manifestBytes, err := ioutil.ReadFile(manifestFile)
	if err != nil {
		exit(err, "failed to read manifest file")
	}
	manifest, err := readManifest(manifestBytes)
	if err != nil {
		exit(err, "failed to unmarshal manifest")
	}
	for _, dependency := range manifest.Dependencies {
		if err := dependency.Resolve(path.Join(tmpdir, "sources")); err != nil {
			exit(err, fmt.Sprintf("failed to resolve: %+v", dependency))
		}
	}
}

func readManifest(by []byte) (Manifest, error) {
	manifest := Manifest{}
	err := yaml.Unmarshal(by, &manifest)
	return manifest, err
}

type Manifest struct {
	Dependencies []Dependency `json:"dependencies"`
}

type Dependency struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Sha    string `json:"sha"`
}

func (d Dependency) Resolve(dir string) error {
	ref := d.Branch
	if d.Sha != "" {
		ref = d.Sha
	}
	url := fmt.Sprintf("https://github.com/%s/archive/%s.tar.gz", d.Repo, ref)
	file := strings.ReplaceAll(d.Repo, "/", "~") + ".tar.gz"
	return util.Download(url, path.Join(dir, file))
}
