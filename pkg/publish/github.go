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

package publish

import (
	"context"
	"fmt"
	"os"
	"path"

	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"

	"github.com/google/go-github/v28/github"
	"golang.org/x/oauth2"
)

var ptrue = true

// Github triggers a complete release to github. This includes tagging all source branches, and publishing
// a release to the main istio repo.
func Github(manifest model.Manifest, githubOrg string) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)

	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	for _, dep := range manifest.Dependencies {
		// Do not use dep.Org, as the source org is not necessarily the same as the publishing org
		if err := GithubTag(client, githubOrg, dep.Repo, manifest.Version, dep.Sha); err != nil {
			return fmt.Errorf("failed to tag repo %v: %v", dep.Repo, err)
		}
	}

	if err := GithubRelease(manifest, client, githubOrg); err != nil {
		return fmt.Errorf("failed to create release: %v", err)
	}

	return nil
}

// GithubRelease publishes a release.
func GithubRelease(manifest model.Manifest, client *github.Client, githuborg string) error {
	ctx := context.Background()

	// TODO this may not be the right location. Derive from GCS url.
	body := fmt.Sprintf(`[ARTIFACTS](http://gcsweb.istio.io/gcs/istio-release/releases/%s/)
* [istio-sidecar.deb](https://storage.googleapis.com/istio-release/releases/%s/deb/istio-sidecar.deb)
* [istio-sidecar.deb.sha256](https://storage.googleapis.com/istio-release/releases/%s/deb/istio-sidecar.deb.sha256)
* [Helm Chart Index](https://storage.googleapis.com/istio-release/releases/%s/charts/index.yaml)

[RELEASE NOTES](https://istio.io/about/notes/%s.html)`,
		manifest.Version, manifest.Version, manifest.Version, manifest.Version, manifest.Version)

	relName := fmt.Sprintf("Istio %s", manifest.Version)
	rel, _, err := client.Repositories.CreateRelease(ctx, githuborg, "istio", &github.RepositoryRelease{
		TagName:    &manifest.Version,
		Body:       &body,
		Draft:      &ptrue,
		Prerelease: &ptrue,
		Name:       &relName,
	})
	if err != nil {
		return fmt.Errorf("failed to publish github release: %v", err)
	}
	util.YamlLog("Release", rel)

	// TODO upload all assets
	fname := fmt.Sprintf("istio-%s-linux.tar.gz", manifest.Version)
	f, err := os.Open(path.Join(manifest.OutDir(), fname))
	if err != nil {
		return fmt.Errorf("failed to read file %v: %v", fname, err)
	}
	asset, _, err := client.Repositories.UploadReleaseAsset(ctx, githuborg, "istio", *rel.ID, &github.UploadOptions{
		Name: fname,
	}, f)
	if err != nil {
		return fmt.Errorf("failed to upload asset %v: %v", fname, err)
	}
	util.YamlLog("Release asset", asset)

	return nil
}

// GithubTag tags a given repo with a version
func GithubTag(client *github.Client, org string, repo string, version string, sha string) error {
	ctx := context.Background()

	// First, create a tag
	msg := fmt.Sprintf("Istio release %s", version)
	tagType := "commit"
	tag, _, err := client.Git.CreateTag(ctx, org, repo, &github.Tag{
		Tag:     &version,
		Message: &msg,
		Object: &github.GitObject{
			Type: &tagType,
			SHA:  &sha,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create tag: %v", err)
	}
	util.YamlLog("Tag", tag)

	// Then create a reference to the tag
	ref := fmt.Sprintf("refs/tags/%s", version)
	reference, _, err := client.Git.CreateRef(ctx, org, repo, &github.Reference{
		Ref: &ref,
		Object: &github.GitObject{
			Type: &tagType,
			SHA:  &sha,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create tag reference: %v", err)
	}
	util.YamlLog("Reference", reference)

	return nil
}
