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
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
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
func Scanner(manifest model.Manifest, githubToken string) error {
	// Retrieve BASE_VERSION from the istio/istio Makefile
	istioDir := manifest.RepoDir("istio")
	var out bytes.Buffer
	grepCmd := exec.Command("grep", "BASE_VERSION", istioDir+"/Makefile.core.mk")
	grepCmd.Stdout = &out
	err := grepCmd.Run()
	if err != nil {
		return fmt.Errorf("grep error: %v", err)
	}
	baseVersion := strings.TrimSpace(strings.Split(out.String(), " ")[2]) // Assumes line of the form BASE_VERSION ?= baseVersion

	// Call imagescanner passing in base image name. If request times out, retry the request
	baseImageName := "istio/base:" + baseVersion
	numberRetries := 4
	var resp *http.Response
	for numberRetries > 0 {
		resp, err = http.Get("http://imagescanner.cloud.ibm.com/scan?image=" + strings.TrimSpace(baseImageName))
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				fmt.Println("Scanner request timed out. Need to run request again.")
			} else {
				return err
			}
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
	if err := json.Unmarshal(body, &response); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		if (resp.StatusCode) == http.StatusInternalServerError {
			return fmt.Errorf("scanning error (%s): %s", baseImageName, response.Progress)
		}
		return fmt.Errorf("scanning error (%s): %d", baseImageName, resp.StatusCode)
	}
	if response.Results.Status == "OK" {
		log.Infof("Base image scan of %s was successful", baseImageName)
		return nil
	}

	// There were vulnerabilities. Output message listing vulnerabilities.
	log.Infof("Base image scan of %s failed with:\n %s", baseImageName, string(body))

	// If IgnoreVulernability is true, just just return
	if manifest.IgnoreVulnerability {
		return nil
	}

	// Else build a new set of images.
	// baseVersion is like 1.10-dev.1
	// Increment the digit after the last period to get the new tag.
	index := strings.LastIndex(baseVersion, ".") + 1
	var digit int
	if digit, err = strconv.Atoi(baseVersion[index:]); err != nil {
		return err
	}
	newBaseVersion := baseVersion[:index] + strconv.Itoa(digit+1)
	log.Infof("new baseVersion: %s", newBaseVersion)

	// Run the script to create the base images
	buildImageEnv := []string{
		"HUBS=docker.io/istio gcr.io/istio-release",
		"TAG=" + newBaseVersion,
	}
	cmd := util.VerboseCommand("tools/build-base-images.sh")
	cmd.Env = util.StandardEnv(manifest)
	cmd.Env = append(cmd.Env, buildImageEnv...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = manifest.RepoDir("istio")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build base images: %v", err)
	}

	// Now create a PR to update the TAG to use the new images
	sedString := "s/BASE_VERSION ?=.*/BASE_VERSION ?= " + newBaseVersion + "/"
	sedCmd := util.VerboseCommand("sed", "-i", sedString, "Makefile.core.mk")
	sedCmd.Dir = manifest.RepoDir("istio")
	if err := sedCmd.Run(); err != nil {
		return fmt.Errorf("failed to run sed command: %v", err)
	}

	if err := util.CreatePR(manifest, "istio", "newBaseVersion"+newBaseVersion,
		"Update BASE_VERSION to "+newBaseVersion, false, githubToken); err != nil {
		return fmt.Errorf("failed PR creation: %v", err)
	}

	return fmt.Errorf("new base images created, new PR needs to be merged before another build is run")
}
