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

.PHONY: test-unit
test-unit: ginkgo generate fmt vet envtest ## Run unit tests
	# Set up environment for Kubernetes and run Ginkgo concurrently.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GINKGO) -p --nodes 4 -r -randomize-all --randomize-suites --skip-package=tests --cover --coverpkg=`go list ./... | grep -v fakes | tr '\n' ','` ./...

.PHONY: test-all
test-all: test-unit ## Run all tests by default (currently only unit tests).

.PHONY: coverage-html
coverage-html: test-unit ## Generate HTML coverage report.
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

# go-get-tool fetches and installs a Go tool if it is not present.
# $1: Target binary path.
# $2: Go package path with version.
define go-get-tool
@[ -f $(1) ] || { \
	set -e ;\
	TMP_DIR=$$(mktemp -d) ;\
	cd $$TMP_DIR ;\
	echo "Downloading $(2)" ;\
	GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
	rm -rf $$TMP_DIR ;\
}
endef
