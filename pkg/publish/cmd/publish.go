package main

import (
	"fmt"
	"os"
	"path"

	"github.com/ghodss/yaml"
	"github.com/howardjohn/istio-release/pkg"
	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/publish"
	"github.com/spf13/cobra"

	"istio.io/pkg/log"
)

var (
	flags = struct {
		release   string
		dockerhub string
	}{}
	rootCmd = &cobra.Command{
		Use:          "istio-publish",
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
			manifestYaml, _ := yaml.Marshal(manifest)
			log.Infof("Manifest: %v", string(manifestYaml))

			return Publish(manifest)
		},
	}
)

func validateFlags() error {
	if flags.dockerhub == "" {
		return fmt.Errorf("--dockerhub required")
	}
	if flags.release == "" {
		return fmt.Errorf("--release required")
	}
	return nil
}

func Publish(manifest model.Manifest) error {
	if err := publish.Docker(manifest, flags.dockerhub); err != nil {
		return fmt.Errorf("failed to publish to docker: %v", err)
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flags.release, "release", flags.release,
		"The directory with the Istio release binary.")
	rootCmd.PersistentFlags().StringVar(&flags.dockerhub, "dockerhub", flags.dockerhub,
		"The docker hub to push images to.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}
