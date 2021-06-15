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
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v35/github"
	"golang.org/x/oauth2"

	"istio.io/pkg/log"
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

	// Git the worktree status to see if there are any changes. Return if none.
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
		// user.Email may be nil, so set to an empty string
		emptyString := ""
		if user.Email == nil {
			user.Email = &emptyString
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
			return true, fmt.Errorf("failed to push: %v", err)
		}
	}
	return true, nil
}

// CreatePR will look for changes. If changes exist, it will create a branch and push a commit with
// the specified commit text, and then create a PR in the upstream repo.
func CreatePR(manifest model.Manifest, repo, branch, commitString string, dryrun bool, githubToken string) error {
	// Get user from the GitHub token
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return err
	}

	changes, err := PushCommit(manifest, repo, branch, commitString, dryrun, githubToken, *user)
	if err != nil {
		return err
	}

	if changes && !dryrun {
		newPR := &github.NewPullRequest{
			Title:               &commitString,
			Head:                &branch,
			Base:                &manifest.Dependencies.Get()[repo].Branch,
			MaintainerCanModify: github.Bool(true),
		}

		repoStrings := strings.Split(manifest.Dependencies.Get()[repo].Git, "/")
		l := len(repoStrings)
		orgString := repoStrings[l-2]
		repoString := repoStrings[l-1]

		pr, _, err := client.PullRequests.Create(ctx, orgString, repoString, newPR)
		if err != nil {
			return err
		}

		log.Infof("PR created: %s\n", pr.GetHTMLURL())

		// Add a release-notes-none label on the PR if istio org, but not envoy repo.
		if orgString == "istio" && repoString != "envoy" {
			label, _, err := client.Issues.AddLabelsToIssue(ctx, orgString, repoString, *pr.Number, []string{"release-notes-none"})
			if err != nil {
				return err
			}
			log.Infof("Labels:\n%v", label)
		}

	}
	return nil
}

// GetGithubToken returns the GitHub token from the specified file. If the filename
// isn't specified, it will return the token set in the GITHUB_TOKEN environment variable.
func GetGithubToken(file string) (string, error) {
	if file != "" {
		b, err := ioutil.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("failed to read github token: %v", file)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return os.Getenv("GITHUB_TOKEN"), nil
}
