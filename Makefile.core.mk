export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org

include common/Makefile.common.mk

lint: lint-all

test:
	go test -race ./...

format: format-go