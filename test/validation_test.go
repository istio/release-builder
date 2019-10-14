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

package test

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg"
	"istio.io/release-builder/pkg/model"
)

var (
	release  *string
	tmpDir   string
	manifest model.Manifest
	archive  string
)

func TestMain(m *testing.M) {
	release = flag.String("release", "", "directory for the release")
	flag.Parse()
	if *release == "" {
		log.Info("skipping validation, no release specified")
		os.Exit(0)
	}
	var err error
	tmpDir, err = ioutil.TempDir("/tmp", "release-test")
	if err != nil {
		panic(err)
	}
	log.Infof("test temporary dir at %s", tmpDir)

	manifest, err = pkg.ReadManifest(filepath.Join(*release, "manifest.yaml"))
	if err != nil {
		panic(err)
	}

	if err := exec.Command("tar", "xvf", filepath.Join(*release, fmt.Sprintf("istio-%s-linux.tar.gz", manifest.Version)), "-C", tmpDir).Run(); err != nil {
		panic(err)
	}
	archive = filepath.Join(tmpDir, "istio-"+manifest.Version)
	os.Exit(m.Run())
}

func TestIstioctl(t *testing.T) {
	// Check istioctl from archive
	t.Run("archive", func(t *testing.T) {
		buf := &bytes.Buffer{}
		cmd := exec.Command(filepath.Join(archive, "bin", "istioctl"), "version", "--remote=false", "--short")
		cmd.Stdout = buf
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		got := strings.TrimSpace(buf.String())
		if got != manifest.Version {
			t.Fatalf("istioctl version output incorrect, got %v expected %v", got, manifest.Version)
		}
	})

	// Check istioctl from stand-alone archive
	t.Run("standalone", func(t *testing.T) {
		istioctlArchivePath := filepath.Join(*release, fmt.Sprintf("istioctl-%s-linux.tar.gz", manifest.Version))
		if err := exec.Command("tar", "xvf", istioctlArchivePath, "-C", tmpDir).Run(); err != nil {
			t.Fatal(err)
		}
		buf := &bytes.Buffer{}
		cmd := exec.Command(filepath.Join(tmpDir, "istioctl"), "version", "--remote=false", "--short")
		cmd.Stdout = buf
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		got := strings.TrimSpace(buf.String())
		if got != manifest.Version {
			t.Fatalf("istioctl version output incorrect, got %v expected %v", got, manifest.Version)
		}
	})
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

func getValues(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	values, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var typedValues map[string]interface{}
	if err := yaml.Unmarshal(values, &typedValues); err != nil {
		t.Fatal(err)
	}
	return typedValues
}

func TestHelmVersions(t *testing.T) {
	checks := []string{
		"install/kubernetes/helm/istio/values.yaml",
		"install/kubernetes/helm/istio-init/values.yaml",
	}
	for _, f := range checks {
		t.Run(f, func(t *testing.T) {
			values := getValues(t, filepath.Join(archive, f))
			tag := GenericMap{values}.Path([]string{"global", "tag"})
			if tag != manifest.Version {
				t.Fatalf("archive tag incorrect, got %v expected %v", tag, manifest.Version)
			}
			hub := GenericMap{values}.Path([]string{"global", "hub"})
			if hub != manifest.Docker {
				t.Fatalf("hub incorrect, got %v expected %v", hub, manifest.Docker)
			}
		})
	}
	cniChecks := []string{
		"install/kubernetes/helm/istio-cni/values.yaml",
		"install/kubernetes/helm/istio-cni/values_gke.yaml",
	}
	for _, f := range cniChecks {
		t.Run(f, func(t *testing.T) {
			values := getValues(t, filepath.Join(archive, f))
			tag := GenericMap{values}.Path([]string{"tag"})
			if tag != manifest.Version {
				t.Fatalf("archive tag incorrect, got %v expected %v", tag, manifest.Version)
			}
			hub := GenericMap{values}.Path([]string{"hub"})
			if hub != manifest.Docker {
				t.Fatalf("hub incorrect, got %v expected %v", hub, manifest.Docker)
			}
		})
	}
	operatorChecks := []string{
		"install/kubernetes/operator/profiles/default.yaml",
	}
	for _, f := range operatorChecks {
		t.Run(f, func(t *testing.T) {
			values := getValues(t, filepath.Join(archive, f))
			tag := GenericMap{values}.Path([]string{"spec", "tag"})
			if tag != manifest.Version {
				t.Fatalf("archive tag incorrect, got %v expected %v", tag, manifest.Version)
			}
			hub := GenericMap{values}.Path([]string{"spec", "hub"})
			if hub != manifest.Docker {
				t.Fatalf("hub incorrect, got %v expected %v", hub, manifest.Docker)
			}
		})
	}
}

func TestManifest(t *testing.T) {
	for _, repo := range []string{"api", "cni", "gogo-genproto", "istio", "operator", "pkg", "proxy"} {
		t.Run(repo, func(t *testing.T) {
			d, f := manifest.AllDependencies[repo]
			if !f || d == "" {
				t.Fatalf("Got empty SHA")
			}
		})
	}
	if manifest.Directory != "" {
		t.Fatalf("expected manifest directory to be hidden, got %v", manifest.Directory)
	}
}

func TestLicenses(t *testing.T) {
	license, err := ioutil.ReadFile(filepath.Join(*release, "LICENSES"))
	if err != nil {
		t.Fatal(err)
	}
	// TODO validate license once it is fixed
	_ = license
}
