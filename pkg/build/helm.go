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
	"regexp"
	"strings"

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
