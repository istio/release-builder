package main

import (
	"os"

	"github.com/howardjohn/istio-release/pkg/cmd"
)

func main() {
	rootCmd := cmd.GetRootCmd(os.Args[1:])
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
