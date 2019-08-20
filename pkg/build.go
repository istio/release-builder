package pkg

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/util"

	"istio.io/pkg/log"
)

func Build(manifest model.Manifest) error {
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

func buildDeb(manifest model.Manifest) error {
	return runMake(manifest, nil, "sidecar.deb")
}

func buildCharts(manifest model.Manifest) error {
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
			return fmt.Errorf("failed to sanitze chart %v: %v", chart, err)
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

func buildDocker(manifest model.Manifest) error {
	if err := runMake(manifest, []string{"DOCKER_BUILD_VARIANTS=default distroless"}, "docker.save"); err != nil {
		return fmt.Errorf("failed to create docker archives: %v", err)
	}
	return nil
}

func buildArchive(manifest model.Manifest) error {
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
		if err := util.CopyDirFiltered(path.Join(istioSrc, "samples"), path.Join(out, "samples"), includePatterns); err != nil {
			return err
		}
		if err := util.CopyDirFiltered(path.Join(istioSrc, "install"), path.Join(out, "install"), includePatterns); err != nil {
			return err
		}

		istioctlArch := fmt.Sprintf("istioctl-%s", arch)
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

func runMake(manifest model.Manifest, env []string, c ...string) error {
	cmd := exec.Command("make", c...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOPATH="+path.Join(manifest.WorkingDirectory, "work"))
	cmd.Env = append(cmd.Env, "TAG=tag")
	cmd.Env = append(cmd.Env, "GOBUILDFLAGS=-mod=vendor")
	cmd.Env = append(cmd.Env, env...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = path.Join(manifest.WorkingDirectory, "work", "src", "istio.io", "istio")
	log.Infof("Running make %v with env %v", strings.Join(c, " "), strings.Join(env, " "))
	return cmd.Run()
}

func sanitizeChart(s string, manifest model.Manifest) error {
	// TODO improve this to not use raw string handling of yaml
	currentVersion, err := ioutil.ReadFile(path.Join(s, "Chart.yaml"))
	if err != nil {
		return err
	}
	chart := make(map[string]interface{})
	if err := yaml.Unmarshal(currentVersion, &chart); err != nil {
		log.Errorf("unmarshal failed for Chart.yaml: %v", string(currentVersion))
		return fmt.Errorf("failed to unmarshal chart: %v", err)
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
