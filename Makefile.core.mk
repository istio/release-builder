export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org

test:
	go test ./...

run:
	go run ./pkg/build/cmd

format:
	@scripts/run_gofmt.sh