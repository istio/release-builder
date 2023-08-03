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

function cleanup() {
  # shellcheck disable=SC2046
  docker stop $(docker ps -a -q --filter label=istio-release-builder)
}
trap cleanup EXIT

# Setup fake GCS and registry
docker run -d  --rm  \
  -p "7480:5000" --label istio-release-builder \
  --name "release-builder-registry" \
  gcr.io/istio-testing/registry:2
docker run -d  --rm  \
  -p "7481:7481" --label istio-release-builder \
  --name "release-builder-gcs" \
  gcr.io/istio-testing/fake-gcs-server:1.45.2 \
  -scheme http -port 7481

# Setup our bucket. Add retry since the registry may not be ready yet
counter=0
while : ; do
  [[ "$counter" == 10 ]] && exit 1
  curl -X POST \
    -d '{"name":"istio-build"}' \
    -H "content-type: application/json" \
    -H "accept: application/json" \
    'http://127.0.0.1:7481/storage/v1/b?alt=json&project=test&projection=full' && break
   sleep 1
   echo "Trying again... Try #$counter"
   counter=$((counter+1))
done

DOCKER_HUB=${DOCKER_HUB:-"localhost:7480"}
export GCS_HOST=${GCS_HOST-"http://localhost:7481"}
GCS_BUCKET=${GCS_BUCKET:-istio-build/test}
HELM_BUCKET=${HELM_BUCKET:-istio-build/test/charts}
VERSION="1.19.0-releasebuilder.$(git rev-parse --short HEAD)"
COSIGN_KEY=${COSIGN_KEY:-}
GITHUB_ORG=${GITHUB_ORG:-istio}
ARCH=${ARCH:-linux/amd64,linux/arm64}
ARCHS=$(echo "[$ARCH]" | sed 's/, */, /g')

if [[ -n ${ISTIO_ENVOY_BASE_URL:-} ]]; then
  PROXY_OVERRIDE="proxyOverride: ${ISTIO_ENVOY_BASE_URL}"
fi

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

if [[ -z "${DRY_RUN:-}" ]]; then
go run main.go publish --release "${WORK_DIR}/out" \
  --cosignkey "${COSIGN_KEY:-}" \
  --helmbucket "${HELM_BUCKET}" \
  --helmhub "${DOCKER_HUB}/charts" \
  --gcsbucket "${GCS_BUCKET}" \
  --dockerhub "${DOCKER_HUB}" \
  --dockertags "${VERSION}"
fi
