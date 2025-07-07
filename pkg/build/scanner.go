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
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"istio.io/istio/pkg/log"
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

var alwaysGenerateBaseImage = func() bool {
	b, err := strconv.ParseBool(os.Getenv("ALWAYS_GENERATE_BASE_IMAGE"))
	if err != nil {
		return false
	}
	return b
}()

// Scanner checks the base image for any CVEs.
func Scanner(manifest model.Manifest, githubToken, git, branch string) error {
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

	// Call image scanner passing in base image name. If request times out, retry the request
	istioBaseRegistry := os.Getenv("ISTIO_BASE_REGISTRY")
	if istioBaseRegistry == "" {
		istioBaseRegistry = "istio"
	}
	baseImageName := istioBaseRegistry + "/base:" + baseVersion

	trivyScanOutput, err := util.RunWithOutput(
		"trivy",
		"image",
		"--security-checks", "vuln", // Disable secret scanning which is not relevant
		"--ignore-unfixed",
		"--no-progress",
		"--exit-code",
		"2",
		baseImageName,
	)
	if err == nil {
		log.Infof("Base image scan of %s was successful", baseImageName)
		if alwaysGenerateBaseImage {
			log.Infof("Generating base image anyways due to ALWAYS_GENERATE_BASE_IMAGE=true")
		} else {
			return nil
		}
	} else {
		//--exit-code 2 above states to return 2 if vulnerabilities are found. If we get a different error code or we can't check the error code, bail out
		if exitError, ok := err.(*exec.ExitError); ok {
			// Scanner failed with an exit code indicating a failure other than vulnerabilities found
			if exitError.ExitCode() != 2 {
				return fmt.Errorf("base image scan of %s failed with error:\n %s", baseImageName, err.Error())
			}
		} else {
			// Scanner failed, but not with an ExitError
			return fmt.Errorf("base image scan of %s failed. Unable to process exit code:\n %s", baseImageName, err.Error())
		}
	}

	// Else build a new set of images.
	// Time format chosen for consistency with build tools tag:
	// https://github.com/istio/tools/blob/ee7da00900dc878a2e865e43250c34735f130b7a/docker/build-tools/build-and-push.sh#L27
	const timeFormat = "2006-01-02T15-04-05"
	tag := fmt.Sprintf("%s-%s", manifest.Version, time.Now().Format(timeFormat))
	log.Infof("new base tag: %s", tag)

	// Setup for multiarch build.
	// See https://medium.com/@artur.klauser/building-multi-architecture-docker-images-with-buildx-27d80f7e2408 for more info
	if err := util.VerboseCommand("docker",
		"run", "--rm", "--privileged", "multiarch/qemu-user-static", "--reset", "-p", "yes").Run(); err != nil {
		return fmt.Errorf("failed to run qemu-user-static container: %v", err)
	}

	targetArchitecture := os.Getenv("ARCH")
	if targetArchitecture == "" {
		targetArchitecture = "linux/amd64,linux/arm64"
	}

	dockerHubs := os.Getenv("HUBS")
	if dockerHubs == "" {
		dockerHubs = "docker.io/istio gcr.io/istio-release"
	}

	// Run the script to create the base images
	buildImageEnv := []string{
		"DOCKER_ARCHITECTURES=" + targetArchitecture,
		"HUBS=" + dockerHubs,
	}
	if manifest.Version == "master" {
		// Push :latest tag for master
		buildImageEnv = append(buildImageEnv, fmt.Sprintf("TAGS=%s %s", tag, "latest"))
	} else {
		buildImageEnv = append(buildImageEnv, "TAG="+tag)
	}
	cmd := util.VerboseCommand("tools/build-base-images.sh")
	cmd.Env = util.StandardEnv(manifest)
	cmd.Env = append(cmd.Env, buildImageEnv...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = istioDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build base images: %v", err)
	}

	// Now create a PR to update the TAG to use the new images
	sedString := "s/BASE_VERSION ?=.*/BASE_VERSION ?= " + tag + "/"
	sedCmd := util.VerboseCommand("sed", "-i", sedString, "Makefile.core.mk")
	sedCmd.Dir = istioDir
	if err := sedCmd.Run(); err != nil {
		return fmt.Errorf("failed to run sed command: %v", err)
	}

	if err := util.CreatePR(
		manifest,
		"istio",
		"newBaseVersion"+tag,
		"Update BASE_VERSION to "+tag,
		fmt.Sprintf("```\n%s\n```", trivyScanOutput),
		false,
		githubToken,
		git,
		branch,
		[]string{"auto-merge"}, ""); err != nil {
		return fmt.Errorf("failed PR creation: %v", err)
	}

	return nil
}
