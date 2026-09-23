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

// pushCommitFn is the function used by CreatePR and CreateOrUpdatePR to push commits.
// It is a variable so tests can substitute a stub without a real git repo.
var pushCommitFn = PushCommit

// PushCommit will look for changes. If changes exist, it will create a branch and push a commit with the specified commit text
// to the upstream repo. Set force to true to force-push when updating an existing remote branch.
func PushCommit(manifest model.Manifest, repo, branch, commitString string,
	dryrun bool, githubToken string, user github.User, force bool,
) (changes bool, err error) {
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
			Force: force,
		})
		if err != nil {
			return true, fmt.Errorf("failed to push branch '%s' to repository '%s': %v", branch, repo, err)
		}
	}
	return true, nil
}

// parseOrgRepo extracts the GitHub org and repo name from a git URL.
func parseOrgRepo(gitURL string) (org, repo string) {
	parts := strings.Split(gitURL, "/")
	l := len(parts)
	return parts[l-2], parts[l-1]
}

// setupGithubClientFn is the function used by CreatePR and CreateOrUpdatePR to build the GitHub
// client. It is a variable so tests can substitute a stub that points at a mock HTTP server.
var setupGithubClientFn = setupGithubClient

// setupGithubClient creates a GitHub API client and fetches the authenticated user.
func setupGithubClient(githubToken string) (*github.Client, context.Context, *github.User, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, nil, nil, err
	}
	return client, ctx, user, nil
}

// openNewPR creates a pull request and applies the supplied labels.
// In non-envoy istio repos it also appends "release-notes-none".
func openNewPR(ctx context.Context, client *github.Client, org, repo, head, base, commitString, description string, labels []string) error {
	newPR := &github.NewPullRequest{
		Title:               &commitString,
		Head:                &head,
		Base:                &base,
		Body:                &description,
		MaintainerCanModify: github.Bool(true),
	}
	log.Infof("Creating PR, org: %s repo: %s base: %s head: %s", org, repo, base, head)
	pr, _, err := client.PullRequests.Create(ctx, org, repo, newPR)
	if err != nil {
		return err
	}
	log.Infof("PR created: %s\n", pr.GetHTMLURL())

	if org == "istio" && repo != "envoy" {
		labels = append(labels, "release-notes-none")
	}
	if len(labels) > 0 {
		label, _, err := client.Issues.AddLabelsToIssue(ctx, org, repo, *pr.Number, labels)
		if err != nil {
			return err
		}
		log.Infof("Labels:\n%v", label)
	}
	return nil
}

// CreatePR will look for changes. If changes exist, it will create a branch and push a commit with
// the specified commit text, and then create a PR in the upstream repo.
func CreatePR(manifest model.Manifest, repo, newBranchName, commitString, description string, dryrun bool, githubToken, git, branch string,
	labels []string, prRepoOrg string,
) error {
	if git == "" {
		git = manifest.Dependencies.Get()[repo].Git
	}
	if branch == "" {
		branch = manifest.Dependencies.Get()[repo].Branch
	}

	var client *github.Client
	var ctx context.Context
	user := &github.User{}
	if !dryrun {
		var err error
		client, ctx, user, err = setupGithubClientFn(githubToken)
		if err != nil {
			return err
		}

		orgString, repoString := parseOrgRepo(git)
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

	changes, err := pushCommitFn(manifest, repo, newBranchName, commitString, dryrun, githubToken, *user, false)
	if err != nil {
		return err
	}

	if changes {
		orgString, repoString := parseOrgRepo(git)
		head := newBranchName
		if prRepoOrg != "" && prRepoOrg != orgString {
			log.Infof("create PR from a fork %s -> %s", orgString, prRepoOrg)
			// For cross-repository pull requests, namespace head as username:branch.
			head = fmt.Sprintf("%s:%s", orgString, newBranchName)
			orgString = prRepoOrg
		}

		if dryrun {
			log.Infof("Skipping, DRY_RUN=true")
			return nil
		}

		return openNewPR(ctx, client, orgString, repoString, head, branch, commitString, description, labels)
	}

	return nil
}

// CreateOrUpdatePR looks for an existing open PR on stableBranchName. If one is found it
// force-pushes the new commit and edits the PR title/body; otherwise it creates a new PR.
// This prevents duplicate PRs when a periodic job runs while a previous PR is still open.
func CreateOrUpdatePR(manifest model.Manifest, repo, stableBranchName, commitString, description string, dryrun bool, githubToken, git, branch string,
	labels []string, prRepoOrg string,
) error {
	if git == "" {
		git = manifest.Dependencies.Get()[repo].Git
	}
	if branch == "" {
		branch = manifest.Dependencies.Get()[repo].Branch
	}

	orgString, repoString := parseOrgRepo(git)

	var client *github.Client
	var ctx context.Context
	user := &github.User{}
	existingPRNumber := 0
	if !dryrun {
		var err error
		client, ctx, user, err = setupGithubClientFn(githubToken)
		if err != nil {
			return err
		}

		// Search for an existing open PR on the stable branch.
		searchQuery := fmt.Sprintf("repo:%s/%s is:pr is:open head:%s", orgString, repoString, stableBranchName)
		results, _, err := client.Search.Issues(ctx, searchQuery, &github.SearchOptions{})
		if err != nil {
			log.Warnf("Could not search for existing PRs on branch '%s' (proceeding as new PR): %v", stableBranchName, err)
		} else if len(results.Issues) > 0 {
			existingPRNumber = results.Issues[0].GetNumber()
			log.Infof("Found existing open PR #%d on branch '%s', will update it", existingPRNumber, stableBranchName)
		}
	}

	// Always force-push: this automation exclusively owns the stable branch, so there is no
	// history worth preserving. A non-forced push would fail if the branch exists from a prior
	// run whose PR was merged but whose branch was not auto-deleted.
	changes, err := pushCommitFn(manifest, repo, stableBranchName, commitString, dryrun, githubToken, *user, true)
	if err != nil {
		return err
	}
	if !changes {
		return nil
	}

	if dryrun {
		log.Infof("Skipping PR create/update, DRY_RUN=true")
		return nil
	}

	targetOrg := orgString
	prHead := stableBranchName
	if prRepoOrg != "" && prRepoOrg != orgString {
		log.Infof("create PR from a fork %s -> %s", orgString, prRepoOrg)
		prHead = fmt.Sprintf("%s:%s", orgString, stableBranchName)
		targetOrg = prRepoOrg
	}

	if existingPRNumber != 0 {
		_, _, err = client.PullRequests.Edit(ctx, targetOrg, repoString, existingPRNumber,
			&github.PullRequest{Title: &commitString, Body: &description})
		if err != nil {
			return fmt.Errorf("failed to update PR #%d: %v", existingPRNumber, err)
		}
		log.Infof("Updated existing PR #%d: https://github.com/%s/%s/pull/%d\n", existingPRNumber, targetOrg, repoString, existingPRNumber)
		return nil
	}

	return openNewPR(ctx, client, targetOrg, repoString, prHead, branch, commitString, description, labels)
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

// ValidateGithubToken checks if a GitHub token is available.
// This provides early validation and clear error messages to users.
func ValidateGithubToken(token string) error {
	if token == "" {
		return fmt.Errorf("GitHub token is required when dry-run is disabled.\n\n" +
			"To set up authentication in the container, run:\n" +
			"  GITHUB_TOKEN=$(gh auth token) make shell\n\n" +
			"Alternative methods:\n" +
			"  - Use --githubtoken flag to specify a token file\n" +
			"  - Set the GH_TOKEN environment variable\n" +
			"  - Set the GITHUB_TOKEN environment variable")
	}

	return nil
}
