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

set -eux

DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}

REPO_ORG=${REPO_ORG:-istio}

DRY_RUN=${DRY_RUN:=true}
VERSION=${VERSION:-"$(< "${WD}/trigger-branch" grep VERSION= | cut -d'=' -f2)"}
STEP=${STEP:-"$(< "${WD}/trigger-branch" grep STEP= | cut -d'=' -f2)"}
WORK_DIR="$(mktemp -d)/branch"
mkdir -p "${WORK_DIR}"

# STEP 1 and 2 use the master branch, while the remaining steps use the release branch
if [[ "${STEP}" == "1" || "${STEP}" == "2" ]]; then
  base_branch=master
else
  base_branch=release-${VERSION}
fi
BASE_BRANCH=${BASE_BRANCH:-${base_branch}}

MANIFEST=$(cat <<EOF
version: "${VERSION}"
docker: "${DOCKER_HUB}"
directory: "${WORK_DIR}"
dependencies:
${DEPENDENCIES:-$(cat <<EOD
  istio:
    git: https://github.com/${REPO_ORG}/istio
    branch: ${BASE_BRANCH}
  api:
    git: https://github.com/${REPO_ORG}/api
    branch: ${BASE_BRANCH}
  client-go:
    git: https://github.com/${REPO_ORG}/client-go
    branch: ${BASE_BRANCH}
  common-files:
    git: https://github.com/${REPO_ORG}/common-files
    branch: ${BASE_BRANCH}
  envoy:
    git: https://github.com/${REPO_ORG}/envoy
    branch: ${BASE_BRANCH}
  gogo-genproto:
    git: https://github.com/${REPO_ORG}/gogo-genproto
    branch: ${BASE_BRANCH}
  pkg:
    git: https://github.com/${REPO_ORG}/pkg
    branch: ${BASE_BRANCH}
  proxy:
    git: https://github.com/${REPO_ORG}/proxy
    branch: ${BASE_BRANCH}
  release-builder:
    git: https://github.com/${REPO_ORG}/release-builder
    branch: ${BASE_BRANCH}
  test-infra:
    git: https://github.com/${REPO_ORG}/test-infra
    branch: master
  tools:
    git: https://github.com/${REPO_ORG}/tools
    branch: ${BASE_BRANCH}
EOD
)}
EOF
)

# "Temporary" hacks
export PATH=${GOPATH}/bin:${PATH}

go run main.go branch --manifest <(echo "${MANIFEST}") --step="${STEP}" --dryrun="${DRY_RUN}"
