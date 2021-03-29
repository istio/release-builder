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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v28/github"
	"golang.org/x/oauth2"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
)

// PushCommit will create a branch and push a commit with the specified commit text
func PushCommit(manifest model.Manifest, repo, branch, commitString string, dryrun bool, githubToken string) (changes bool, err error) {
	output := bytes.Buffer{}
	cmd := VerboseCommand("git", "status", "--porcelain")
	cmd.Dir = manifest.RepoDir(repo)
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return false, err
	}
	if output.Len() == 0 {
		log.Infof("no changes found to commit")
		return false, nil
	}
	log.Infof("changes found:\n%s", &output)

	if !dryrun {
		cmd = VerboseCommand("git", "checkout", "-b", branch)
		cmd.Dir = manifest.RepoDir(repo)
		if err := cmd.Run(); err != nil {
			return true, err
		}

		cmd = VerboseCommand("git", "add", "-A")
		cmd.Dir = manifest.RepoDir(repo)
		if err := cmd.Run(); err != nil {
			return true, err
		}

		// Get user.email and user.name from the GitHub token
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)

		tc := oauth2.NewClient(ctx, ts)
		client := github.NewClient(tc)

		user, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return true, err
		}

		cmd = VerboseCommand("git", "commit", "-m", commitString, "-c", "user.name="+*user.Name, "-c", "user.email="+*user.Email,
			"--author="+*user.Name+"<"+*user.Email+">")
		cmd.Dir = manifest.RepoDir(repo)
		if err := cmd.Run(); err != nil {
			return true, err
		}

		cmd = VerboseCommand("git", "push", "--set-upstream", "origin", branch)
		cmd.Dir = manifest.RepoDir(repo)
		if err := cmd.Run(); err != nil {
			return true, err
		}
	}
	return true, nil
}

// CreatePR will look for changes. If changes exist, it will create
// a branch and push a commit with the specified commit text
func CreatePR(manifest model.Manifest, repo, branch, commitString string, dryrun bool, githubToken string) error {
	changes, err := PushCommit(manifest, repo, branch, commitString, dryrun, githubToken)
	if err != nil {
		return err
	}

	if changes && !dryrun {
		var cmd *exec.Cmd
		if repo != "envoy" {
			cmd = VerboseCommand("gh", "pr", "create", "--repo", manifest.Dependencies.Get()[repo].Git,
				"--fill", "--head", branch, "--base", manifest.Dependencies.Get()[repo].Branch, "--label", "release-notes-none")
		} else {
			cmd = VerboseCommand("gh", "pr", "create", "--repo", manifest.Dependencies.Get()[repo].Git,
				"--fill", "--head", branch, "--base", manifest.Dependencies.Get()[repo].Branch)
		}
		cmd.Dir = manifest.RepoDir(repo)
		if err := cmd.Run(); err != nil {
			return err
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
