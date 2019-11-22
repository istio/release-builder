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

package build

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"

	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"

	"istio.io/pkg/log"
)

var (
	// Currently tags are set as `release-1.x-latest-daily` or `latest` or `1.x-dev`
	tagRegexes = []*regexp.Regexp{
		regexp.MustCompile(`tag: .*-latest-daily`),
		regexp.MustCompile(`tag: latest`),
		regexp.MustCompile(`tag: 1\..-dev`),
	}

	operatorDeployRegex = regexp.MustCompile(`image: gcr.io/istio-testing/operator:.*`)

	// Currently tags are set as `gcr.io/istio-testing` or `gcr.io/istio-release`
	hubs = []string{"gcr.io/istio-testing", "gcr.io/istio-release"}

	allCharts = map[string][]string{
		"istio": {"install/kubernetes/helm/istio", "install/kubernetes/helm/istio-init"},
		"cni":   {"deployments/kubernetes/install/helm/istio-cni"},
	}
)

// SanitizeAllCharts rewrites versions, tags, and hubs for helm charts. This is done independent of Helm
// as it is required for both the helm charts and the archive
func SanitizeAllCharts(manifest model.Manifest) error {
	for repo, charts := range allCharts {
		for _, chart := range charts {
			if err := sanitizeChart(manifest, path.Join(manifest.RepoDir(repo), chart)); err != nil {
				return fmt.Errorf("failed to sanitze chart %v: %v", chart, err)
			}
		}
	}
	return nil
}

// Helm outputs the helm charts for the installation
func Helm(manifest model.Manifest) error {
	// Setup a working directory for helm
	helm := path.Join(manifest.WorkDir(), "helm")
	if err := os.MkdirAll(path.Join(helm, "packages"), 0750); err != nil {
		return fmt.Errorf("failed to setup helm directory: %v", err)
	}
	if err := util.VerboseCommand("helm", "--home", helm, "init", "--client-only").Run(); err != nil {
		return fmt.Errorf("failed to setup helm: %v", err)
	}

	for repo, charts := range allCharts {
		for _, chart := range charts {
			// Package will create the tar.gz bundle for the given chart
			cmd := util.VerboseCommand("helm", "--home", helm, "package", chart, "--destination", path.Join(helm, "packages"))
			cmd.Dir = manifest.RepoDir(repo)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to package %v by `%v`: %v", chart, cmd.Args, err)
			}
		}
	}
	if err := util.VerboseCommand("helm", "--home", helm, "repo", "index", path.Join(helm, "packages")).Run(); err != nil {
		return fmt.Errorf("failed to create helm index.yaml")
	}

	// Copy to final charts/ output directory
	if err := util.CopyDir(path.Join(manifest.WorkDir(), "helm", "packages"), path.Join(manifest.OutDir(), "charts")); err != nil {
		return fmt.Errorf("failed to package helm chart: %v", err)
	}
	return nil
}

// In the final published charts, we need the version and tag to be set for the appropriate version
// In order to do this, we simply replace the current version with the new one.
// Currently the current version is not explicitly a placeholder - see https://github.com/istio/istio/issues/17146.
func sanitizeChart(manifest model.Manifest, s string) error {
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
		if fname == "Chart.yaml" || fname == "values.yaml" || fname == "values_gke.yaml" || fname == "requirements.yaml" {
			read, err := ioutil.ReadFile(p)
			if err != nil {
				return err
			}
			contents := string(read)
			// These fields contain the version, we swap out the placeholder with the correct version
			for _, replacement := range []string{"appVersion", "version"} {
				before := fmt.Sprintf("%s: %s", replacement, cv)
				after := fmt.Sprintf("%s: %s", replacement, manifest.Version)
				contents = strings.ReplaceAll(contents, before, after)
			}

			err = ioutil.WriteFile(p, []byte(contents), 0)
			if err != nil {
				return err
			}

			if err := sanitizeTemplate(manifest, p); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Similar to sanitizeChart, but works on generic templates rather than only Helm charts.
// This updates the hub and tag fields for a single file
func sanitizeTemplate(manifest model.Manifest, p string) error {
	read, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}
	contents := string(read)

	// The hub and tag should be update
	for _, hub := range hubs {
		contents = strings.ReplaceAll(contents, fmt.Sprintf("hub: %s", hub), fmt.Sprintf("hub: %s", manifest.Docker))
	}
	for _, tagRegex := range tagRegexes {
		contents = tagRegex.ReplaceAllString(contents, fmt.Sprintf("tag: %s", manifest.Version))
	}

	// Some manifests have images directly embedded
	// Rather than try to make a very generic regex, specifically enumerate these to avoid false positives
	contents = operatorDeployRegex.ReplaceAllString(contents, fmt.Sprintf("image: %s/operator:%s", manifest.Docker, manifest.Version))

	err = ioutil.WriteFile(p, []byte(contents), 0)
	if err != nil {
		return err
	}

	return nil
}
