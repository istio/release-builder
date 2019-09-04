package util

import (
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/howardjohn/istio-release/pkg/model"

	"istio.io/pkg/log"
)

func RunMake(manifest model.Manifest, repo string, env []string, c ...string) error {
	cmd := VerboseCommand("make", c...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOPATH="+manifest.WorkDir(), "TAG="+manifest.Version, "ISTIO_VERSION="+manifest.Version)
	cmd.Env = append(cmd.Env, env...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = manifest.RepoDir(repo)
	log.Infof("Running make %v with env=%v wd=%v", strings.Join(c, " "), strings.Join(env, " "), cmd.Dir)
	return cmd.Run()
}

func YamlLog(prefix string, i interface{}) {
	manifestYaml, _ := yaml.Marshal(i)
	log.Infof("%s: %v", prefix, string(manifestYaml))
}
