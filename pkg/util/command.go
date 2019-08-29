package util

import (
	"os"
	"strings"

	"github.com/howardjohn/istio-release/pkg/model"

	"istio.io/pkg/log"
)

func RunMake(manifest model.Manifest, repo string, env []string, c ...string) error {
	cmd := VerboseCommand("make", c...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOPATH="+manifest.WorkDir())
	cmd.Env = append(cmd.Env, "TAG="+manifest.Version)
	cmd.Env = append(cmd.Env, "ISTIO_VERSION="+manifest.Version)
	// TODO make this less hacky
	if repo == "istio" {
		cmd.Env = append(cmd.Env, "GOBUILDFLAGS=-mod=vendor")
	}
	cmd.Env = append(cmd.Env, env...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = manifest.RepoDir(repo)
	log.Infof("Running make %v with env=%v wd=%v", strings.Join(c, " "), strings.Join(env, " "), cmd.Dir)
	return cmd.Run()
}
