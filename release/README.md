# Istio Release Pipeline

This folder contains the scripts to trigger official Istio builds. The build and publish step are done in two separate phases.

## Build

First, a build of the release is created. This is done by modifying the [`trigger-build`](./trigger-build) file and submitting a PR.

In CI, Prow will check if the `trigger-build` file has been changed, and if it has it will create a build in the postsubmit.

**WARNING**: Any change to this file will trigger a build, including comments or reverts.

## Publish

Once the build created in the previous step has been validated, it can be published by modifying the [`trigger-publish`](./trigger-publish) file.

In CI, Prow will check if the `trigger-publish` file has been changed, and if it has it will publish the build in the postsubmit.

**WARNING**: Any change to this file will trigger a publish, including comments or reverts.

## Branch

The release branching is controlled by the the [`trigger-branch`](./trigger-branch) file. Currently the release branch code is not automated, but should be easy to add if desired. Prow could check if the `trigger-branch` file has been changed and then run the `branch.sh` script.
