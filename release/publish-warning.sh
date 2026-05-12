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
set +x

SOURCE_R2_BUCKET=${SOURCE_R2_BUCKET:-istio-prerelease/prerelease}
R2_BUCKET=${R2_BUCKET:-istio-release/releases}
GCS_BUCKET=${GCS_BUCKET:-istio-release/releases}
DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}
GITHUB_ORG=${GITHUB_ORG:-istio}

VERSION="$(cat "${WD}/trigger-publish")"

ENDPOINT="$(echo "${CF_CREDENTIALS}" | jq -r '.endpoint' | tr -d '\n')"
AWS_ACCESS_KEY_ID="$(echo "${CF_CREDENTIALS}" | jq -r '.access_key' | tr -d '\n')"
AWS_SECRET_ACCESS_KEY="$(echo "${CF_CREDENTIALS}" | jq -r '.secret_key' | tr -d '\n')"
AWS_REGION="$(echo "${CF_CREDENTIALS}" | jq -r '.region' | tr -d '\n')"
AWS_SESSION_TOKEN="$(echo "${CF_CREDENTIALS}" | jq -r '.session_token' | tr -d '\n')"
export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION AWS_SESSION_TOKEN

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
R2 Bucket: ${R2_BUCKET}
Docker Hub: ${DOCKER_HUB}
Github Org: ${GITHUB_ORG}
Source: ${SOURCE_R2_BUCKET}/${VERSION}

R2 Contents (will publish to GCS and R2 from this):
$(aws --endpoint-url "${ENDPOINT}" s3 cp "s3://${SOURCE_R2_BUCKET}/${VERSION}/manifest.yaml" -)
$(aws --endpoint-url "${ENDPOINT}" s3 ls "s3://${SOURCE_R2_BUCKET}/${VERSION}/" --recursive)
EOF

exit 2
