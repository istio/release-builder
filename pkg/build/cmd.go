package build

import (
	"fmt"

	"github.com/howardjohn/istio-release/pkg"
	"github.com/spf13/cobra"

	"istio.io/pkg/log"
)

var (
	flags = struct {
		manifest string
	}{
		manifest: "example/manifest.yaml",
	}
	buildCmd = &cobra.Command{
		Use:          "build",
		Short:        "Builds a release of Istio",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			manifest, err := pkg.ReadManifest(flags.manifest)
			if err != nil {
				return fmt.Errorf("failed to unmarshal manifest: %v", err)
			}

			if err := pkg.Sources(manifest); err != nil {
				return fmt.Errorf("failed to fetch sources: %v", err)
			}
			log.Infof("Fetched all sources, setup working directory at %v", manifest.WorkDir())

			if err := pkg.StandardizeManifest(&manifest); err != nil {
				return fmt.Errorf("failed to standardize manifest: %v", err)
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
}

func GetBuildCommand() *cobra.Command {
	return buildCmd
}
