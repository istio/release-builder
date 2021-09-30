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

set -x

MANIFEST=$(cat <<EOF
version: "${VERSION}"
docker: "${DOCKER_HUB}"
directory: "${WORK_DIR}"
dependencies:
${DEPENDENCIES:-$(cat <<EOD
  istio:
    git: https://github.com/istio/istio
    branch: master
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
    branch: master
    goversionenabled: true
  gogo-genproto:
    git: https://github.com/istio/gogo-genproto
    branch: master
  test-infra:
    git: https://github.com/istio/test-infra
    branch: master
  tools:
    git: https://github.com/istio/tools
    branch: master
  envoy:
    git: https://github.com/envoyproxy/envoy
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

if [ $BUILD_BASE_IMAGES = true ] ; then
  MANIFEST=$(cat <<EOF
version: "1.8.0-test"
docker: "docker.io/istio"
directory: "${WORK_DIR}"
dependencies:
  istio:
    git: https://github.com/istio/istio
    branch: master
EOF
)
  go run main.go build \
    --manifest <(echo "${MANIFEST}") \
    --githubtoken "${GITHUB_TOKEN_FILE}" \
    --build-base-images
  exit 1
fi

go run main.go build \
  --manifest <(echo "${MANIFEST}") \
  --githubtoken "${GITHUB_TOKEN_FILE}"

go run main.go validate --release "${WORK_DIR}/out"

go run main.go publish --release "${WORK_DIR}/out" \
  --cosignkey "${COSIGN_KEY:-}" \
  --gcsbucket "${GCS_BUCKET}" \
  --helmbucket "${HELM_BUCKET}" \
  --dockerhub "${PRERELEASE_DOCKER_HUB}" \
  --dockertags "${VERSION}"
