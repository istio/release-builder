package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/howardjohn/istio-release/pkg"
	"github.com/howardjohn/istio-release/util"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

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
	if err := Sources(manifest); err != nil {
		log.Fatalf("failed to fetch sources: %v", err)
	}
	log.Infof("Fetched all sources, setup working directory at %v", path.Join(manifest.WorkingDirectory, "work"))

	if err := Build(manifest); err != nil {
		log.Fatalf("failed to build: %v", err)
	}
	log.Infof("Build complete")

	if err := Package(manifest); err != nil {
		log.Fatalf("failed to package: %v", err)
	}

	log.Infof("Built release at %v", manifest.WorkingDirectory)
}

func Package(manifest pkg.Manifest) error {
	out := path.Join(manifest.WorkingDirectory, "out")

	//istioOut := path.Join(manifest.WorkingDirectory, "work", "out", "linux_amd64", "release")
	//if err := util.CopyDir(path.Join(istioOut, "docker"), path.Join(out, "docker")); err != nil {
	//	return fmt.Errorf("failed to package docker images: %v", err)
	//}

	if err := util.CopyDir(path.Join(manifest.WorkingDirectory, "work", "helm", "packages"), path.Join(out, "charts")); err != nil {
		return fmt.Errorf("failed to package helm chart: %v", err)
	}
	if err := writeManifest(manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}
	return nil
}

func writeManifest(manifest pkg.Manifest) error {
	// TODO we should replace indirect refs with SHA (in other part of code)
	yml, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %v", err)
	}
	if err := ioutil.WriteFile(path.Join(manifest.WorkingDirectory, "out", "manifest.yaml"), yml, 0640); err != nil {
		return fmt.Errorf("failed to write manifest: %v", err)
	}
	return nil
}

func Build(manifest pkg.Manifest) error {
	//if err := buildDocker(manifest); err != nil {
	//	return err
	//}
	if err := buildCharts(manifest); err != nil {
		return err
	}
	return nil
}

func buildCharts(manifest pkg.Manifest) error {
	helm := path.Join(manifest.WorkingDirectory, "work", "helm")
	if err := os.MkdirAll(helm, 0750); err != nil {
		return fmt.Errorf("failed to setup helm directory: %v", err)
	}
	if err := os.MkdirAll(path.Join(helm, "packages"), 0750); err != nil {
		return fmt.Errorf("failed to setup helm directory: %v", err)
	}
	if err := exec.Command("helm", "--home", helm, "init", "--client-only").Run(); err != nil {
		return fmt.Errorf("failed to setup helm: %v", err)
	}

	// TODO: cni
	charts := []string{
		"istio/install/kubernetes/helm/istio",
		"istio/install/kubernetes/helm/istio-init",
	}
	for _, chart := range charts {
		if err := sanitizeChart(path.Join(manifest.WorkingDirectory, "work", "src", "istio.io", chart), manifest); err != nil {
			return err
		}
		cmd := exec.Command("helm", "--home", helm, "package", chart, "--destination", path.Join(helm, "packages"))
		cmd.Dir = path.Join(manifest.WorkingDirectory, "work", "src", "istio.io")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to package %v by `%v`: %v", chart, cmd.Args, err)
		}
	}
	return nil
}

func sanitizeChart(s string, manifest pkg.Manifest) error {
	// TODO improve this to not use raw string handling of yaml
	currentVersion, err := ioutil.ReadFile(path.Join(s, "Chart.yaml"))
	if err != nil {
		return err
	}
	chart := make(map[string]interface{})
	if err := yaml.Unmarshal(currentVersion, chart); err != nil {
		return err
	}
	// Getting the current version is a bit of a hack, we should have a more explicit way to handle this
	cv := chart["appVersion"].(string)
	if err := filepath.Walk(s, func(p string, info os.FileInfo, err error) error {
		fname := path.Base(p)
		if fname == "Chart.yaml" || fname == "values.yaml" {
			read, err := ioutil.ReadFile(p)
			if err != nil {
				return err
			}
			contents := string(read)
			for _, replacement := range []string{"appVersion", "version", "tag"} {
				before := fmt.Sprintf("%s: %s", replacement, cv)
				after := fmt.Sprintf("%s: %s", replacement, manifest.Version)
				contents = strings.ReplaceAll(contents, before, after)
			}

			err = ioutil.WriteFile(p, []byte(contents), 0)
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func buildDocker(manifest pkg.Manifest) error {
	// TODO: make distroless
	cmd := exec.Command("make", "docker.save")
	cmd.Env = os.Environ()
	// TODO: this uses modules instead of vendor for some reason
	cmd.Env = append(cmd.Env, "GOPATH="+path.Join(manifest.WorkingDirectory, "work"))
	cmd.Env = append(cmd.Env, "TAG=tag")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = path.Join(manifest.WorkingDirectory, "work", "src", "istio.io", "istio")
	return cmd.Run()
}

func Sources(manifest pkg.Manifest) error {
	for _, dependency := range manifest.Dependencies {
		if err := util.Clone(dependency, path.Join(manifest.WorkingDirectory, "sources", dependency.Repo)); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to resolve: %+v", dependency))
		}
		log.Infof("Resolved %v", dependency.Repo)
		src := path.Join(manifest.WorkingDirectory, "sources", dependency.Repo)
		dst := path.Join(manifest.WorkingDirectory, "work", "src", "istio.io", dependency.Repo)
		if err := util.CopyDir(src, dst); err != nil {
			return fmt.Errorf("failed to copy dependency %v to working directory: %v", dependency.Repo, err)
		}
	}
	return nil
}

func readManifest(manifestFile string) (pkg.Manifest, error) {

	return pkg.Manifest{
		//WorkingDirectory: "/tmp/istio-release365668469",
		Version: "1.3.0",
		Dependencies: []pkg.Dependency{
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
	//	log.Fatalf("failed to read manifest file: %v", err, )
	//}
	//manifest := pkg.Manifest{}
	//err := yaml.Unmarshal(by, &manifest)
	//return manifest, err
}
