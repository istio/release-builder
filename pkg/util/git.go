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

package util

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v35/github"
	"golang.org/x/oauth2"

	"istio.io/istio/pkg/log"
	"istio.io/release-builder/pkg/model"
)

// PushCommit will look for changes. If changes exist, it will create a branch and push a commit with the specified commit text
// to the upstremam repo.
func PushCommit(manifest model.Manifest, repo, branch, commitString string, dryrun bool, githubToken string, user github.User) (changes bool, err error) {
	// Use go-git since it will take an already cloned and changed file-system and use that as a
	// working tree to create the commit instead of using `git` commands. This allows the use of
	// the passed in github token without it leaking in the logs.
	r, err := git.PlainOpen(manifest.RepoDir(repo))
	if err != nil {
		return false, fmt.Errorf("failed to open path: %v", err)
	}
	w, err := r.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to retrieve work tree: %v", err)
	}

	// Get the worktree status to see if there are any changes. Return if none.
	status, err := w.Status()
	if err != nil {
		return false, fmt.Errorf("failed to retrieve status: %v", err)
	}
	if status.IsClean() {
		log.Infof("no changes found to commit")
		return false, nil
	}
	log.Infof("changes found:\n%v", &status)

	// If a dry_run, create a commit and push to the upstream repo
	if !dryrun {
		// Add the changed files to staging
		// w.AddWithOptions added some ignored files, like out/.env, so possibly
		// an issue with the library. Add the files noted as changed in the worktree status.
		for changedFile := range status {
			_, err = w.Add(changedFile)
			if err != nil {
				return true, fmt.Errorf("failed to add file to staging %s: %v", changedFile, err)
			}
		}

		// Checkout the specified branch
		err = w.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(branch),
			Create: true,
			Keep:   true,
		})
		if err != nil {
			return true, fmt.Errorf("failed to checkout branch: %v", err)
		}

		// Create a commit on that branch
		// user.Email may be nil if set to private in GitHub,
		// fall back to global gitconfig, which may be an empty string
		if user.Email == nil {
			cfg, err := config.LoadConfig(config.GlobalScope)
			if err != nil {
				emptyString := ""
				user.Email = &emptyString
			} else {
				user.Email = &cfg.User.Email
			}
		}
		commit, err := w.Commit(commitString, &git.CommitOptions{
			Author: &object.Signature{
				Name:  *user.Name,
				Email: *user.Email,
				When:  time.Now(),
			},
		})
		if err != nil {
			return true, fmt.Errorf("failed to create commit: %v", err)
		}
		log.Infof("commit created:\n%v", commit)

		// Push to the upstream repo.
		err = r.Push(&git.PushOptions{
			Auth: &http.BasicAuth{
				Username: *user.Name, // yes, this can be anything except an empty string
				Password: githubToken,
			},
		})
		if err != nil {
			return true, fmt.Errorf("failed to push branch '%s' to repository '%s': %v", branch, repo, err)
		}
	}
	return true, nil
}

// CreatePR will look for changes. If changes exist, it will create a branch and push a commit with
// the specified commit text, and then create a PR in the upstream repo.
func CreatePR(manifest model.Manifest, repo, newBranchName, commitString, description string, dryrun bool, githubToken, git, branch string,
	labels []string, prRepoOrg string,
) error {
	// Set git and branch from manifest if not passed in
	if git == "" {
		git = manifest.Dependencies.Get()[repo].Git
	}

	if branch == "" {
		branch = manifest.Dependencies.Get()[repo].Branch
	}
	// Get client to access GH and then get user for GH token. Only needed if not a dryrun.
	var client *github.Client
	var ctx context.Context
	user := &github.User{} // default to empty user for PushCommit call
	if !dryrun {
		ctx = context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
		var err error
		user, _, err = client.Users.Get(ctx, "")
		if err != nil {
			return err
		}

		// Check if the branch already exists remotely before attempting to push
		repoStrings := strings.Split(git, "/")
		l := len(repoStrings)
		orgString := repoStrings[l-2]
		repoString := repoStrings[l-1]

		log.Infof("Checking if branch '%s' already exists in %s/%s", newBranchName, orgString, repoString)
		existingBranch, _, err := client.Repositories.GetBranch(ctx, orgString, repoString, newBranchName)
		if err == nil && existingBranch != nil {
			return fmt.Errorf(
				"branch '%s' already exists in %s/%s. "+
					"Please delete it or use a different branch name with BRANCH_SUFFIX: %s",
				newBranchName, orgString, repoString, newBranchName)
		}
		// If we get a 404 error, the branch doesn't exist (expected).
		if err != nil && !strings.Contains(err.Error(), "404") {
			log.Warnf("Could not check if branch '%s' exists (proceeding anyway): %v", newBranchName, err)
		} else if err != nil {
			log.Infof("Branch '%s' does not exist (as expected), proceeding with PR creation", newBranchName)
		}
	}

	changes, err := PushCommit(manifest, repo, newBranchName, commitString, dryrun, githubToken, *user)
	if err != nil {
		return err
	}

	if changes {
		newPR := &github.NewPullRequest{
			Title:               &commitString,
			Head:                &newBranchName,
			Base:                &branch,
			Body:                &description,
			MaintainerCanModify: github.Bool(true),
		}

		repoStrings := strings.Split(git, "/")
		l := len(repoStrings)
		orgString := repoStrings[l-2]
		repoString := repoStrings[l-1]

		if prRepoOrg != "" && prRepoOrg != orgString {
			log.Infof("create PR from a fork %s -> %s", orgString, prRepoOrg)
			// The name of the branch where your changes are implemented.
			// For cross-repository pull requests in the same network,
			// namespace head with a user like this: username:branch.
			head := fmt.Sprintf("%s:%s", orgString, newBranchName)
			newPR.Head = &head
			orgString = prRepoOrg
		}

		log.Infof("Creating PR, org: %s repo: %s base: %s head: %s",
			orgString, repoString, *newPR.Base, *newPR.Head)
		if dryrun {
			log.Infof("Skipping, DRY_RUN=true")
			return nil
		}

		pr, _, err := client.PullRequests.Create(ctx, orgString, repoString, newPR)
		if err != nil {
			return err
		}

		log.Infof("PR created: %s\n", pr.GetHTMLURL())

		// Add additional supplied labels plus release-notes-note in non-envoy repos
		if orgString == "istio" && repoString != "envoy" {
			labels = append(labels, []string{"release-notes-none"}...)
		}

		var label []*github.Label
		if len(labels) > 0 {
			label, _, err = client.Issues.AddLabelsToIssue(ctx, orgString, repoString, *pr.Number, labels)
			if err != nil {
				return err
			}
		}
		log.Infof("Labels:\n%v", label)
	}

	return nil
}

// GetGithubToken returns the GitHub token from the specified file. If the filename
// isn't specified, it will return the token set in the GITHUB_TOKEN environment variable.
func GetGithubToken(file string) (string, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("failed to read github token: %v", file)
		}
		return strings.TrimSpace(string(b)), nil
	}
	if t, f := os.LookupEnv("GH_TOKEN"); f {
		return t, nil
	}
	return os.Getenv("GITHUB_TOKEN"), nil
}
