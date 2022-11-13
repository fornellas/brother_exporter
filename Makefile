GO ?= go
GOIMPORTS_VERSION ?= latest
GOIMPORTS ?= goimports
GOIMPORTS_LOCAL ?= github.com/fornellas/brother_expporter
GOLANGCI_LINT_VERSION ?= latest
GOLANGCI_LINT ?= golangci-lint
GO_TEST ?= $(GO) test
GO_TEST_FLAGS ?= -v -race -cover

GO_FILES := $(shell find . -name \*.go)
ifneq ($(.SHELLSTATUS),0)
	$(error Failed to set GO_FILES)
endif

##
## Help
##

.PHONY: help
help:

##
## Install Deps
##

.PHONY: install-deps-help
install-deps-help:
	@echo 'install-deps: install dependencies required by the build'
help: install-deps-help
.PHONY: install-deps
install-deps:
	$(GO) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

##
## Generate
##

.PHONY: generate-help
generate-help:
	@echo 'generate: run `go generate`'
help: generate-help
.PHONY: generate
generate:
	$(GO) generate

##
## Lint
##

# goimports

.PHONY: goimports-help
goimports-help:
	@echo 'goimports: goimports format all .go files'
help: goimports-help
.PHONY: goimports
goimports: generate
	$(GOIMPORTS) -w -local $(GOIMPORTS_LOCAL) $(GO_FILES)

# go mod tidy

.PHONY: go-mod-tidy-help
go-mod-tidy-help:
	@echo 'go-mod-tidy: runn `go mod tidy`'
help: go-mod-tidy-help
.PHONY: go-mod-tidy
go-mod-tidy: goimports
	$(GO) mod tidy

# golangci-lint

.PHONY: golangci-lint-help
golangci-lint-help:
	@echo 'golangci-lint: runs golangcli-lint'
help: golangci-lint-help
.PHONY: golangci-lint
golangci-lint: go-mod-tidy generate
	$(GOLANGCI_LINT) run

.PHONY: clean-golangci-lint
clean-golangci-lint:
	$(GOLANGCI_LINT) cache clean
clean: clean-golangci-lint

# lint

.PHONY: lint-help
lint-help:
	@echo 'lint: runs all linters'
help: lint-help
.PHONY: golangci-lint
lint: golangci-lint go-mod-tidy goimports

##
## Test
##

.PHONY: test-help
test-help:
	@echo 'test: runs all tests'
help: test-help
.PHONY: test
test: generate
	$(GO_TEST) ./... $(GO_TEST_FLAGS)

.PHONY: clean-test
clean-test:
	$(GO) clean -r -testcache
clean: clean-test

##
## Build
##

.PHONY: build-help
build-help:
	@echo 'build: go generate and build'
help: build-help
.PHONY: build
build: generate
	$(GO) build

.PHONY: clean-build
clean-build:
	$(GO) clean -r -cache -modcache
clean: clean-build

##
## Clean
##

.PHONY: clean
clean: 