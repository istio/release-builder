# Istio Release

This repository defines the release process for Istio.

The release process is split into two phases, the build and publish

## Build

The build step will pull down all needed sources to build a release. Generally this is from GitHub, but local files can be used as well for private builds or local testing.

Next, it will build a variety of different artifacts, including a `manifest.yaml` which defines what dependencies were a part of the build.

While not completely possible today, the goal is for the build process to be runnable in an air gapped environment once all dependencies have been downloaded.

### Manifest

A build takes a `manifest.yaml` to determine what to build. See below for possible values:

```yaml
# Version specifies which version is being built
# This is use for `--version`, metrics, and determining proxy capabilities.
# Note that since this determines proxy capabilities, it is desirable to follow Istio semver
version: 1.2.3

# Some of the artifacts have a docker hub built in - currently this is the operator and Helm charts
# The specified docker hub will be used there. Note - this does not effect where the images are published,
# but if the images are not later published to this hub the charts will not pull a valid image
docker: docker.io/istio

# Directory specifies the working directory to build in
directory: /tmp/istio-release

# Dependencies specifies dependencies of the build
# Note - only istio, cni, and operator are strictly required
# Other dependencies are only required to grab licenses and publish tags to Github.
# Fields:
#   localpath: rather than pull from git, copy a local git repository
#
#   git: specifies the git source to pull from
#     branch: branch to pull from git
#     sha: sha to pull from git
#     auto: rather than a static branch/sha, determine the sha to use from istio/istio.
#           possible values are `deps` to check istio.deps, and `modules` to check go.mod
dependencies:
  istio:
    git: https://github.com/istio/istio
    branch: release-1.4
  cni:
    git: https://github.com/istio/cni
    auto: deps
  operator:
    git: https://github.com/istio/operator
    auto: modules
# Extra dependencies, just for publish
  api:
    git: https://github.com/istio/api
    auto: modules
  proxy:
    git: https://github.com/istio/proxy
    auto: deps
# proxyOverride specifies an Envoy binary to use, instead of pulling one from GCS
proxyOverride: /path/to/envoy
```

## Publish

The publish step takes in the build artifacts as an input, and publishes them to a variety of places:

* Copy artifacts to GCS
* Push docker images
* Tag all Github source repositories
* Publish a Github release

All of these steps can be done in isolation. For example, a daily build will first publish to a staging GCS and dockerhub, then once testing has completed publish again to all locations.

### Credentials

The following credentials are needed
* Github token: as environment variable `GITHUB_TOKEN` or `--githubtoken file`.
* Docker credentials (TODO - how to set these).
* GCP credentials for GCS (TODO - how to set these).

## Running a build

To ensure a consistent environment, the build should run in a docker container, `gcr.io/istio-testing/build-tools`.

The exact config used, including the specific docker tag, for Istio builds can be found at <https://github.com/istio/test-infra/blob/master/prow/config/jobs/release-builder.yaml>.
