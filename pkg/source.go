package pkg

import (
	"bytes"
	"fmt"
	"path"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"

	"istio.io/pkg/log"
)

func Sources(manifest model.Manifest) error {
	for _, dependency := range manifest.Dependencies {
		if err := util.Clone(dependency, path.Join(manifest.SourceDir(), dependency.Repo)); err != nil {
			return fmt.Errorf("failed to resolve %+v: %v", dependency, err)
		}
		log.Infof("Resolved %v", dependency.Repo)
		src := path.Join(manifest.SourceDir(), dependency.Repo)
		if err := util.CopyDir(src, manifest.RepoDir(dependency.Repo)); err != nil {
			return fmt.Errorf("failed to copy dependency %v to working directory: %v", dependency.Repo, err)
		}
		if err := TagRepo(manifest, dependency.Repo); err != nil {
			return fmt.Errorf("failed to tag repo %v: %v", dependency.Repo, err)
		}
	}
	return nil
}

func TagRepo(manifest model.Manifest, repo string) error {
	cmd := util.VerboseCommand("git", "tag", manifest.Version)
	cmd.Dir = manifest.RepoDir(repo)
	return cmd.Run()
}

func StandardizeManifest(manifest *model.Manifest) error {
	for i, dep := range manifest.Dependencies {
		buf := bytes.Buffer{}
		cmd := util.VerboseCommand("git", "rev-parse", "HEAD")
		cmd.Stdout = &buf
		cmd.Dir = manifest.RepoDir(dep.Repo)
		if err := cmd.Run(); err != nil {
			return err
		}
		dep.Sha = buf.String()
		dep.Branch = ""
		manifest.Dependencies[i] = dep
	}
	return nil
}
