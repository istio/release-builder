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

package branch

import (
	"fmt"

	"github.com/spf13/cobra"

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg"
	"istio.io/release-builder/pkg/util"
)

var (
	flags = struct {
		manifest        string
		dryrun          bool
		step            int
		githubTokenFile string
		prRepoOrg       string
	}{
		manifest: "example/manifest_branch.yaml",
		dryrun:   true, // Default to dry-run for now
	}
	branchCmd = &cobra.Command{
		Use:          "branch",
		Short:        "creates release branches for Istio",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			if err := validateFlags(); err != nil {
				return fmt.Errorf("invalid flags: %v", err)
			}

			inManifest, err := pkg.ReadInManifest(flags.manifest)
			if err != nil {
				return fmt.Errorf("failed to unmarshal manifest: %v", err)
			}

			manifest, err := pkg.InputManifestToManifest(inManifest)
			if err != nil {
				return fmt.Errorf("failed to setup manifest: %v", err)
			}

			if err := pkg.SetupWorkDir(manifest.Directory); err != nil {
				return fmt.Errorf("failed to setup work dir: %v", err)
			}

			if err := pkg.Sources(manifest); err != nil {
				return fmt.Errorf("failed to fetch sources: %v", err)
			}
			log.Infof("Fetched all sources and setup working directory at %v", manifest.WorkDir())

			token, err := util.GetGithubToken(flags.githubTokenFile)
			if err != nil {
				return err
			}

			// Validate GitHub token is set
			if err := util.ValidateGithubToken(token); err != nil {
				return err
			}

			if err := Branch(manifest, flags.step, flags.dryrun, token, flags.prRepoOrg); err != nil {
				return fmt.Errorf("failed to branch: %v", err)
			}

			log.Infof("Branch step %v to release-%s done in %v", flags.step, manifest.Version, manifest.WorkDir())
			return nil
		},
	}
)

func init() {
	branchCmd.PersistentFlags().StringVar(&flags.manifest, "manifest", flags.manifest,
		"The manifest used to get the repos for the branch cut.")
	branchCmd.PersistentFlags().BoolVar(&flags.dryrun, "dryrun", flags.dryrun,
		"Do not run any github commands.")
	branchCmd.PersistentFlags().IntVar(&flags.step, "step", flags.step,
		"Which step to run.")
	branchCmd.PersistentFlags().StringVar(&flags.githubTokenFile, "githubtoken", flags.githubTokenFile,
		"The file containing a github token.")
	branchCmd.PersistentFlags().StringVar(&flags.prRepoOrg, "prRepoOrg", flags.prRepoOrg,
		"The GitHub organization for the PR.")
}

func GetBranchCommand() *cobra.Command {
	return branchCmd
}

func validateFlags() error {
	if flags.step == 0 {
		return fmt.Errorf("--step required")
	}
	return nil
}
