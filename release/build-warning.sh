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

set -eu

PRERELEASE_DOCKER_HUB=${PRERELEASE_DOCKER_HUB:-gcr.io/istio-prerelease-testing}
GCS_BUCKET=${GCS_BUCKET:-istio-prerelease/prerelease}

VERSION="$(cat "${WD}/trigger-build")"

cat <<EOF
WARNING
If you are seeing this test, you have modified the trigger-build file.

This will trigger a official build of Istio once merged.

If this is unexpected, do not merge this PR.

If this is expected, this message can be ignored.

Build information
=================
Version: ${VERSION}
Staging GCS Bucket: ${GCS_BUCKET}
Staging Docker Hub: ${PRERELEASE_DOCKER_HUB}
EOF

exit 1
