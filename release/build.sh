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

if [[ -n ${ISTIO_ENVOY_BASE_URL:-} ]]; then
  PROXY_OVERRIDE="proxyOverride: ${ISTIO_ENVOY_BASE_URL}"
fi

# We shouldn't push here right now, this is just which version to embed in the Helm charts
DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}

GITHUB_TOKEN_FILE=${GITHUB_TOKEN_FILE:-}

VERSION="$(cat "${WD}/trigger-build")"

WORK_DIR="$(mktemp -d)/build"
mkdir -p "${WORK_DIR}"

MANIFEST=$(cat <<EOF
version: "${VERSION}"
docker: "${DOCKER_HUB}"
directory: "${WORK_DIR}"
dependencies:
${DEPENDENCIES:-$(cat <<EOD
  istio:
    git: https://github.com/istio/istio
    branch: release-1.11
  api:
    git: https://github.com/istio/api
    auto: modules
  proxy:
    git: https://github.com/istio/proxy
    auto: deps
  pkg:
    git: https://github.com/istio/pkg
    auto: modules
  client-go:
    git: https://github.com/istio/client-go
    branch: release-1.11
    goversionenabled: true
  gogo-genproto:
    git: https://github.com/istio/gogo-genproto
    branch: release-1.11
  test-infra:
    git: https://github.com/istio/test-infra
    branch: master
  tools:
    git: https://github.com/istio/tools
    branch: release-1.11
  envoy:
    git: https://github.com/istio/envoy
    auto: proxy_workspace
EOD
)}
dashboards:
  istio-extension-dashboard: 13277
  istio-mesh-dashboard: 7639
  istio-performance-dashboard: 11829
  istio-service-dashboard: 7636
  istio-workload-dashboard: 7630
  pilot-dashboard: 7645
${PROXY_OVERRIDE:-}
EOF
)

# "Temporary" hacks
export PATH=${GOPATH}/bin:${PATH}

go run main.go build --manifest <(echo "${MANIFEST}") --githubtoken "${GITHUB_TOKEN_FILE}"
go run main.go validate --release "${WORK_DIR}/out"
go run main.go publish --release "${WORK_DIR}/out" --gcsbucket "${GCS_BUCKET}" --dockerhub "${PRERELEASE_DOCKER_HUB}" --dockertags "${VERSION}"
