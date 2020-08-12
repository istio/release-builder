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
# Note - only istio and cni
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
# Extra dependencies, just for publish
  api:
    git: https://github.com/istio/api
    auto: modules
  proxy:
    git: https://github.com/istio/proxy
    auto: deps
  envoy:
    git: https://github.com/istio/envoy
    auto: proxy_workspace
# proxyOverride specifies an alternative URL to pull Envoy binary from
proxyOverride: https://storage.googleapis.com/istio-build/proxy
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
* Docker credentials (if publishing to docker) (TODO - how to set these).
* GCP credentials (if publishing to GCS) (TODO - how to set these).
* Grafana credentials (if publishing to grafana): as environment variable `GRAFANA_TOKEN` or `--grafanatoken file`.

## Running a build locally

To build locally and ensure a consistent environment, you need to have Docker installed and run the build in a docker container using a
`gcr.io/istio-testing/build-tools` image. The exact config used, including the specific docker tag, for Istio builds can be found at
<https://github.com/istio/test-infra/blob/master/prow/config/jobs/release-builder.yaml>. For example, the specified image might be
`gcr.io/istio-testing/build-tools:master-2020-02-14T13-09-14`.

Next, create a manifest to use for the builds. A good starting point is the `example/manifest.yaml`.

Using docker, create a container using the above found `build-tools` image. On the command line you specify the commands to do the build and validate which
point at your manifest.yaml file and also the directory specified in the manifest. An example Docker command to start the container and do a build and validate is:

```bash
docker run -it -e BUILD_WITH_CONTAINER="0" -e TZ="`readlink "" /etc/localtime | sed -e 's/^.*zoneinfo\///'`" -v /var/run/docker.sock:/var/run/docker.sock --mount type=bind,source=$(PWD),destination="/work" --mount type=volume,source=go,destination="/go" --mount type=volume,source=gocache,destination="/gocache"  -w /work gcr.io/istio-testing/build-tools:master-2020-02-14T13-09-14 /bin/bash -c "mkdir -p /tmp/istio-release; go run main.go build --manifest example/manifest.yaml; go run main.go validate --release /tmp/istio-release/out"
```

When the command finishes and you should have an information message:

```text
Release validation PASSED
```

and there will be a stopped container which will contain the artifacts
from the build. To extract the artifacts from the stopped container, use `docker ps -a` to find the name of the stopped container, and then run `docker cp` to
copy the artifacts. For example, the command might be `docker cp happy_pare:/tmp/istio-release/out artifacts`. This will place the artifacts in the `artifacts`
directory in your current working directory. The `artifacts` directory will contain the artifacts(subject to change):

| Syntax | Description |
| --- | ----------- |
| istio-{version}-{linux-<arch>/osx/win}.tar.gz | _Release archive that users will download_ |
| istioctl-{version}-{linux-<arch>/osx/win}.tar.gz | |
| manifest.yaml | _Defines what dependencies were a part of the build_ |
| sources.tar.gz | _Bundle of all sources used in the build_|
| "charts" subdirectory | _Operator release charts_ |
| "deb" subdirectory | _"istio-sidecar.deb" and it's sha_ |
| "docker" subdirectory | _tar files for the created docker images_ |
| "licenses" subdirectory | _tar.gz of the license files from the specified dependency repos_ |
