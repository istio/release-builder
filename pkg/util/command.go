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

	"github.com/ghodss/yaml"

	"istio.io/release-builder/pkg/model"

	"istio.io/pkg/log"
)

// RunMake runs a make command for the repo, with standard environment variables set
func RunMake(manifest model.Manifest, repo string, env []string, c ...string) error {
	cmd := VerboseCommand("make", c...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOPATH="+manifest.WorkDir(), "TAG="+manifest.Version, "VERSION="+manifest.Version)
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
