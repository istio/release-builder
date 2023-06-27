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

set -eux

if [[ -n "${DOCKER_CONFIG_DATA:-}" ]]; then
  # Custom docker config as inline environment variable
  mkdir ~/.docker
  set +x
  echo "${DOCKER_CONFIG_DATA}" > ~/.docker/config.json
  set -x
  export DOCKER_CONFIG=~/.docker
  gcloud auth configure-docker -q
elif [[ -n "${DOCKER_CONFIG:-}" ]]; then
  # If DOCKER_CONFIG is set, we are mounting a known docker config.
  # we will want to merge in gcloud options, so we can push to GCR *and* the other (docker hub) credentials.
  # However, DOCKER_CONFIG is a read only mount. So we copy it to somewhere writeable then merge in the GCR creds
  mkdir ~/.docker
  cp "${DOCKER_CONFIG}/config.json" ~/.docker/
  export DOCKER_CONFIG=~/.docker
  gcloud auth configure-docker -q
fi
# No else needed - the prow entrypoint already runs configure-docker for standard cases

GITHUB_ORG=${GITHUB_ORG:-istio}

WORK_DIR="$(mktemp -d)/build"
mkdir -p "${WORK_DIR}"

MANIFEST=$(cat <<EOF
version: "${VERSION}"
directory: "${WORK_DIR}"
dependencies:
  istio:
    git: https://github.com/${GITHUB_ORG}/istio
    branch: master
EOF
)
go run main.go build \
  --manifest <(echo "${MANIFEST}") \
  --githubtoken "${GITHUB_TOKEN_FILE:-}" \
  --build-base-images
