package main

import (
"fmt"
"io/ioutil"
"os"
"path"

"github.com/howardjohn/istio-release/pkg"
"github.com/howardjohn/istio-release/pkg/build"
"github.com/spf13/cobra"

"istio.io/pkg/log"
)

func setupWorkDir() string {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "istio-release")
	if err != nil {
		log.Fatalf("failed to create working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(tmpdir, "sources"), 0750); err != nil {
		log.Fatalf("failed to set up working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(tmpdir, "work"), 0750); err != nil {
		log.Fatalf("failed to set up working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(tmpdir, "out"), 0750); err != nil {
		log.Fatalf("failed to set up working directory: %v", err)
	}
	return tmpdir
}

var (
	rootCmd = &cobra.Command{
		Use:   "istio-build",
		Short: "Builds a release of Istio",
		SilenceUsage: true,
		Args:  cobra.ExactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			manifest, err := pkg.ReadManifest("release/manifest.yaml")
			if err != nil {
				return fmt.Errorf("failed to unmarshal manifest: %v", err)
			}

			if manifest.Directory == "" {
				manifest.Directory = setupWorkDir()

			}
			if err := pkg.Sources(manifest); err != nil {
				return fmt.Errorf("failed to fetch sources: %v", err)
			}
			log.Infof("Fetched all sources, setup working directory at %v", manifest.WorkDir())

			if err := pkg.StandardizeManifest(&manifest); err != nil {
				return fmt.Errorf("failed to standardize manifest: %v", err)
			}

			if err := build.Build(manifest); err != nil {
				return fmt.Errorf("failed to build: %v", err)
			}

			log.Infof("Built release at %v", manifest.OutDir())
			return nil
		},
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(-1)
	}
}
