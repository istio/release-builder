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

package pkg

import (
	"testing"

	"istio.io/release-builder/pkg/model"
)

func TestInputManifestToManifestBuildOutputs(t *testing.T) {
	cases := []struct {
		name    string
		outputs []string
		want    []model.BuildOutput
	}{
		{"docker", []string{"docker"}, []model.BuildOutput{model.Docker}},
		{"helm", []string{"helm"}, []model.BuildOutput{model.Helm}},
		{"debian", []string{"debian"}, []model.BuildOutput{model.Debian}},
		{"rpm", []string{"rpm"}, []model.BuildOutput{model.Rpm}},
		{"archive", []string{"archive"}, []model.BuildOutput{model.Archive}},
		{"grafana", []string{"grafana"}, []model.BuildOutput{model.Grafana}},
		{"scanner", []string{"scanner"}, []model.BuildOutput{model.Scanner}},
		{"mixed case", []string{"Debian", "RPM"}, []model.BuildOutput{model.Debian, model.Rpm}},
		{
			"default",
			nil,
			[]model.BuildOutput{model.Docker, model.Helm, model.Debian, model.Rpm, model.Archive, model.Grafana, model.Scanner},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := InputManifestToManifest(model.InputManifest{Directory: t.TempDir(), BuildOutputs: tc.outputs})
			if err != nil {
				t.Fatalf("InputManifestToManifest(%v) failed: %v", tc.outputs, err)
			}
			if len(m.BuildOutputs) != len(tc.want) {
				t.Fatalf("got %d build outputs, want %d: %v", len(m.BuildOutputs), len(tc.want), m.BuildOutputs)
			}
			for _, o := range tc.want {
				if _, f := m.BuildOutputs[o]; !f {
					t.Errorf("build output %v missing: %v", o, m.BuildOutputs)
				}
			}
		})
	}
}

func TestInputManifestToManifestUnknownBuildOutput(t *testing.T) {
	if _, err := InputManifestToManifest(model.InputManifest{Directory: t.TempDir(), BuildOutputs: []string{"foo"}}); err == nil {
		t.Fatal("expected error for unknown build output")
	}
}
