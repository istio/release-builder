#!/bin/bash

# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

WD=$(dirname "$0")
WD=$(cd "$WD"; pwd)

set -eu

SOURCE_GCS_BUCKET=${SOURCE_GCS_BUCKET:-istio-prerelease/prerelease}
GCS_BUCKET=${GCS_BUCKET:-istio-release/releases}
DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}
GITHUB_ORG=${GITHUB_ORG:-istio}

VERSION="$(cat "${WD}/trigger-publish")"

cat <<EOF
WARNING
If you are seeing this test, you have modified the trigger-publish file.

This will publish an official build of Istio once merged.

If this is unexpected, do not merge this PR.

If this is expected, this message can be ignored.

Build information
=================
Version: ${VERSION}

GCS Bucket: ${GCS_BUCKET}
Docker Hub: ${DOCKER_HUB}
Github Org: ${GITHUB_ORG}
Source: ${SOURCE_GCS_BUCKET}/${VERSION}
Contents:
$(gsutil cat "gs://${SOURCE_GCS_BUCKET}/${VERSION}/manifest.yaml")
$(gsutil ls -r "gs://${SOURCE_GCS_BUCKET}/${VERSION}")
EOF

exit 2
