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

gcloud auth activate-service-account --key-file="${GOOGLE_APPLICATION_CREDENTIALS}"

# Temporary hack to get around some gcloud credential issues
mkdir ~/.docker
cp "${DOCKER_CONFIG}/config.json" ~/.docker/
export DOCKER_CONFIG=~/.docker
gcloud auth configure-docker -q

PRERELEASE_DOCKER_HUB=${PRERELEASE_DOCKER_HUB:-gcr.io/istio-prerelease-testing}
GCS_BUCKET=${GCS_BUCKET:-istio-prerelease/prerelease}

# We shouldn't push here right now, this is just which version to embed in the Helm charts
DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}

VERSION="$(cat "${WD}/trigger-build")"

WORK_DIR="$(mktemp -d)/build"
mkdir -p "${WORK_DIR}"

MANIFEST=$(cat <<EOF
version: ${VERSION}
docker: ${DOCKER_HUB}
directory: ${WORK_DIR}
dependencies:
  istio:
    git: https://github.com/istio/istio
    branch: release-1.4
  cni:
    git: https://github.com/istio/cni
    auto: deps
  operator:
    git: https://github.com/istio/operator
    auto: modules
EOF
)

# "Temporary" hacks
export PATH=${GOPATH}/bin:${PATH}

go run main.go build --manifest <(echo "${MANIFEST}")
go test ./test/... --release "${WORK_DIR}/out" -v
go run main.go publish --release "${WORK_DIR}/out" --gcsbucket "${GCS_BUCKET}" --dockerhub "${PRERELEASE_DOCKER_HUB}" --dockertags "${VERSION}"
