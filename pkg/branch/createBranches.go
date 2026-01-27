// Copyright Istio Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package branch

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// CreateBranches goes to each repo and creates the branches
func CreateBranches(manifest model.Manifest, release string, dryrun bool, githubToken string) error {
	log.Infof("*** Creating release branches for release: %s", release)
	for repo, dep := range manifest.Dependencies.Get() {
		if dep == nil {
			// Missing a dependency is not always a failure; many are optional dependencies just for
			// tagging.
			log.Infof("skipping missing dependency: %v", repo)
			continue
		}
		// test-infra does not use release branches and envoy repo should be manually branched
		// from correct envoy commit
		if repo == "test-infra" {
			log.Infof("Skipping repo: %v", repo)
			continue
		}

		branchName := "release-" + release
		branchExists, err := remoteBranchExists(manifest.RepoDir(repo), branchName, githubToken)
		if err != nil {
			log.Warnf("Failed to check if branch %s exists in repo %s: %v", branchName, repo, err)
		} else if branchExists {
			log.Warnf("Branch %s already exists in repo %s. This may cause issues; please verify and delete it if needed.", branchName, repo)
		}

		log.Infof("*** Creating a release branch %s for %s from directory: %s", release, repo, manifest.RepoDir(repo))
		cmd := util.VerboseCommand("git", "checkout", "-b", branchName)
		cmd.Dir = manifest.RepoDir(repo)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to checkout release: %v", err)
		}
		if !dryrun {
			r, err := git.PlainOpen(manifest.RepoDir(repo))
			if err != nil {
				return fmt.Errorf("failed to open repository %s: %v", repo, err)
			}
			err = r.Push(&git.PushOptions{
				RemoteName: "origin",
				Auth: &http.BasicAuth{
					Username: "git",
					Password: githubToken,
				},
				RefSpecs: []config.RefSpec{
					config.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName),
				},
			})
			if err != nil {
				log.Warnf("failed to push branch to repo: %v. Ignoring as it may already exist.", err)
			}
		}
	}
	log.Infof("*** Release branches created")
	return nil
}

func remoteBranchExists(repoDir, branchName, githubToken string) (bool, error) {
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		return false, fmt.Errorf("failed to open repository %s: %v", repoDir, err)
	}

	remote, err := r.Remote("origin")
	if err != nil {
		return false, fmt.Errorf("failed to get origin remote: %v", err)
	}

	listOptions := &git.ListOptions{}
	if githubToken != "" {
		listOptions.Auth = &http.BasicAuth{
			Username: "git",
			Password: githubToken,
		}
	}

	refs, err := remote.List(listOptions)
	if err != nil {
		return false, fmt.Errorf("failed to list remote branches: %v", err)
	}

	targetRef := "refs/heads/" + branchName
	for _, ref := range refs {
		if ref.Name().String() == targetRef {
			return true, nil
		}
	}
	return false, nil
}
