export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org

include common/Makefile.common.mk

lint: lint-dockerfiles lint-scripts lint-yaml lint-go lint-markdown

test:
	go test ./...

format: format-go