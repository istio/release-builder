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

package cmd

import (
	"github.com/spf13/cobra"

	"istio.io/release-builder/pkg/branch"
	"istio.io/release-builder/pkg/validate"

	"istio.io/release-builder/pkg/build"
	"istio.io/release-builder/pkg/publish"
)

// GetRootCmd returns the root of the cobra command-tree.
func GetRootCmd(args []string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "istio-release",
		Short:        "Istio build, release, and publishing tool.",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(build.GetBuildCommand())
	rootCmd.AddCommand(validate.GetValidateCommand())
	rootCmd.AddCommand(publish.GetPublishCommand())
	rootCmd.AddCommand(branch.GetBranchCommand())

	return rootCmd
}
