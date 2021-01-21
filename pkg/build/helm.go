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

	"istio.io/pkg/log"

	"istio.io/release-builder/pkg/model"
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

	helmCharts = []string{
		"manifests/charts/base",
		"manifests/charts/gateways/istio-egress",
		"manifests/charts/gateways/istio-ingress",
		"manifests/charts/istio-cni",
		"manifests/charts/istio-control/istio-discovery/",
		"manifests/charts/istio-operator",
		"manifests/charts/istiod-remote",
	}
)

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

// SanitizeAllCharts rewrites versions, tags, and hubs for helm charts. This is done independent of Helm
// as it is required for both the helm charts and the archive
func SanitizeAllCharts(manifest model.Manifest) error {
	for _, chart := range helmCharts {
		if err := sanitizeChart(manifest, path.Join(manifest.RepoDir("istio"), chart)); err != nil {
			return fmt.Errorf("failed to sanitze chart %v: %v", chart, err)
		}
	}
	return nil
}

// In the final published charts, we need the version and tag to be set for the appropriate version
// In order to do this, we simply replace the current version with the new one.
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
	cv := chart["version"].(string)
	if err := filepath.Walk(s, func(p string, info os.FileInfo, err error) error {
		fname := path.Base(p)
		if fname == "Chart.yaml" || fname == "values.yaml" {
			read, err := ioutil.ReadFile(p)
			if err != nil {
				return err
			}
			contents := string(read)
			// These fields contain the version, we swap out the placeholder with the correct version
			for _, replacement := range []string{"version"} {
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
