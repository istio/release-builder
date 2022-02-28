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
	"fmt"

	"github.com/spf13/cobra"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

var (
	flags = struct {
		manifest        string
		githubTokenFile string
	}{
		manifest: "example/manifest.yaml",
	}
	buildCmd = &cobra.Command{
		Use:          "build",
		Short:        "Builds a release of Istio",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			inManifest, err := pkg.ReadInManifest(flags.manifest)
			if err != nil {
				return fmt.Errorf("failed to unmarshal manifest: %v", err)
			}

			manifest, err := pkg.InputManifestToManifest(inManifest)
			if err != nil {
				return fmt.Errorf("failed to setup manifest: %v", err)
			}

			// Save these values as they are needed for git commits and PRs
			savedIstioGit := inManifest.Dependencies.Get()["istio"].Git
			savedIstioBranch := inManifest.Dependencies.Get()["istio"].Branch
			log.Infof("Saved Istio git:\n%+v", savedIstioGit)
			log.Infof("Saved Istio branch:\n%+v", savedIstioBranch)

			if err := pkg.SetupWorkDir(manifest.Directory); err != nil {
				return fmt.Errorf("failed to setup work dir: %v", err)
			}

			if err := pkg.Sources(manifest); err != nil {
				return fmt.Errorf("failed to fetch sources: %v", err)
			}
			log.Infof("Fetched all sources and setup working directory at %v", manifest.WorkDir())

			if err := pkg.StandardizeManifest(&manifest); err != nil {
				return fmt.Errorf("failed to standardize manifest: %v", err)
			}

			if _, f := manifest.BuildOutputs[model.Scanner]; f {
				token, err := util.GetGithubToken(flags.githubTokenFile)
				if err != nil {
					return err
				}
				if err := Scanner(manifest, token, savedIstioGit, savedIstioBranch); err != nil {
					if manifest.IgnoreVulnerability {
						log.Infof("Ignoring vulnerability scanning error: %v", err)
					} else {
						return fmt.Errorf("failed image scan: %v", err)
					}
				}
			}

			if err := Build(manifest); err != nil {
				return fmt.Errorf("failed to build: %v", err)
			}

			log.Infof("Built release at %v", manifest.OutDir())
			return nil
		},
	}
)

func init() {
	buildCmd.PersistentFlags().StringVar(&flags.manifest, "manifest", flags.manifest,
		"The manifest to build.")
	buildCmd.PersistentFlags().StringVar(&flags.githubTokenFile, "githubtoken", flags.githubTokenFile,
		"The file containing a github token.")
}

func GetBuildCommand() *cobra.Command {
	return buildCmd
}
