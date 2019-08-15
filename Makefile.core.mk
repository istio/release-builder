export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org

test:
	go test ./...
