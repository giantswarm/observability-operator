###############################################################################
# Go Formatting and Code Quality
###############################################################################

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

###############################################################################
# Testing & Coverage
###############################################################################

# Default Kubernetes version for envtest
ENVTEST_K8S_VERSION ?= 1.33.0

generate-golden-files: ## Generate golden files for tests
	@echo "Generating golden files"
	@UPDATE_GOLDEN_FILES=true go test -v ./...

.PHONY: setup-envtest
setup-envtest: envtest ## Download and setup Kubernetes test binaries for the specified version
	@echo "Setting up envtest binaries for Kubernetes $(ENVTEST_K8S_VERSION)"
	$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir ./bin/k8s

.PHONY: test-unit
test-unit: ginkgo fmt vet ## Run unit tests only - pure Go logic (./pkg/... and ./internal/predicates, no envtest required)
	@echo "Running unit tests"
	$(GINKGO) -p --nodes 4 -randomize-all --randomize-suites --cover --coverprofile=coverage-unit.out --coverpkg=$$(go list ./pkg/... ./internal/predicates | tr '\n' ',') ./pkg/... ./internal/predicates

.PHONY: test-integration
test-integration: ginkgo fmt vet envtest ## Run integration tests with envtest
	@echo "Running integration tests with envtest"
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GINKGO) -p --nodes 4 -randomize-all --randomize-suites --cover --coverprofile=coverage-integration.out --coverpkg=$$(go list ./internal/controller ./internal/webhook/... | tr '\n' ',') ./internal/controller ./internal/webhook/...

.PHONY: test-all
test-all: test-unit test-integration ## Run all tests (unit + integration) and merge coverage.
	@echo "Merging coverage reports"
	@echo "mode: set" > coverprofile.out
	@cat coverage-unit.out coverage-integration.out | grep -v "^mode: " >> coverprofile.out
	@rm coverage-unit.out coverage-integration.out
	@echo "Combined coverage profile created: coverprofile.out"

.PHONY: test-controllers
test-controllers: ginkgo fmt vet envtest ## Run only controller integration tests
	@echo "Running controller integration tests with envtest"
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GINKGO) -v ./internal/controller

.PHONY: test-webhooks
test-webhooks: ginkgo fmt vet envtest ## Run only webhook integration tests
	@echo "Running webhook integration tests with envtest"
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GINKGO) -v ./internal/webhook/...

.PHONY: coverage-html
coverage-html: test-all ## Generate HTML coverage report from merged profile.
	go tool cover -html coverprofile.out

# Define the location of the envtest setup script.
ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

# Define the location of the ginkgo testing binary.
GINKGO = $(shell pwd)/bin/ginkgo
.PHONY: ginkgo
ginkgo: ## Download ginkgo locally if necessary.
	$(call go-get-tool,$(GINKGO),github.com/onsi/ginkgo/v2/ginkgo@latest)

###############################################################################
# Linting and Validation
###############################################################################

.PHONY: validate-alertmanager-config
validate-alertmanager-config: ## Validate Alertmanager config.
	./hack/bin/validate-alertmanager-config.sh

###############################################################################
# Local Development
###############################################################################

.PHONY: run-local
run-local: ## Run the application in local mode.
	./hack/bin/run-local.sh

###############################################################################
# Helper Functions
###############################################################################

# PROJECT_DIR is the directory of this Makefile.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

# go-get-tool will 'go install' any package $2 and install it to $1.
# $1: Target binary path.
# $2: Go package path with version.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
