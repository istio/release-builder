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
# Note - only istio
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
    branch: release-1.7
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

## Branch

While not all of the release branch steps can be automated, a lot of the work can be. The automated portion of creating the release branches has been broken into `STEPS`. A `STEP` is specified, either via file or enviroment variable, to control which portion of the branching is being done. Branching starts with STEP=1 and progresses through STEP=5. After each `STEP` is run, the created PRs need to be approved and time allowed for those PRs to be merged and any successive automated PRs to complete.

The branch step will pull down all needed sources to branch a release. Generally this is from GitHub, but local files can be used as well for private builds or local testing.

The automated `STEPS`:

* (Automation step=1) Update dependencies
* (Automation step=2) Create the release branches
* (Automation step=3) Set up prow on release branches (requires GCR credentials)
* (Automation step=4) Updates istio/tools to create new build image, common-file update prep, CODEOWNERS
* (Automation step=5) Update common-files with image from step=4.

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
`gcr.io/istio-testing/build-tools:master-2020-11-12T22-29-05`. In this case, one could set IMAGE_VERSION=master-2020-11-12T22-29-05.

Next, create a manifest to use for the builds. A good starting point is the `example/manifest.yaml`.

Create a build container and get to its command line using `make shell` or if you want a specific build image from above
`IMAGE_VERSION=xyz make shell`. The current path in the container is `/work` which will map to the release-builder root directory.

On the command line you specify the commands to do the build using your manifest and then validate the build
specifying the directory specified in the manifest (ex: /tmp/istio-release) appending `/out`.

```bash
mkdir -p /tmp/istio-release; go run main.go build --manifest example/manifest.yaml; go run main.go validate --release /tmp/istio-release/out
```

When the command finishes and you should have an information message:

```text
Release validation PASSED
```

To extract the artifacts from the container, use `docker ps -a` to find the name of the build container, and then run `docker cp` to
copy the artifacts. For example, the command might be `docker cp happy_pare:/tmp/istio-release/out artifacts`. This will place the artifacts in the `artifacts`
directory in your current working directory. The `artifacts` directory will contain the artifacts(subject to change):

| Syntax | Description |
| --- | ----------- |
| istio-{version}-{linux-\<arch>/osx/win}.tar.gz | _Release archive that users will download_ |
| istioctl-{version}-{linux-\<arch>/osx/win}.tar.gz | |
| manifest.yaml | _Defines what dependencies were a part of the build_ |
| sources.tar.gz | _Bundle of all sources used in the build_|
| "charts" subdirectory | _Operator release charts_ |
| "deb" subdirectory | _"istio-sidecar.deb" and it's sha_ |
| "docker" subdirectory | _tar files for the created docker images_ |
| "licenses" subdirectory | _tar.gz of the license files from the specified dependency repos_ |

## Running a branch locally

It's easiest to run a build against an org with has all the istio repos forked:
istio, api, client-go, cni, common-files, envoy, gogo-genproto, pkg, proxy,
release-builder,test-infra, tools.

If you don’t, the initial cloning of the code will fail.

1. Run `make shell` to get a prompt within the build container.

1. Run `docker ps` and verify that works. If it fails, check the istio wiki to see if there are any hints.
    For example, later versions of Docker Desktop for Mac may require an environment variable set so the build
    container can communincate with Docker on the host.

1. As a test which can be run long before the branch cut to verify that this works locally,
    run `REPO_ORG=<your org name (ex. ericvn)> STEP=1 ./release/branch.sh`.

    STEP=1 is `make update_dependencies` and a `make gen` for reference. You will see a git clone of the various
    repos to a tmp directory.  Then a message

    ```text
    *** Updating the istio.istio dependencies.
    ```

    At the end of the `make gen` (it does take a bit), the code will iterate over the repos giving a list of
    changes found for each repo. And you should see something similar to:

    ```text
    2021-01-10T18:18:06.589542Z info  *** Checking repo istio
    2021-01-10T18:18:06.589574Z info  Running command: git status --porcelain
    2021-01-10T18:18:06.861929Z info  changes found:
    M go.mod
    M go.sum
    M istio.deps
    M prow/release-commit.sh
    ```

    and finally a message:

    ```text
    Branch step 1 to release-1.9 done in /tmp/tmp.vqqPvgjZjZ/branch/work
    ```

    If you actually cd to the directory listed `+/istio.io/<repo>` you can do things like `git diff` and
    `git status`.

1. Adding DRY_RUN=false (ex: `REPO_ORG=ericvn STEP=1 DRY_RUN=false ./release/branch.sh`) will create a
    PR in the REPO_ORG/repo if changes are found. It does rely on the git user info being available
    inside the build container. You should end up with a REPO_ORG/istio PR (you can close after
    verifying it’s there if it’s a test). If this fails, verify that the GITHUB_TOKEN or github files
    are available in the container. You may also do a `git login` inside the container to set the
    credentials.

Notes:

* `STEP=2 DRY_RUN=false` needs to be done without REPO_ORG specified so the branches are created in istio.io org.
* STEPs > 2 need to have the branches created so run a STEP=2 against your personal org for dry runs of later steps.
* There is a little magic I am trying to use to limit the number of automated PRs being created by the pipeline
  (only create a single set of automated common-files update PRs instead of 2). In Step 4, I update one of the
  common files which specify which common-files branch to pull from and then in Step 5 when common-files gets
  updated it will cause the pipeline automation to go through the repos to actually update from common-files
  (which uses the branch changed in step 4).
