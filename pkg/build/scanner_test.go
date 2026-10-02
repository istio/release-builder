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

import "testing"

func TestStableBranchName(t *testing.T) {
	cases := []struct {
		version string
		want    string
	}{
		{"master", "update-base-version-master"},
		{"1.21", "update-base-version-1-21"},
		{"1.21.1", "update-base-version-1-21-1"},
		{"1.31", "update-base-version-1-31"},
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			got := stableBranchName(tc.version)
			if got != tc.want {
				t.Errorf("stableBranchName(%q) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}
