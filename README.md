# Istio Release

This repository defines the release process for Istio.

The release process is split into two phases, the build and publish

## Build

The build step will pull down all needed sources to build a release. Generally this is from GitHub, but local files can be used as well for private builds or local testing.

Next, it will build a variety of different artifacts, including a `manifest.yaml` which defines what dependencies were a part of the build.

While not completely possible today, the goal is for the build process to be runnable in an air gapped environment once all dependencies have been downloaded.

## Publish

The publish step takes in the build artifacts as an input, and publishes them to a variety of places:

* Copy artifacts to GCS
* Push docker images
* Tag all Github source repositories
* Publish a Github release

All of these steps can be done in isolation. For example, a daily build will first publish to a staging GCS and dockerhub, then once testing has completed publish again to all locations.
