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

	// Docker
	istioOut := path.Join(manifest.WorkingDirectory, "work", "out", "linux_amd64", "release")
	if err := util.CopyDir(path.Join(istioOut, "docker"), path.Join(out, "docker")); err != nil {
		return fmt.Errorf("failed to package docker images: %v", err)
	}

	// Helm
	if err := util.CopyDir(path.Join(manifest.WorkingDirectory, "work", "helm", "packages"), path.Join(out, "charts")); err != nil {
		return fmt.Errorf("failed to package helm chart: %v", err)
	}

	// Sidecar Debian
	if err := util.CopyFile(path.Join(istioOut, "istio-sidecar.deb"), path.Join(out, "deb", "istio-sidecar.deb")); err != nil {
		return fmt.Errorf("failed to package istio-sidecar.deb: %v", err)
	}
	if err := util.CreateSha(path.Join(out, "deb", "istio-sidecar.deb")); err != nil {
		return fmt.Errorf("failed to package istio-sidecar.deb: %v", err)
	}

	// Istioctl
	for _, arch := range []string{"linux", "osx", "win"} {
		archive := fmt.Sprintf("istio-%s-%s.tar.gz", manifest.Version, arch)
		if arch == "win" {
			archive = fmt.Sprintf("istio-%s-%s.zip", manifest.Version, arch)
		}
		archivePath := path.Join(manifest.WorkingDirectory, "work", "archive", arch, archive)
		if err := util.CopyFile(archivePath, path.Join(out, archive)); err != nil {
			return fmt.Errorf("failed to package %v release archive: %v", arch, err)
		}
	}

	// Manifest
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
	if err := buildDocker(manifest); err != nil {
		return err
	}
	if err := buildDeb(manifest); err != nil {
		return err
	}
	if err := buildCharts(manifest); err != nil {
		return err
	}
	if err := buildArchive(manifest); err != nil {
		return err
	}
	return nil
}

func buildDeb(manifest pkg.Manifest) error {
	return runMake(manifest, nil, "sidecar.deb")
}

func runMake(manifest pkg.Manifest, env []string, c ...string) error {
	cmd := exec.Command("make", c...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOPATH="+path.Join(manifest.WorkingDirectory, "work"))
	cmd.Env = append(cmd.Env, "TAG=tag")
	cmd.Env = append(cmd.Env, "GOBUILDFLAGS=-mod=vendor")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = path.Join(manifest.WorkingDirectory, "work", "src", "istio.io", "istio")
	return cmd.Run()
}

func buildArchive(manifest pkg.Manifest) error {
	if err := runMake(manifest, nil, "istioctl-all", "istioctl.completion"); err != nil {
		return fmt.Errorf("failed to make istioctl: %v", err)
	}
	for _, arch := range []string{"linux", "osx", "win"} {
		out := path.Join(manifest.WorkingDirectory, "work", "archive", arch, fmt.Sprintf("istio-%s", manifest.Version))
		if err := os.MkdirAll(out, 0750); err != nil {
			return err
		}
		istioOut := path.Join(manifest.WorkingDirectory, "work", "out", "linux_amd64", "release")
		istioSrc := path.Join(manifest.WorkingDirectory, "work", "src", "istio.io", "istio")

		srcToOut := func(p string) error {
			if err := util.CopyFile(path.Join(istioSrc, p), path.Join(out, p)); err != nil {
				return err
			}
			return nil
		}

		if err := srcToOut("LICENSE"); err != nil {
			return err
		}
		if err := srcToOut("README.md"); err != nil {
			return err
		}

		// Setup tools. The tools/ folder contains a bunch of extra junk, so just select exactly what we want
		if err := srcToOut("tools/convert_RbacConfig_to_ClusterRbacConfig.sh"); err != nil {
			return err
		}
		if err := srcToOut("tools/packaging/common/istio-iptables.sh"); err != nil {
			return err
		}
		if err := srcToOut("tools/dump_kubernetes.sh"); err != nil {
			return err
		}

		// Set up install and samples. We filter down to only some file patterns
		// TODO - clean this up. We probably include files we don't want and exclude files we do want.
		includePatterns := []string{"*.yaml", "*.md", "cleanup.sh", "*.txt", "*.pem", "*.conf", "*.tpl", "*.json"}
		if err := copyDirFiltered(path.Join(istioSrc, "samples"), path.Join(out, "samples"), includePatterns); err != nil {
			return err
		}
		if err := copyDirFiltered(path.Join(istioSrc, "install"), path.Join(out, "install"), includePatterns); err != nil {
			return err
		}

		istioctlArch := fmt.Sprintf("istioctl-%s", arch)
		// TODO make windows use zip
		if arch == "win" {
			istioctlArch += ".exe"
		}
		if err := util.CopyFile(path.Join(istioOut, istioctlArch), path.Join(out, "bin", istioctlArch)); err != nil {
			return err
		}

		if arch == "win" {
			archive := fmt.Sprintf("istio-%s-%s.zip", manifest.Version, arch)
			cmd := util.VerboseCommand("zip", "-rq", archive, fmt.Sprintf("istio-%s", manifest.Version))
			cmd.Dir = path.Join(out, "..")
			if err := cmd.Run(); err != nil {
				return err
			}
		} else {
			archive := path.Join(out, "..", fmt.Sprintf("istio-%s-%s.tar.gz", manifest.Version, arch))
			cmd := util.VerboseCommand("tar", "-czf", archive, fmt.Sprintf("istio-%s", manifest.Version))
			cmd.Dir = path.Join(out, "..")
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDirFiltered(src, dst string, include []string) error {
	if err := util.CopyDir(src, dst); err != nil {
		return err
	}
	if err := filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		fname := filepath.Base(path)
		for _, pattern := range include {
			if matched, _ := filepath.Match(pattern, fname); matched {
				// It matches one of the patterns, so stop early
				return nil
			}
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove filted file %v: %v", path, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to filter: %v", err)
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
	if err := runMake(manifest, []string{`DOCKER_BUILD_VARIANTS="default distroless"`}, "docker.save"); err != nil {
		return fmt.Errorf("failed to create docker archives: %v", err)
	}
	return nil
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
