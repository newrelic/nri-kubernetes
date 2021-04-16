OSFLAG := $(shell uname -s | tr A-Z a-z)
OSFLAG := $(OSFLAG)_amd64
BIN_DIR = ./bin
TOOLS_DIR := $(BIN_DIR)/dev-tools
BINARY_NAME = nri-kubernetes
E2E_BINARY_NAME := $(BINARY_NAME)-e2e

# GOOS and GOARCH will likely come from env
GOOS ?=
GOARCH ?=
CGO_ENABLED ?= 0

ifneq ($(strip $(GOOS)), )
BINARY_NAME := $(BINARY_NAME)-$(GOOS)
endif

ifneq ($(strip $(GOARCH)), )
BINARY_NAME := $(BINARY_NAME)-$(GOARCH)
endif

GOLANGCILINT_VERSION = 1.36.0

.PHONY: all
all: build

.PHONY: build
build: clean lint  test compile

.PHONY: clean
clean:
	@echo "[clean] Removing integration binaries"
	@rm -rf $(BIN_DIR)/$(BINARY_NAME) $(BIN_DIR)/$(E2E_BINARY_NAME)

$(TOOLS_DIR):
	@mkdir -p $@

$(TOOLS_DIR)/golangci-lint: $(TOOLS_DIR)
	@echo "[tools] Downloading 'golangci-lint'"
	@wget -O - -q https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | BINDIR=$(@D) sh -s v$(GOLANGCILINT_VERSION) &> /dev/null

.PHONY: lint
lint: $(TOOLS_DIR)/golangci-lint
	@echo "[validate] Validating source code running golangci-lint"
	@$(TOOLS_DIR)/golangci-lint run

.PHONY: lint-all
lint-all: $(TOOLS_DIR)/golangci-lint
	@echo "[validate] Validating source code running golangci-lint"
	@$(TOOLS_DIR)/golangci-lint run

.PHONY: compile
compile:
	@echo "[compile] Building $(BINARY_NAME)"
	CGO_ENABLED=$(CGO_ENABLED) go build -o $(BIN_DIR)/$(BINARY_NAME) ./src

.PHONY: compile-dev
compile-dev:
	@echo "[compile-dev] Building $(BINARY_NAME) for development environment"
	@GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME) ./src

.PHONY: deploy-dev
deploy-dev: compile-dev
	@echo "[deploy-dev] Deploying dev container image containing $(BINARY_NAME) in Kubernetes"
	@skaffold run

.PHONY: deploy-dev-openshift
deploy-dev-openshift: compile-dev
	@echo "[deploy-dev-openshift] Deploying dev container image containing $(BINARY_NAME) in Openshift"
	@skaffold -v debug run -p openshift

.PHONY: test
test:
	@echo "[test] Running unit tests"
	@go test ./...

guard-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* not set"; \
		exit 1; \
	fi

buildLicenseNotice:
	@go list -mod=mod -m -json all | go-licence-detector -noticeOut=NOTICE.txt -rules ./assets/licence/rules.json  -noticeTemplate ./assets/licence/THIRD_PARTY_NOTICES.md.tmpl -noticeOut THIRD_PARTY_NOTICES.md -overrides ./assets/licence/overrides -includeIndirect

.PHONY: e2e
e2e: guard-CLUSTER_NAME guard-NR_LICENSE_KEY
	@go run e2e/cmd/e2e.go --verbose

.PHONY: e2e-compile
e2e-compile:
	@echo "[compile E2E binary] Building $(E2E_BINARY_NAME)"
	# CGO_ENABLED=0 is needed since the binary is compiled in a non alpine linux.
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(E2E_BINARY_NAME) ./e2e/cmd/e2e.go

.PHONY: e2e-compile-only
e2e-compile-only:
	@echo "[compile E2E binary] Building $(E2E_BINARY_NAME)"
	# CGO_ENABLED=0 is needed since the binary is compiled in a non alpine linux.
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_DIR)/$(E2E_BINARY_NAME) ./e2e/cmd/e2e.go

.PHONY: run-static
run-static:
	@go run cmd/kubernetes-static/main.go cmd/kubernetes-static/basic_http_client.go
