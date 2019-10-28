export GO111MODULE ?= on
export GOPROXY ?= https://proxy.golang.org

include common/Makefile.common.mk

lint: lint-all

install:
	go install ./...

test:
	go test -race ./...

gen: tidy-go mirror-licenses

gen-check: gen check-clean-repo

format: format-go tidy-go
