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

PRERELEASE_DOCKER_HUB=${PRERELEASE_DOCKER_HUB:-gcr.io/istio-prerelease-testing}
GCS_BUCKET=${GCS_BUCKET:-istio-prerelease/prerelease}
HELM_BUCKET=${HELM_BUCKET:-istio-prerelease/charts}
COSIGN_KEY=${COSIGN_KEY:-}
GITHUB_ORG=${GITHUB_ORG:-istio}
ARCH=${ARCH:-linux/amd64,linux/arm64}
ARCHS=$(echo "[$ARCH]" | sed 's/, */, /g')

if [[ -n ${ISTIO_ENVOY_BASE_URL:-} ]]; then
  PROXY_OVERRIDE="proxyOverride: ${ISTIO_ENVOY_BASE_URL}"
fi

# We shouldn't push here right now, this is just which version to embed in the Helm charts
DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}

# When set, we skip the actual build, scan base images, and create and push new ones if needed.
BUILD_BASE_IMAGES=${BUILD_BASE_IMAGES:=false}

VERSION=${VERSION:-$(cat "${WD}/trigger-build")}

WORK_DIR="$(mktemp -d)/build"
mkdir -p "${WORK_DIR}"

MANIFEST=$(cat <<EOF
version: "${VERSION}"
docker: "${DOCKER_HUB}"
directory: "${WORK_DIR}"
architectures: ${ARCHS}
dependencies:
${DEPENDENCIES:-$(cat <<EOD
  istio:
    git: https://github.com/${GITHUB_ORG}/istio
    branch: release-1.19
  api:
    git: https://github.com/${GITHUB_ORG}/api
    auto: modules
    goversionenabled: true
  proxy:
    git: https://github.com/${GITHUB_ORG}/proxy
    auto: deps
  ztunnel:
    git: https://github.com/${GITHUB_ORG}/ztunnel
    auto: deps
  client-go:
    git: https://github.com/${GITHUB_ORG}/client-go
    branch: release-1.19
    goversionenabled: true
  test-infra:
    git: https://github.com/${GITHUB_ORG}/test-infra
    branch: master
  tools:
    git: https://github.com/${GITHUB_ORG}/tools
    branch: release-1.19
  envoy:
    git: https://github.com/envoyproxy/envoy
    auto: proxy_workspace
  release-builder:
    git: https://github.com/${GITHUB_ORG}/release-builder
    branch: release-1.19
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

go run main.go build --manifest <(echo "${MANIFEST}")

go run main.go validate --release "${WORK_DIR}/out"

go run main.go publish --release "${WORK_DIR}/out" \
  --cosignkey "${COSIGN_KEY:-}" \
  --gcsbucket "${GCS_BUCKET}" \
  --helmbucket "${HELM_BUCKET}" \
  --helmhub "${PRERELEASE_DOCKER_HUB}/charts" \
  --dockerhub "${PRERELEASE_DOCKER_HUB}" \
  --dockertags "${VERSION}"
