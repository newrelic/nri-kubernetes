OSFLAG := $(shell uname -s | tr A-Z a-z)
OSFLAG := $(OSFLAG)_amd64
BIN_DIR = ./bin
TEST_COVERAGE_DIR := $(BIN_DIR)/test-coverage
BINARY_NAME = nri-kubernetes
E2E_BINARY_NAME := $(BINARY_NAME)-e2e
GOFLAGS = -mod=readonly

# GOOS and GOARCH will likely come from env
GOOS ?=
GOARCH ?=
CGO_ENABLED ?= 0

BUILD_DATE := $(shell date)
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
	@echo "[validate] Validating source code running semgrep... "
	@[ -f .semgrep.yml ] && semgrep_config=".semgrep.yml" || semgrep_config="p/golang" ; \
	docker run --rm -v "${PWD}:/src:ro" --workdir /src returntocorp/semgrep semgrep -c "$$semgrep_config"

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
	@mkdir -p $(TEST_COVERAGE_DIR)
	go test ./... -count=1 -coverprofile=$(TEST_COVERAGE_DIR)/coverage.out -covermode=count

buildLicenseNotice:
	@go list -mod=mod -m -json all | go-licence-detector -noticeOut=NOTICE.txt -rules ./assets/licence/rules.json  -noticeTemplate ./assets/licence/THIRD_PARTY_NOTICES.md.tmpl -noticeOut THIRD_PARTY_NOTICES.md -overrides ./assets/licence/overrides -includeIndirect

.PHONY: run-static
run-static:
	@go run cmd/kubernetes-static/main.go

.PHONY: local-env-start
local-env-start:
	minikube start
	helm repo add newrelic https://helm-charts.newrelic.com
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm dependency update ./charts/newrelic-infrastructure
	helm dependency update ./charts/internal/e2e-resources
	$(MAKE) tilt-up

.PHONY: tilt-up
tilt-up:
	eval $$(minikube docker-env); tilt up ; tilt down

# rt-update-changelog runs the release-toolkit run.sh script by piping it into bash to update the CHANGELOG.md.
# It also passes down to the script all the flags added to the make target. To check all the accepted flags,
# see: https://github.com/newrelic/release-toolkit/blob/main/contrib/ohi-release-notes/run.sh
#  e.g. `make rt-update-changelog -- -v`
.PHONY: rt-update-changelog
rt-update-changelog:
	curl "https://raw.githubusercontent.com/newrelic/release-toolkit/v1/contrib/ohi-release-notes/run.sh" | bash -s -- $(filter-out $@,$(MAKECMDGOALS))
