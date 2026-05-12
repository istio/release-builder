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
ROOT=$(dirname "$WD")

# Ensure we are running from the repo root
cd "${ROOT}"

set -eu
set +x

if [[ -n "${DOCKER_CONFIG:-}" ]]; then
  # If DOCKER_CONFIG is set, we are mounting a known docker config.
  # we will want to merge in gcloud options, so we can push to GCR *and* the other (docker hub) credentials.
  # However, DOCKER_CONFIG is a read only mount. So we copy it to somewhere writeable then merge in the GCR creds
  mkdir ~/.docker
  cp "${DOCKER_CONFIG}/config.json" ~/.docker/
  export DOCKER_CONFIG=~/.docker
  gcloud auth configure-docker -q
fi
# No else needed - the prow entrypoint already runs configure-docker for standard cases


VERSION="$(cat "${WD}/trigger-publish")"

SOURCE_R2_BUCKET=${SOURCE_R2_BUCKET:-istio-prerelease/prerelease}
GCS_BUCKET=${GCS_BUCKET:-istio-release/releases}
R2_BUCKET=${R2_BUCKET:-istio-release/releases}
HELM_BUCKET=${HELM_BUCKET:-istio-release/charts}
R2_HELM_BUCKET=${R2_HELM_BUCKET:-istio-release/charts}
HELM_HUB_RELEASE=${HELM_HUB_RELEASE:-gcr.io/istio-release/charts}
# We actually push to these hubs. This doesn't affect the default hub in Helm charts
DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}
DOCKER_HUB_MIRROR=${DOCKER_HUB_MIRROR:-gcr.io/istio-release}
GITHUB_ORG=${GITHUB_ORG:-istio}
GITHUB_TOKEN_FILE=${GITHUB_TOKEN_FILE:-}
GRAFANA_TOKEN_FILE=${GRAFANA_TOKEN_FILE:-}
COSIGN_KEY=${COSIGN_KEY:-}

WORK_DIR="$(mktemp -d)/release"
mkdir -p "${WORK_DIR}"

ENDPOINT="$(echo "${CF_PRERELEASE_CREDENTIALS}" | jq -r '.endpoint' | tr -d '\n')"
AWS_ACCESS_KEY_ID="$(echo "${CF_PRERELEASE_CREDENTIALS}" | jq -r '.access_key' | tr -d '\n')" \
    AWS_SECRET_ACCESS_KEY="$(echo "${CF_PRERELEASE_CREDENTIALS}" | jq -r '.secret_key' | tr -d '\n')" \
    AWS_REGION="$(echo "${CF_PRERELEASE_CREDENTIALS}" | jq -r '.region' | tr -d '\n')" \
    AWS_SESSION_TOKEN="$(echo "${CF_PRERELEASE_CREDENTIALS}" | jq -r '.session_token' | tr -d '\n')" \
    aws s3 cp --recursive "s3://${SOURCE_R2_BUCKET}/${VERSION}/" "${WORK_DIR}/" --endpoint-url "${ENDPOINT}"

ENDPOINT="$(echo "${CF_CREDENTIALS}" | jq -r '.endpoint' | tr -d '\n')"
AWS_ACCESS_KEY_ID="$(echo "${CF_CREDENTIALS}" | jq -r '.access_key' | tr -d '\n')" \
    AWS_SECRET_ACCESS_KEY="$(echo "${CF_CREDENTIALS}" | jq -r '.secret_key' | tr -d '\n')" \
    AWS_REGION="$(echo "${CF_CREDENTIALS}" | jq -r '.region' | tr -d '\n')" \
    AWS_SESSION_TOKEN="$(echo "${CF_CREDENTIALS}" | jq -r '.session_token' | tr -d '\n')" \
    go run main.go publish --release "${WORK_DIR}" \
    --cosignkey "${COSIGN_KEY:-}" \
    --gcsbucket "${GCS_BUCKET}" \
    --s3bucket "${R2_BUCKET}" \
    --helmbucket "${HELM_BUCKET}" \
    --s3helmbucket "${R2_HELM_BUCKET}" \
    --dockerhub "${DOCKER_HUB}" --dockertags "${VERSION}" \
    --github "${GITHUB_ORG}" --githubtoken "${GITHUB_TOKEN_FILE}" \
    --grafanatoken "${GRAFANA_TOKEN_FILE}" \
    --s3-base-endpoint "${ENDPOINT}"

# Also push images to a GCR repo, in case of dockerhub rate limiting issues for
# large clusters (see https://docs.docker.com/docker-hub/download-rate-limit/).
# Docker hub doesn't support Helm registries, so we also push these only to GCR.
go run main.go publish --release "${WORK_DIR}" \
    --cosignkey "${COSIGN_KEY:-}" \
    --helmhub "${HELM_HUB_RELEASE}" \
    --dockerhub "${DOCKER_HUB_MIRROR}" \
    --dockertags "${VERSION}"
