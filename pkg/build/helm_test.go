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
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"sigs.k8s.io/yaml"

	"istio.io/release-builder/pkg/model"
)

func TestHelmUpdate(t *testing.T) {
	cases := []struct {
		name            string
		inputChartfile  string
		inputValuesfile string
		inputManifest   model.Manifest
	}{
		{
			"test-version",
			filepath.Join("testdata", "chart-deps-in.yaml"),
			filepath.Join("testdata", "chart-values-in.yaml"),
			model.Manifest{
				Version: "1.19.13-eks-8df270", // sufficiently oddball version string
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			chFile := createWritableTempVersion(t, dir, "Chart.yaml", tc.inputChartfile)
			_ = createWritableTempVersion(t, dir, "values.yaml", tc.inputValuesfile)

			err := stampChartForRelease(tc.inputManifest, dir)
			if err != nil {
				t.Fatal(err)
			}

			updated, err := os.ReadFile(chFile.Name())
			if err != nil {
				t.Fatal(err)
			}

			chartFile := chart.Metadata{}
			if err := yaml.Unmarshal(updated, &chartFile); err != nil {
				t.Fatal(err)
			}

			if chartFile.AppVersion != tc.inputManifest.Version {
				t.Fatalf("appVersion doesn't match: %s", chartFile.AppVersion)
			}

			if chartFile.Version != tc.inputManifest.Version {
				t.Fatalf("version doesn't match: %s", chartFile.Version)
			}

			for _, dep := range chartFile.Dependencies {
				if dep.Version != tc.inputManifest.Version {
					t.Fatalf("dep version doesn't match: %+v", dep)
				}
			}
		})
	}
}

func createWritableTempVersion(t *testing.T, tmpDir, destFileName, sourceFilePath string) *os.File {
	file, err := os.Create(path.Join(tmpDir, destFileName))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(sourceFilePath)
	if err != nil {
		t.Fatal(err)
	}
	sourceReader := bytes.NewReader(data)
	_, err = io.Copy(file, sourceReader)
	if err != nil {
		t.Fatal(err)
	}

	return file
}
