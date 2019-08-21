package pkg

import (
	"fmt"
	"path"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/util"
	"github.com/pkg/errors"

	"istio.io/pkg/log"
)

func Sources(manifest model.Manifest) error {
	for _, dependency := range manifest.Dependencies {
		if err := util.Clone(dependency, path.Join(manifest.SourceDir(), dependency.Repo)); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to resolve: %+v", dependency))
		}
		log.Infof("Resolved %v", dependency.Repo)
		src := path.Join(manifest.SourceDir(), dependency.Repo)
		if err := util.CopyDir(src, manifest.RepoDir(dependency.Repo)); err != nil {
			return fmt.Errorf("failed to copy dependency %v to working directory: %v", dependency.Repo, err)
		}
	}
	return nil
}
