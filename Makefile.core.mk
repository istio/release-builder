export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org

include common/Makefile.common.mk

MARKDOWN_LINT_ALLOWLIST = https://storage.googleapis.com/istio-build/proxy

.PHONY: lint
lint: lint-all

.PHONY: install
install:
	go install ./...

.PHONY: test
test:
	go test -race ./...

.PHONY: gen
gen: mirror-licenses format

.PHONY: gen-check
gen-check: gen check-clean-repo

.PHONY: format
format: format-go tidy-go
