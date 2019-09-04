package publish

import (
	"context"
	"fmt"
	"os"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"

	"github.com/google/go-github/v28/github"
	"golang.org/x/oauth2"
)

var ptrue = true

func Github(manifest model.Manifest, githuborg string) error {

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	for _, dep := range manifest.Dependencies {
		// Do not use dep.Org, as the source org is not necessarily the same as the publishing org
		if err := GithubTag(client, githuborg, dep.Repo, manifest.Version, dep.Sha); err != nil {
			return fmt.Errorf("failed to tag repo %v: %v", dep.Repo, err)
		}
	}

	if err := GithubRelease(manifest, client, githuborg); err != nil {
		return fmt.Errorf("failed to create release: %v", err)
	}

	return nil
}

func GithubRelease(manifest model.Manifest, client *github.Client, githuborg string) error {
	ctx := context.Background()

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
	return nil
}

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
