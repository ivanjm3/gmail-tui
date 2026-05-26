SHELL := /bin/sh

GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOCACHE ?= $(CURDIR)/.tmp/go-build
GOTMPDIR ?= $(CURDIR)/.tmp/go-tmp
COVERAGE_FILE ?= coverage.out
API_COVERAGE_FILE ?= coverage_api.out
COVERAGE_HTML ?= coverage.html

export GOCACHE
export GOTMPDIR

.PHONY: lint test build coverage integration-test clean prepare

prepare:
	mkdir -p "$(GOCACHE)" "$(GOTMPDIR)"

lint: prepare
	$(GOLANGCI_LINT) run

test: prepare
	$(GO) test -race -count=1 ./...

build: prepare
	$(GO) build ./...

coverage: prepare
	$(GO) test -coverprofile="$(COVERAGE_FILE)" ./...
	$(GO) test -coverprofile="$(API_COVERAGE_FILE)" ./api
	$(GO) tool cover -html="$(COVERAGE_FILE)" -o "$(COVERAGE_HTML)"
	@api_cov=`$(GO) tool cover -func="$(API_COVERAGE_FILE)" | awk '/total:/ {print $$3}' | tr -d '%'`; \
	awk "BEGIN { exit !($$api_cov >= 80) }" || { echo "api coverage below 80%: $$api_cov%"; exit 1; }

integration-test: prepare
	$(GO) test -tags integration ./api/...

clean:
	rm -rf "$(COVERAGE_FILE)" "$(API_COVERAGE_FILE)" "$(COVERAGE_HTML)" .tmp
