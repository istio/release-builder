package cmd

import (
	"github.com/howardjohn/istio-release/pkg/build"
	"github.com/howardjohn/istio-release/pkg/publish"
	"github.com/spf13/cobra"
)

// GetRootCmd returns the root of the cobra command-tree.
func GetRootCmd(args []string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "istio-release",
		Short:        "Istio build, release, and publishing tool.",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(build.GetBuildCommand())
	rootCmd.AddCommand(publish.GetPublishCommand())

	return rootCmd
}
