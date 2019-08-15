package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/howardjohn/istio-release/util"
	"github.com/pkg/errors"
	"istio.io/pkg/log"
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
	manifest, err := readManifest(manifestFile)
	if err != nil {
		exit(err, "failed to unmarshal manifest")
	}

	if manifest.WorkingDirectory == "" {
		manifest.WorkingDirectory = setupWorkDir()
	}

	//if err := Sources(manifest); err != nil {
	//	exit(err)
	//}
	log.Infof("Fetched all sources, setup working directory at %v", path.Join(manifest.WorkingDirectory, "work"))

	if err := Build(manifest); err != nil {
		exit(err)
	}

	log.Infof("Built release at %v", manifest.WorkingDirectory)
}

const dockerImage = "gcr.io/istio-testing/istio-builder:v20190807-7d818206"

func Build(manifest Manifest) error {
	//return util.Docker(dockerImage, path.Join(manifest.WorkingDirectory, "work"), "ls")
	cmd := exec.Command("make", "V=1", "build")
	//cmd := exec.Command("bash", "-c", "echo $GOPATH")
	cmd.Env =  os.Environ()
	cmd.Env = append(cmd.Env, "GOPATH="+path.Join(manifest.WorkingDirectory, "work"))
	cmd.Env = append(cmd.Env, "TAG=tag")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = path.Join(manifest.WorkingDirectory, "work", "src", "istio.io", "istio")
	//out, err := cmd.CombinedOutput()
	//log.Infof("Build: %v", string(out))
	//return err
	return cmd.Run()
}

func Sources(manifest Manifest) error {
	for _, dependency := range manifest.Dependencies {
		if err := dependency.Resolve(path.Join(manifest.WorkingDirectory, "sources")); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to resolve: %+v", dependency))
		}
		log.Infof("Resolved %v", dependency.Repo)
		src := path.Join(manifest.WorkingDirectory, "sources", fmt.Sprintf("%s-%s", dependency.Repo, dependency.Ref()))
		dst := path.Join(manifest.WorkingDirectory, "work", "src", "istio.io", dependency.Repo)
		if err := util.CopyDir(src, dst); err != nil {
			return fmt.Errorf("failed to copy dependency %v to working directory: %v", dependency.Repo, err)
		}
	}
	return nil
}

func readManifest(manifestFile string) (Manifest, error) {

	return Manifest{
		WorkingDirectory: "/tmp/istio-release849212648",
		Dependencies: []Dependency{
			{
				Org:    "istio",
				Repo:   "istio",
				Branch: "master",
			},
			//{
			//	Org:  "istio",
			//	Repo: "installer",
			//	Sha:  "b45de18499220e85e067cc8a71155e2af79cf170",
			//},
		},
	}, nil
	//by, err := ioutil.ReadFile(manifestFile)
	//if err != nil {
	//	exit(err, "failed to read manifest file")
	//}
	//manifest := Manifest{}
	//err := yaml.Unmarshal(by, &manifest)
	//return manifest, err
}

type Manifest struct {
	Dependencies     []Dependency `json:"dependencies"`
	WorkingDirectory string
}

type Dependency struct {
	Org    string `json:"org"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Sha    string `json:"sha"`
}

func (d Dependency) Ref() string {
	ref := d.Branch
	if d.Sha != "" {
		ref = d.Sha
	}
	return ref
}

func (d Dependency) Resolve(dir string) error {
	url := fmt.Sprintf("https://github.com/%s/%s/archive/%s.tar.gz", d.Org, d.Repo, d.Ref())
	//file := strings.ReplaceAll(d.Repo, "/", "~") + ".tar.gz"
	return util.Download(url, dir)
}
