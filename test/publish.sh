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

if [[ $(command -v gcloud) ]]; then
  gcloud auth configure-docker -q
elif [[ $(command -v docker-credential-gcr) ]]; then
  docker-credential-gcr configure-docker
else
  echo "No credential helpers found, push to docker may not function properly"
fi

GCS_BUCKET=${GCS_BUCKET:-istio-prerelease/prerelease}
HELM_BUCKET=${HELM_BUCKET:-istio-prerelease/charts}
COSIGN_KEY=${COSIGN_KEY:-}
GITHUB_ORG=${GITHUB_ORG:-istio}
DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}

if [[ -n ${ISTIO_ENVOY_BASE_URL:-} ]]; then
  PROXY_OVERRIDE="proxyOverride: ${ISTIO_ENVOY_BASE_URL}"
fi

VERSION="1.14.0-releasebuilder.$(git rev-parse --short HEAD)"

WORK_DIR="$(mktemp -d)/build"
mkdir -p "${WORK_DIR}"

MANIFEST=$(cat <<EOF
version: "${VERSION}"
docker: "${DOCKER_HUB}"
directory: "${WORK_DIR}"
architectures: [linux/amd64, linux/arm64]
dependencies:
${DEPENDENCIES:-$(cat <<EOD
  istio:
    git: https://github.com/${GITHUB_ORG}/istio
    branch: release-1.16
  api:
    git: https://github.com/${GITHUB_ORG}/api
    auto: modules
  proxy:
    git: https://github.com/${GITHUB_ORG}/proxy
    auto: deps
  pkg:
    git: https://github.com/${GITHUB_ORG}/pkg
    auto: modules
  client-go:
    git: https://github.com/${GITHUB_ORG}/client-go
    branch: release-1.16
    goversionenabled: true
  test-infra:
    git: https://github.com/${GITHUB_ORG}/test-infra
    branch: master
  tools:
    git: https://github.com/${GITHUB_ORG}/tools
    branch: release-1.16
  envoy:
    git: https://github.com/envoyproxy/envoy
    auto: proxy_workspace
  release-builder:
    git: https://github.com/${GITHUB_ORG}/release-builder
    branch: release-1.16
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

go run main.go build --manifest <(echo "${MANIFEST}")

go run main.go validate --release "${WORK_DIR}/out"

if [[ -z "${DRY_RUN:-}" ]]; then
go run main.go publish --release "${WORK_DIR}/out" \
  --cosignkey "${COSIGN_KEY:-}" \
  --helmbucket "${HELM_BUCKET}" \
  --helmhub "${DOCKER_HUB}/charts" \
  --gcsbucket "${GCS_BUCKET}" \
  --dockerhub "${DOCKER_HUB}" \
  --dockertags "${VERSION}"

fi
