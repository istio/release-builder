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

package util

import (
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
)

func StandardEnv(manifest model.Manifest) []string {
	env := os.Environ()
	env = append(env,
		"GOPATH="+manifest.WorkDir(),
		"TAG="+manifest.Version,
		"VERSION="+manifest.Version,
		"HUB="+manifest.Docker,
		"BUILD_WITH_CONTAINER=0", // Build should already run in container, having multiple layers of docker causes issues
		"IGNORE_DIRTY_TREE=1",
		"INCLUDE_UNTAGGED_DEFAULT=true",
	)
	return env
}

func removeEnvKey(s []string, key string) []string {
	for i, v := range s {
		if strings.HasPrefix(v, key+"=") {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// RunMake runs a make command for the repo, with standard environment variables set
func RunMake(manifest model.Manifest, repo string, env []string, c ...string) error {
	cmd := VerboseCommand("make", c...)
	cmd.Env = StandardEnv(manifest)
	// Unset the environment variables that are set in a container which cause `make` artifacts
	// to build in the container directories. release-builder expects all `make` artifacts to be
	// created in the manifest specified directory.
	cmd.Env = removeEnvKey(cmd.Env, "TARGET_OUT")
	cmd.Env = removeEnvKey(cmd.Env, "TARGET_OUT_LINUX")
	cmd.Env = removeEnvKey(cmd.Env, "CONTAINER_TARGET_OUT")
	cmd.Env = removeEnvKey(cmd.Env, "CONTAINER_TARGET_OUT_LINUX")
	cmd.Env = removeEnvKey(cmd.Env, "TARGET_OS")
	cmd.Env = removeEnvKey(cmd.Env, "TARGET_ARCH")
	cmd.Env = append(cmd.Env, env...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = manifest.RepoDir(repo)
	log.Infof("Running make %v with env=%v wd=%v", strings.Join(c, " "), strings.Join(env, " "), cmd.Dir)
	return cmd.Run()
}

// YamlLog logs a object as yaml
func YamlLog(prefix string, i interface{}) {
	manifestYaml, _ := yaml.Marshal(i)
	log.Infof("%s: %v", prefix, string(manifestYaml))
}
