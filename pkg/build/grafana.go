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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// Grafana packages Istio dashboards in a form that is ready to be published to grafana.com
func Grafana(manifest model.Manifest) error {
	if err := util.CopyDir(
		path.Join(manifest.RepoDir("istio"), "manifests/addons/dashboards"),
		path.Join(manifest.WorkDir(), "grafana"),
	); err != nil {
		return err
	}
	dashboards, err := os.ReadDir(path.Join(manifest.WorkDir(), "grafana"))
	if err != nil {
		return fmt.Errorf("failed to read dashboards: %v", err)
	}
	for _, dashboard := range dashboards {
		if !strings.HasSuffix(dashboard.Name(), "-dashboard.json") && !strings.HasSuffix(dashboard.Name(), "-dashboard.gen.json") {
			log.Infof("skipping non-dashboard file dashboard %v", dashboard.Name())
			continue
		}
		if err := externalizeDashboard(manifest.Version, path.Join(path.Join(manifest.WorkDir(), "grafana", dashboard.Name()))); err != nil {
			return fmt.Errorf("failed to process dashboard %v: %v", dashboard.Name(), err)
		}
	}
	if err := util.CopyDir(
		path.Join(manifest.WorkDir(), "grafana"),
		path.Join(manifest.OutDir(), "grafana"),
	); err != nil {
		return err
	}
	return nil
}

// externalizeDashboard converts a grafana dashboard from the "internal" representation, which is used
// in the charts, to the "external" representation. This is the form needed to publish to grafana.com
// This has two fields added, __inputs and __requires, and the datasource is not hardcoded.
func externalizeDashboard(version, file string) error {
	original, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read %v: %v", file, err)
	}
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(original, &msg); err != nil {
		return fmt.Errorf("failed to unmarshall %v: %v", file, err)
	}

	var inputMsg json.RawMessage
	if err := json.Unmarshal([]byte(`[
    {
      "name": "DS_PROMETHEUS",
      "label": "Prometheus",
      "description": "",
      "type": "datasource",
      "pluginId": "prometheus",
      "pluginName": "Prometheus"
    }
  ]`), &inputMsg); err != nil {
		return fmt.Errorf("failed to construct __inputs: %v", err)
	}
	msg["__inputs"] = inputMsg

	var requiresMesh json.RawMessage
	if err := json.Unmarshal([]byte(`[
    {
      "type": "grafana",
      "id": "grafana",
      "name": "Grafana",
      "version": "6.4.3"
    },
    {
      "type": "panel",
      "id": "graph",
      "name": "Graph",
      "version": ""
    },
    {
      "type": "datasource",
      "id": "prometheus",
      "name": "Prometheus",
      "version": "5.0.0"
    },
    {
      "type": "panel",
      "id": "table",
      "name": "Table",
      "version": ""
    }
  ]`), &requiresMesh); err != nil {
		return fmt.Errorf("failed to construct __requires: %v", err)
	}
	msg["__requires"] = requiresMesh

	// Validate there is not already a description, and that there is a tile
	if by, _ := msg["description"].MarshalJSON(); string(by) != `""` && string(by) != `null` {
		return fmt.Errorf("already has a description: %v", string(by))
	}
	titleBytes := msg["title"][1 : len(msg["title"])-1]
	if len(titleBytes) == 0 {
		return fmt.Errorf("no title: %v", string(titleBytes))
	}

	// Set the description, so the version is included
	msg["description"] = []byte(fmt.Sprintf(`"%s version %s"`, string(titleBytes), version))

	// Write out the result
	result, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal%v", err)
	}
	// Substitute the datasource with the variable placeholder
	result = bytes.ReplaceAll(result, []byte(`"datasource": "Prometheus"`), []byte(`"datasource": "${DS_PROMETHEUS}"`))
	if err := os.WriteFile(file, result, 0o644); err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}
	return nil
}
