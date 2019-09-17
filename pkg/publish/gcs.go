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

package publish

import (
	"fmt"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"
)

// GcsArchive publishes the final release archive to the given GCS bucket
func GcsArchive(manifest model.Manifest, bucket string) error {
	// TODO use golang libraries
	// A bit painful since we cannot just copy the directory it seems, but must do each file
	if err := util.VerboseCommand("gsutil", "-m", "cp", "-r", manifest.OutDir()+"/*", bucket+"/"+manifest.Version+"/").Run(); err != nil {
		return fmt.Errorf("failed to write to gcs: %v", err)
	}
	return nil
}
