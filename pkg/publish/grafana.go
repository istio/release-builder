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
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
)

// Grafana publishes the grafana dashboards to grafana.com
func Grafana(manifest model.Manifest, token string) error {
	for db, id := range manifest.GrafanaDashboards {
		url := fmt.Sprintf("https://grafana.com/api/dashboards/%d/revisions", id)
		dashboard := filepath.Join(manifest.Directory, "grafana", db+".json")
		req, err := fileUploadRequest(url, "json", dashboard)
		if err != nil {
			return fmt.Errorf("failed to create request for %v: %v", db, err)
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("request to update %v failed: %v", db, err)
		}
		body, _ := io.ReadAll(resp.Body)
		log.Infof("Dashboard %v uploaded with code: %v. Body: %v", db, resp.StatusCode, string(body))
	}

	return nil
}

// Creates a new file upload http request
func fileUploadRequest(uri string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	file.Close()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, fi.Name())
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(fileContents); err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", writer.FormDataContentType())
	return req, nil
}
