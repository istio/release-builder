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
	"fmt"
	"path"

	"github.com/spf13/cobra"

	"istio.io/release-builder/pkg"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"

	"istio.io/pkg/log"
)

var (
	flags = struct {
		release    string
		dockerhub  string
		dockertags []string
		gcsbucket  string
		github     string
	}{}
	publishCmd = &cobra.Command{
		Use:          "publish",
		Short:        "Publish a release of Istio",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			if err := validateFlags(); err != nil {
				return fmt.Errorf("invalid flags: %v", err)
			}

			log.Infof("Publishing Istio release from: %v", flags.release)

			manifest, err := pkg.ReadManifest(path.Join(flags.release, "manifest.yaml"))
			if err != nil {
				return fmt.Errorf("failed to read manifest from release: %v", err)
			}
			manifest.Directory = path.Join(flags.release, "..")
			util.YamlLog("Manifest", manifest)

			return Publish(manifest)
		},
	}
)

func init() {
	publishCmd.PersistentFlags().StringVar(&flags.release, "release", flags.release,
		"The directory with the Istio release binary.")
	publishCmd.PersistentFlags().StringVar(&flags.dockerhub, "dockerhub", flags.dockerhub,
		"The docker hub to push images to. Example: docker.io/istio.")
	publishCmd.PersistentFlags().StringSliceVar(&flags.dockertags, "dockertags", flags.dockertags,
		"The tags to apply to docker images. Example: latest")
	publishCmd.PersistentFlags().StringVar(&flags.gcsbucket, "gcsbucket", flags.gcsbucket,
		"The gcs bucket to publish binaries to. Example, gs://istio-release.")
	publishCmd.PersistentFlags().StringVar(&flags.github, "github", flags.github,
		"The Github org to trigger a release, and tag, for. Example: istio.")
}

func GetPublishCommand() *cobra.Command {
	return publishCmd
}

func validateFlags() error {
	if flags.release == "" {
		return fmt.Errorf("--release required")
	}
	return nil
}

func Publish(manifest model.Manifest) error {
	if flags.dockerhub != "" {
		if err := Docker(manifest, flags.dockerhub, flags.dockertags); err != nil {
			return fmt.Errorf("failed to publish to docker: %v", err)
		}
	}
	if flags.gcsbucket != "" {
		if err := GcsArchive(manifest, flags.gcsbucket); err != nil {
			return fmt.Errorf("failed to publish to gcs: %v", err)
		}
	}
	if flags.github != "" {
		if err := Github(manifest, flags.github); err != nil {
			return fmt.Errorf("failed to publish to github: %v", err)
		}
	}
	return nil
}
