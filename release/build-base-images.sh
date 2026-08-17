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

GITHUB_ORG=${GITHUB_ORG:-istio}

WORK_DIR="$(mktemp -d)/build"
mkdir -p "${WORK_DIR}"
BRANCH="master"
if [[ "${VERSION}" != "master" ]]; then
  BRANCH="release-${VERSION}"
fi
MANIFEST=$(cat <<EOF
version: "${VERSION}"
directory: "${WORK_DIR}"
dependencies:
  istio:
    git: https://github.com/${GITHUB_ORG}/istio
    branch: "${BRANCH}"
EOF
)
go run main.go build \
  --manifest <(echo "${MANIFEST}") \
  --githubtoken "${GITHUB_TOKEN_FILE:-}" \
  --build-base-images
