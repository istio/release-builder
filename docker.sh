#!/usr/bin/env bash

docker run -t -i --sig-proxy=true -u "$(id -u)" --rm \
	-e GOOS="${GOOS}" \
	-e GOARCH="${GOARCH}" \
	-e GOBIN="${GOBIN}" \
	-e BUILD_WITH_CONTAINER="$1" \
	-v /etc/passwd:/etc/passwd:ro \
	-v $(readlink /etc/localtime):/etc/localtime:ro \
	-v /var/run/docker.sock:/var/run/docker.sock \
	--mount type=bind,source="$(pwd)",destination="/work" \
	--mount type=bind,source="${HOME}/istio_out/istio-release",destination="/targetout" \
	--mount type=volume,source=home,destination="/home" \
	-w /work gcr.io/istio-testing/build-tools:2019-09-25T19-39-04 "$@"