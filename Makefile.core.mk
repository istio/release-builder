export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org

include common/Makefile.common.mk

MARKDOWN_LINT_ALLOWLIST = https://storage.googleapis.com/istio-build/proxy

lint: lint-all

install:
	go install ./...

test:
	go test -race ./...

gen: mirror-licenses format

gen-check: gen check-clean-repo

format: format-go tidy-go
