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

function cleanup() {
  # shellcheck disable=SC2046
  docker stop $(docker ps -a -q --filter label=istio-release-builder)
}
trap cleanup EXIT

# Setup local registry and S3-compatible storage.
docker run -d  --rm  \
  -p "7480:5000" --label istio-release-builder \
  --name "release-builder-registry" \
  registry.istio.io/testing/registry:2
docker run -d  --rm  \
  -p "7481:9000" --label istio-release-builder \
  --name "release-builder-s3" \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -e MINIO_DOMAIN=localhost \
  quay.io/minio/minio:RELEASE.2025-04-22T22-12-26Z \
  server /data --address ":9000"

# Setup the local S3 bucket. Add retries while MinIO starts.
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
export AWS_REGION=us-east-1
S3_ENDPOINT=${S3_ENDPOINT:-http://localhost:7481}
counter=0
while : ; do
  [[ "$counter" == 10 ]] && exit 1
  aws s3api create-bucket --bucket istio-build --endpoint-url "${S3_ENDPOINT}" && break
   sleep 1
   echo "Trying again... Try #$counter"
   counter=$((counter+1))
done

DOCKER_HUB=${DOCKER_HUB:-"localhost:7480"}
S3_BUCKET=${S3_BUCKET:-istio-build/test}
S3_HELM_BUCKET=${S3_HELM_BUCKET:-istio-build/test/charts}
S3_HELM_URL=${S3_HELM_URL:-${S3_ENDPOINT/localhost/istio-build.localhost}/test/charts}
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
    branch: master
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
    auto: modules
    goversionenabled: true
  test-infra:
    git: https://github.com/${GITHUB_ORG}/test-infra
    branch: master
  tools:
    git: https://github.com/${GITHUB_ORG}/tools
    branch: master
  envoy:
    git: https://github.com/envoyproxy/envoy
    auto: proxy_workspace
  release-builder:
    git: https://github.com/${GITHUB_ORG}/release-builder
    branch: master
EOD
)}
dashboards:
  istio-extension-dashboard: 13277
  istio-mesh-dashboard: 7639
  istio-performance-dashboard: 11829
  istio-service-dashboard: 7636
  istio-workload-dashboard: 7630
  pilot-dashboard: 7645
  ztunnel-dashboard: 0
${PROXY_OVERRIDE:-}
EOF
)

go run main.go build --manifest <(echo "${MANIFEST}")

go run main.go validate --release "${WORK_DIR}/out"

if [[ -z "${DRY_RUN:-}" ]]; then
go run main.go publish --release "${WORK_DIR}/out" \
  --cosignkey "${COSIGN_KEY:-}" \
  --helmhub "${DOCKER_HUB}/charts" \
  --s3bucket "${S3_BUCKET}" \
  --s3helmbucket "${S3_HELM_BUCKET}" \
  --s3helmurl "${S3_HELM_URL}" \
  --s3-base-endpoint "${S3_ENDPOINT}" \
  --dockerhub "${DOCKER_HUB}" \
  --dockertags "${VERSION}"
fi
