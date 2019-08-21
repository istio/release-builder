package main

import (
	"fmt"
	"os"
	"path"

	"github.com/ghodss/yaml"
	"github.com/howardjohn/istio-release/pkg"
	"github.com/spf13/cobra"
	"istio.io/pkg/log"
)

var (
	flags = struct {
		release string
	}{}
	rootCmd = &cobra.Command{
		Use:   "istio-publish",
		Short: "Publish a release of Istio",
		SilenceUsage: true,
		Args:  cobra.ExactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			log.Infof("Publishing Istio release from: %v", flags.release)

			manifest, err := pkg.ReadManifest(path.Join(flags.release, "manifest.yaml"))
			if err != nil {
				return fmt.Errorf("failed to read manifest from release: %v", err)
			}
			manifestYaml, _ := yaml.Marshal(manifest)
			log.Infof("Manifest: %v", string(manifestYaml))
			return nil
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&flags.release, "release-binary", flags.release,
		"The directory with the Istio release binary.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}
