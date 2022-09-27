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
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v35/github"
	"golang.org/x/oauth2"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

var ptrue = true

var githubArtifiactsPattern = regexp.MustCompile("istio.*")

// Github triggers a complete release to github. This includes tagging all source branches, and publishing
// a release to the main istio repo.
func Github(manifest model.Manifest, githubOrg string, githubToken string) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)

	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	for repo, dep := range manifest.Dependencies.Get() {
		if dep == nil {
			log.Warnf("skipping missing dependency %v", repo)
			continue
		}
		// Do not use dep.Org, as the source org is not necessarily the same as the publishing org
		if err := GithubTag(client, githubOrg, repo, manifest.Version, dep.GoVersionEnabled, dep.Sha); err != nil {
			return fmt.Errorf("failed to tag repo %v: %v", repo, err)
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

	body := fmt.Sprintf(`[Artifacts](http://gcsweb.istio.io/gcs/istio-release/releases/%s/)
[Release Notes](https://istio.io/news/releases/%s/announcing-%s/)`,
		manifest.Version, manifest.Version[:strings.LastIndex(manifest.Version, ".")]+".x", manifest.Version)

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

	if err := GithubUploadReleaseAssets(ctx, manifest, client, githuborg, rel); err != nil {
		return fmt.Errorf("failed to publish github release assets: %v", err)
	}
	return nil
}

func GithubUploadReleaseAssets(ctx context.Context, manifest model.Manifest, client *github.Client, githuborg string, rel *github.RepositoryRelease) error {
	files, err := ioutil.ReadDir(path.Join(manifest.Directory))
	if err != nil {
		return err
	}
	for _, file := range files {
		fname := file.Name()
		if githubArtifiactsPattern.MatchString(fname) {
			log.Infof("github: uploading file %v", fname)
			f, err := os.Open(path.Join(manifest.Directory, fname))
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
		} else {
			log.Infof("github: skipping upload of file %v", fname)
		}
	}
	return nil
}

// GithubTag tags a given repo with a version
func GithubTag(client *github.Client, org string, repo string, version string, goVersionEnabled bool, sha string) error {
	ctx := context.Background()

	// append `v` in front of the Istio version number to comply with
	// Go module versioning convention.
	if goVersionEnabled && !strings.HasPrefix(version, "v") {
		if _, err := semver.StrictNewVersion(version); err != nil {
			return fmt.Errorf("cannot tag %v with invalid semantic version %v: %v", repo, version, err)
		}
		version = fmt.Sprintf("v%s", version)
	}

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
			SHA:  tag.SHA,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create tag reference: %v", err)
	}
	util.YamlLog("Reference", reference)

	return nil
}
