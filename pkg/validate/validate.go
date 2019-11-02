// Copyright Istio Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validate

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg"
	"istio.io/release-builder/pkg/model"
)

func NewReleaseInfo(release string) ReleaseInfo {
	tmpDir, err := ioutil.TempDir("/tmp", "release-test")
	if err != nil {
		panic(err)
	}
	log.Infof("test temporary dir at %s", tmpDir)

	manifest, err := pkg.ReadManifest(filepath.Join(release, "manifest.yaml"))
	if err != nil {
		panic(err)
	}

	if err := exec.Command("tar", "xvf", filepath.Join(release, fmt.Sprintf("istio-%s-linux.tar.gz", manifest.Version)), "-C", tmpDir).Run(); err != nil {
		log.Warnf("failed to unpackage release archive")
	}
	return ReleaseInfo{
		tmpDir:   tmpDir,
		manifest: manifest,
		archive:  filepath.Join(tmpDir, "istio-"+manifest.Version),
		release:  release,
	}
}

type ValidationFunction func(ReleaseInfo) error

type ReleaseInfo struct {
	tmpDir   string
	manifest model.Manifest
	archive  string
	release  string
}

func CheckRelease(release string) ([]string, []error) {
	r := NewReleaseInfo(release)
	checks := map[string]ValidationFunction{
		"IstioctlArchive":      TestIstioctlArchive,
		"IstioctlStandalone":   TestIstioctlStandalone,
		"HelmVersionsIstio":    TestHelmVersionsIstio,
		"HelmVersionsCni":      TestHelmVersionsCni,
		"TestDocker":           TestDocker,
		"HelmVersionsOperator": TestHelmVersionsOperator,
		"Manifest":             TestManifest,
		"Demo":                 TestDemo,
		"Licenses":             TestLicenses,
	}
	var errors []error
	var success []string
	for name, check := range checks {
		err := check(r)
		if err != nil {
			errors = append(errors, fmt.Errorf("check %v failed: %v", name, err))
		} else {
			success = append(success, name)
		}
	}
	return success, errors
}

func TestIstioctlArchive(r ReleaseInfo) error {
	// Check istioctl from archive
	buf := &bytes.Buffer{}
	cmd := exec.Command(filepath.Join(r.archive, "bin", "istioctl"), "version", "--remote=false", "--short")
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		return err
	}
	got := strings.TrimSpace(buf.String())
	if got != r.manifest.Version {
		return fmt.Errorf("istioctl version output incorrect, got %v expected %v", got, r.manifest.Version)
	}
	return nil
}

func TestIstioctlStandalone(r ReleaseInfo) error {
	// Check istioctl from stand-alone archive
	istioctlArchivePath := filepath.Join(r.release, fmt.Sprintf("istioctl-%s-linux.tar.gz", r.manifest.Version))
	if err := exec.Command("tar", "xvf", istioctlArchivePath, "-C", r.tmpDir).Run(); err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	cmd := exec.Command(filepath.Join(r.tmpDir, "istioctl"), "version", "--remote=false", "--short")
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		return err
	}
	got := strings.TrimSpace(buf.String())
	if got != r.manifest.Version {
		return fmt.Errorf("istioctl version output incorrect, got %v expected %v", got, r.manifest.Version)
	}
	return nil
}

type GenericMap struct {
	data map[string]interface{}
}

func (g GenericMap) Path(path []string) interface{} {
	current := g.data
	for _, p := range path {
		switch v := current[p].(type) {
		case string:
			return v
		case map[string]interface{}:
			current = v
		default:
			panic("expected map or string")
		}
	}
	return nil
}

func getValues(path string) map[string]interface{} {
	values, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var typedValues map[string]interface{}
	if err := yaml.Unmarshal(values, &typedValues); err != nil {
		panic(err)
	}
	return typedValues
}

func TestHelmVersionsIstio(r ReleaseInfo) error {
	checks := []string{
		"install/kubernetes/helm/istio/values.yaml",
		"install/kubernetes/helm/istio-init/values.yaml",
	}
	for _, f := range checks {
		values := getValues(filepath.Join(r.archive, f))
		tag := GenericMap{values}.Path([]string{"global", "tag"})
		if tag != r.manifest.Version {
			return fmt.Errorf("archive tag incorrect, got %v expected %v", tag, r.manifest.Version)
		}
		hub := GenericMap{values}.Path([]string{"global", "hub"})
		if hub != r.manifest.Docker {
			return fmt.Errorf("hub incorrect, got %v expected %v", hub, r.manifest.Docker)
		}
	}
	return nil
}

func TestDocker(r ReleaseInfo) error {
	expected := []string{"pilot-distroless", "pilot", "install-cni", "proxyv2", "proxyv2-distroless", "operator"}
	found := map[string]struct{}{}
	d, err := ioutil.ReadDir(filepath.Join(r.release, "docker"))
	if err != nil {
		return fmt.Errorf("failed to read docker dir: %v", err)
	}
	for _, i := range d {
		found[i.Name()] = struct{}{}
	}
	for _, i := range expected {
		image := i + ".tar.gz"
		if _, f := found[image]; !f {
			return fmt.Errorf("expected docker image %v, but had %v", image, found)
		}
	}
	return nil
}

func TestHelmVersionsCni(r ReleaseInfo) error {
	cniChecks := []string{
		"install/kubernetes/helm/istio-cni/values.yaml",
		"install/kubernetes/helm/istio-cni/values_gke.yaml",
	}
	for _, f := range cniChecks {
		values := getValues(filepath.Join(r.archive, f))
		tag := GenericMap{values}.Path([]string{"tag"})
		if tag != r.manifest.Version {
			return fmt.Errorf("archive tag incorrect, got %v expected %v", tag, r.manifest.Version)
		}
		hub := GenericMap{values}.Path([]string{"hub"})
		if hub != r.manifest.Docker {
			return fmt.Errorf("hub incorrect, got %v expected %v", hub, r.manifest.Docker)
		}
	}
	return nil
}

func TestHelmVersionsOperator(r ReleaseInfo) error {
	operatorChecks := []string{
		"install/kubernetes/operator/profiles/default.yaml",
	}
	for _, f := range operatorChecks {
		values := getValues(filepath.Join(r.archive, f))
		tag := GenericMap{values}.Path([]string{"spec", "tag"})
		if tag != r.manifest.Version {
			return fmt.Errorf("archive tag incorrect, got %v expected %v", tag, r.manifest.Version)
		}
		hub := GenericMap{values}.Path([]string{"spec", "hub"})
		if hub != r.manifest.Docker {
			return fmt.Errorf("hub incorrect, got %v expected %v", hub, r.manifest.Docker)
		}
	}
	return nil
}

func TestManifest(r ReleaseInfo) error {
	for _, repo := range []string{"api", "cni", "client-go", "istio", "operator", "pkg", "proxy"} {
		d, f := r.manifest.Dependencies.Get()[repo]
		if !f || d.Sha == "" {
			return fmt.Errorf("got empty SHA for %v", repo)
		}
	}
	if r.manifest.Directory != "" {
		return fmt.Errorf("expected manifest directory to be hidden, got %v", r.manifest.Directory)
	}
	return nil
}

func TestDemo(r ReleaseInfo) error {
	d, err := ioutil.ReadFile(filepath.Join(r.archive, "install/kubernetes/istio-demo.yaml"))
	if err != nil {
		return err
	}
	var parsed interface{}
	// Just validate the demo is valid yaml at least
	if err := yaml.Unmarshal(d, &parsed); err != nil {
		return err
	}
	return nil
}

func TestLicenses(r ReleaseInfo) error {
	l, err := ioutil.ReadDir(filepath.Join(r.release, "licenses"))
	if err != nil {
		return err
	}
	// Expect to find license folders for these repos
	expect := map[string]struct{}{
		"istio":         {},
		"gogo-genproto": {},
		"client-go":     {},
		"tools":         {},
		"test-infra":    {},
	}

	for _, repo := range l {
		delete(expect, repo.Name())
	}

	if len(expect) > 0 {
		return fmt.Errorf("failed to find licenses for: %v", expect)
	}
	return nil
}
