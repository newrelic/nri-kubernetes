OSFLAG := $(shell uname -s | tr A-Z a-z)
OSFLAG := $(OSFLAG)_amd64
BIN_DIR = ./bin
TOOLS_DIR := $(BIN_DIR)/dev-tools
BINARY_NAME = nri-kubernetes
E2E_BINARY_NAME := $(BINARY_NAME)-e2e
GOFLAGS = -mod=readonly
GOLANGCI_LINT = github.com/golangci/golangci-lint/cmd/golangci-lint

# GOOS and GOARCH will likely come from env
GOOS ?=
GOARCH ?=
CGO_ENABLED ?= 0

BUILD_DATE := $(shell date)
COMMIT := $(shell git rev-parse HEAD)
TAG ?= dev
COMMIT ?= $(shell git rev-parse HEAD || echo "unknown")

LDFLAGS ?= -ldflags="-X 'main.integrationVersion=$(TAG)' -X 'main.gitCommit=$(COMMIT)' -X 'main.buildDate=$(BUILD_DATE)' "

ifneq ($(strip $(GOOS)), )
BINARY_NAME := $(BINARY_NAME)-$(GOOS)
endif

ifneq ($(strip $(GOARCH)), )
BINARY_NAME := $(BINARY_NAME)-$(GOARCH)
endif

.PHONY: all
all: build

.PHONY: build
build: clean validate test compile

.PHONY: clean
clean:
	@echo "[clean] Removing integration binaries"
	@rm -rf $(BIN_DIR)/$(BINARY_NAME) $(BIN_DIR)/$(E2E_BINARY_NAME)

.PHONY: validate

validate:
	@echo "[validate] Validating source code running golangci-lint & semgrep... "
	go run -modfile tools/go.mod $(GOFLAGS) $(GOLANGCI_LINT) run --verbose
	@[ -f .semgrep.yml ] && semgrep_config=".semgrep.yml" || semgrep_config="p/golang" ; \
	docker run --rm -v "${PWD}:/src:ro" --workdir /src returntocorp/semgrep -c "$$semgrep_config"

.PHONY: codespell
codespell: CODESPELL_BIN := codespell
codespell: ## Runs spell checking.
	@which $(CODESPELL_BIN) >/dev/null 2>&1 || (echo "$(CODESPELL_BIN) binary not found, skipping spell checking"; exit 0)
	@$(CODESPELL_BIN)

.PHONY: compile
compile:
	@echo "[compile] Building $(BINARY_NAME)"
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/nri-kubernetes


.PHONY: compile-multiarch
compile-multiarch:
	$(MAKE) compile GOOS=linux GOARCH=amd64
	$(MAKE) compile GOOS=linux GOARCH=arm64
	$(MAKE) compile GOOS=linux GOARCH=arm

.PHONY: compile-dev
compile-dev:
	@echo "[compile-dev] Building $(BINARY_NAME) for development environment"
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./src

.PHONY: test
test:
	@echo "[test] Running unit tests"
	@go test ./...

buildLicenseNotice:
	@go list -mod=mod -m -json all | go-licence-detector -noticeOut=NOTICE.txt -rules ./assets/licence/rules.json  -noticeTemplate ./assets/licence/THIRD_PARTY_NOTICES.md.tmpl -noticeOut THIRD_PARTY_NOTICES.md -overrides ./assets/licence/overrides -includeIndirect

.PHONY: run-static
run-static:
	@go run cmd/kubernetes-static/main.go
