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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
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

	if err := exec.Command("tar", "xvf", filepath.Join(release, fmt.Sprintf("istio-%s-linux-amd64.tar.gz", manifest.Version)), "-C", tmpDir).Run(); err != nil {
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

func CheckRelease(release string) ([]string, string, []error) {
	if release == "" {
		return nil, "", []error{fmt.Errorf("--release must be passed")}
	}
	r := NewReleaseInfo(release)
	checks := map[string]ValidationFunction{
		"IstioctlArchive":      TestIstioctlArchive,
		"IstioctlStandalone":   TestIstioctlStandalone,
		"TestDocker":           TestDocker,
		"HelmVersionsIstio":    TestHelmVersionsIstio,
		"HelmVersionsOperator": TestHelmVersionsOperator,
		"Manifest":             TestManifest,
		"Licenses":             TestLicenses,
		"Grafana":              TestGrafana,
		"CompletionFiles":      TestCompletionFiles,
		"ProxyVersion":         TestProxyVersion,
		"Debian":               TestDebian,
		"Rpm":                  TestRpm,
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
	sb := strings.Builder{}
	if len(errors) > 0 {
		sb.WriteString(fmt.Sprintf("Checks failed. Release info: %+v", r))
		sb.WriteString("Files in release: \n")
		_ = filepath.Walk(r.release,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				sb.WriteString(fmt.Sprintf("- %s", path))
				return nil
			})
		sb.WriteString("\nFiles in archive: \n")
		_ = filepath.Walk(r.archive,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				sb.WriteString(fmt.Sprintf("- %s", path))
				return nil
			})
	}
	return success, sb.String(), errors
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
	istioctlArchivePath := filepath.Join(r.release, fmt.Sprintf("istioctl-%s-linux-amd64.tar.gz", r.manifest.Version))
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

func (g GenericMap) Path(path []string) (interface{}, error) {
	current := g.data
	var tmpList []interface{}
	for _, p := range path {
		val := current[p]
		// If the last path was a list, instead treat p as the index into that list
		if tmpList != nil {
			i, err := strconv.Atoi(p)
			if err != nil {
				return nil, fmt.Errorf("list requires integer path: %v in %v", p, path)
			}
			val = tmpList[i]
			tmpList = nil
		}
		switch v := val.(type) {
		case string:
			return v, nil
		case map[string]interface{}:
			current = v
		case []interface{}:
			tmpList = v
		default:
			return nil, fmt.Errorf("expected map or string, got %T for %v in %v", v, p, path)
		}
	}
	return nil, nil
}

func getValues(path string) (map[string]interface{}, error) {
	values, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var typedValues map[string]interface{}
	if err := yaml.Unmarshal(values, &typedValues); err != nil {
		return nil, err
	}
	return typedValues, nil
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

type DockerManifest struct {
	Config string `json:"Config"`
}

type DockerConfig struct {
	Config DockerConfigConfig `json:"config"`
}

type DockerConfigConfig struct {
	Env []string `json:"Env"`
}

func TestProxyVersion(r ReleaseInfo) error {
	image := filepath.Join(r.release, "docker", "proxyv2.tar.gz")
	if err := exec.Command("tar", "xvf", image, "-C", r.tmpDir).Run(); err != nil {
		log.Warnf("failed to unpackage release archive")
	}

	manifestBytes, err := ioutil.ReadFile(filepath.Join(r.tmpDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("couldn't read manifest: %v", err)
	}
	manifest := []DockerManifest{}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("failed to unmarshal manifest: %v", err)
	}

	configBytes, err := ioutil.ReadFile(filepath.Join(r.tmpDir, manifest[0].Config))
	if err != nil {
		return fmt.Errorf("couldn't read config: %v", err)
	}
	config := DockerConfig{}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	found := false
	for _, env := range config.Config.Env {
		sp := strings.Split(env, "=")
		if len(sp) != 2 {
			return fmt.Errorf("invalid env: %v", env)
		}

		if sp[0] == "ISTIO_META_ISTIO_VERSION" {
			found = true
			if sp[1] != r.manifest.Version {
				return fmt.Errorf("expected proxy version to be %v, got %v", r.manifest.Version, sp[1])
			}
		}
	}

	if !found {
		return fmt.Errorf("did not find proxy version variable")
	}

	return nil
}

func TestHelmVersionsIstio(r ReleaseInfo) error {
	checks := []string{
		"manifests/charts/gateways/istio-egress/values.yaml",
		"manifests/charts/gateways/istio-ingress/values.yaml",
		"manifests/charts/istio-cni/values.yaml",
		"manifests/charts/istio-control/istio-discovery/values.yaml",
		"manifests/charts/istio-operator/values.yaml",
		"manifests/charts/istiod-remote/values.yaml",
	}
	for _, f := range checks {
		values, err := getValues(filepath.Join(r.archive, f))
		if err != nil {
			return err
		}
		tag, err := GenericMap{values}.Path([]string{"global", "tag"})
		if err != nil {
			return fmt.Errorf("invalid path: %v: %v", f, err)
		}
		if tag != r.manifest.Version {
			return fmt.Errorf("archive tag incorrect: %v: got %v expected %v", f, tag, r.manifest.Version)
		}
		hub, err := GenericMap{values}.Path([]string{"global", "hub"})
		if err != nil {
			return fmt.Errorf("invalid path: %v: %v", f,err)
		}
		if hub != r.manifest.Docker {
			return fmt.Errorf("hub incorrect: %v: got %v expected %v", f, hub, r.manifest.Docker)
		}
	}
	return nil
}

func TestHelmVersionsOperator(r ReleaseInfo) error {
	operatorChecks := []string{
		"manifests/profiles/default.yaml",
	}
	for _, f := range operatorChecks {
		values, err := getValues(filepath.Join(r.archive, f))
		if err != nil {
			return err
		}
		tag, err := GenericMap{values}.Path([]string{"spec", "tag"})
		if err != nil {
			return fmt.Errorf("invalid path: %v", err)
		}
		if tag != r.manifest.Version {
			return fmt.Errorf("archive tag incorrect, got %v expected %v", tag, r.manifest.Version)
		}
		hub, err := GenericMap{values}.Path([]string{"spec", "hub"})
		if err != nil {
			return fmt.Errorf("invalid path: %v", err)
		}
		if hub != r.manifest.Docker {
			return fmt.Errorf("hub incorrect, got %v expected %v", hub, r.manifest.Docker)
		}
	}
	return nil
}

func TestManifest(r ReleaseInfo) error {
	for _, repo := range []string{"api", "client-go", "istio", "pkg", "proxy"} {
		d, f := r.manifest.Dependencies.Get()[repo]
		if d == nil {
			return fmt.Errorf("missing dependency: %v", repo)
		}
		if !f || d.Sha == "" {
			return fmt.Errorf("got empty SHA for %v", repo)
		}
	}
	if r.manifest.Directory != "" {
		return fmt.Errorf("expected manifest directory to be hidden, got %v", r.manifest.Directory)
	}
	return nil
}

func TestGrafana(r ReleaseInfo) error {
	created := map[string]struct{}{}
	dir, err := ioutil.ReadDir(path.Join(r.release, "grafana"))
	if err != nil {
		return err
	}
	for _, db := range dir {
		created[strings.TrimSuffix(db.Name(), ".json")] = struct{}{}
	}
	manifest := map[string]struct{}{}
	for dashboard := range r.manifest.GrafanaDashboards {
		manifest[dashboard] = struct{}{}
	}
	if !reflect.DeepEqual(created, manifest) {
		return fmt.Errorf("dashboards out of sync, release contains %+v, manifest contains %+v", created, manifest)
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
		"istio.tar.gz":         {},
		"gogo-genproto.tar.gz": {},
		"client-go.tar.gz":     {},
		"tools.tar.gz":         {},
		"test-infra.tar.gz":    {},
	}

	for _, repo := range l {
		delete(expect, repo.Name())
	}

	if len(expect) > 0 {
		return fmt.Errorf("failed to find licenses for: %v", expect)
	}
	return nil
}

func TestCompletionFiles(r ReleaseInfo) error {
	for _, file := range []string{"istioctl.bash", "_istioctl"} {
		path := filepath.Join(r.archive, "tools", file)
		if !util.FileExists(path) {
			return fmt.Errorf("file not found %s", path)
		}
	}
	return nil
}

func TestDebian(info ReleaseInfo) error {
	if !fileExists(filepath.Join(info.release, "deb", "istio-sidecar.deb")) {
		return fmt.Errorf("debian package not found")
	}
	return nil
}

func TestRpm(info ReleaseInfo) error {
	if !fileExists(filepath.Join(info.release, "rpm", "istio-sidecar.rpm")) {
		return fmt.Errorf("rpm package not found")
	}
	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
