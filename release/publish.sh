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

VERSION="$(cat "${WD}/trigger-publish")"

SOURCE_GCS_BUCKET=${SOURCE_GCS_BUCKET:-istio-prerelease/prerelease}
GCS_BUCKET=${GCS_BUCKET:-istio-release/releases}
DOCKER_HUB=${DOCKER_HUB:-docker.io/istio}
GITHUB_ORG=${GITHUB_ORG:-istio}
GITHUB_TOKEN_FILE=${GITHUB_TOKEN_FILE:-}
GRAFANA_TOKEN_FILE=${GRAFANA_TOKEN_FILE:-}

WORK_DIR="$(mktemp -d)/release"
mkdir -p "${WORK_DIR}"

# "Temporary" hacks
export PATH=${GOPATH}/bin:${PATH}

gsutil -m cp -r "gs://${SOURCE_GCS_BUCKET}/${VERSION}/*" "${WORK_DIR}"
go run main.go publish --release "${WORK_DIR}" \
    --gcsbucket "${GCS_BUCKET}" \
    --dockerhub "${DOCKER_HUB}" --dockertags "${VERSION}" \
    --github "${GITHUB_ORG}" --githubtoken "${GITHUB_TOKEN_FILE}" \
    --grafanatoken "${GRAFANA_TOKEN_FILE}"
