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
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"strings"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
)

// Structs for json data required
type Response struct {
	Progress string
	Results  Results
}

type Results struct {
	Status string
}

// Scanner checks the base image for any CVEs.
func Scanner(manifest model.Manifest) error {
	// Retrieve BASE_VERSION from the istio/istio Makefile
	istioDir := manifest.RepoDir("istio")
	var out bytes.Buffer
	grepCmd := exec.Command("grep", "BASE_VERSION", istioDir+"/Makefile.core.mk")
	grepCmd.Stdout = &out
	err := grepCmd.Run()
	if err != nil {
		return fmt.Errorf("grep error: %v", err)
	}
	baseVersion := strings.Split(out.String(), " ")[2] // Assumes line of the form BASE_VERSION ?+ baseVersion

	// Call imagescanner passing in base image name. If request times out, retry the request
	baseImageName := "istio/base:" + baseVersion
	// baseImageName = "ericvn/base:ericvn"
	numberRetries := 4
	var resp *http.Response
	for numberRetries > 0 {
		resp, err = http.Get("http://imagescanner.cloud.ibm.com/scan?image=" + strings.TrimSpace(baseImageName))
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				fmt.Println("Scanner request timed out. Need to run request again.")
			}
			return err
		}
		numberRetries--
	}
	if err != nil {
		return err
	}

	// Check response for OK
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var response Response
	json.Unmarshal(body, &response)
	if resp.StatusCode != http.StatusOK {
		if (resp.StatusCode) == http.StatusInternalServerError {
			return fmt.Errorf("Scanning error (%s): %s", baseImageName, response.Progress)
		}
		return fmt.Errorf("Scanning error (%s): %d", baseImageName, resp.StatusCode)
	}
	if response.Results.Status == "OK" {
		log.Infof("Base image scan of %s was successful", baseImageName)
		return nil
	}

	// There were vulnerabilities. Return body.
	return errors.New("Image name: " + baseImageName + "\n" + string(body))
}
