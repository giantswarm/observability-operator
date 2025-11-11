# Paths
CHART_NAME = observability-operator
CHART_DIR = helm/$(CHART_NAME)
ALERTMANAGER_SECRET_PATH = $(CHART_NAME)/templates/alertmanager/secret.yaml
# Put the config file inside the test source directory for easier debugging
ALERTMANAGER_TEST_CONFIG_FILE = tests/alertmanager-routes/$*/alertmanager-config.yaml
TESTS_WORKDIR = tests-workdir
CHART_TEST_OUTPUT_DIR = $(TESTS_WORKDIR)/chart-manifest-$*
CHART_TEST_VALUES_FILE = tests/alertmanager-routes/$*/chart-values.yaml

# Binaries
BIN_DIR = $(TESTS_WORKDIR)/bin
AMTOOL_BIN = $(BIN_DIR)/amtool
BATS_BIN = tests/bats/core/bin/bats
YQ_BIN = $(BIN_DIR)/yq
YQ_VERSION = v4.48.1

# If VERBOSE is set use verbose output for bats tests
BATS_ARGS =
VERBOSE ?= 0
ifeq ($(VERBOSE),1)
	BATS_ARGS += --verbose-run --show-output-of-passing-tests
endif

###############################################################################
# Testing & Coverage
###############################################################################

.PHONY: generate-golden-files
generate-golden-files: ## Generate golden files for tests
	$(call log_build,"Generating golden files")
	@UPDATE_GOLDEN_FILES=true go test -v ./...

.PHONY: coverage-html
coverage-html: test ## Generate HTML coverage report from merged profile
	$(call log_build,"Generating HTML coverage report")
	go tool cover -html coverprofile.out
	$(call log_info,"Coverage report generated - opened in browser")

.PHONY: manual-testing
manual-testing: ## Run manual end-to-end testing script
	$(call log_build,"Running manual e2e testing")
	@if [ -z "$(INSTALLATION)" ]; then \
		echo "Error: INSTALLATION parameter is required"; \
		echo "Usage: make manual-testing INSTALLATION=<installation-name>"; \
		exit 1; \
	fi
	@./hack/bin/manual-testing.sh $(INSTALLATION)

###############################################################################
# Alertmanager unit Tests
###############################################################################

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(AMTOOL_BIN): | $(BIN_DIR) ## Install amtool binary
	@echo "==> Installing amtool binary"
	git clone -q --filter=blob:none --no-checkout https://github.com/grafana/prometheus-alertmanager.git $(TESTS_WORKDIR)/prometheus-alertmanager
	grep "github.com/prometheus/alertmanager =>" go.mod | sed -n 's/.*-\([a-f0-9]\{12\}\)$$/\1/p' | xargs \
		git -C $(TESTS_WORKDIR)/prometheus-alertmanager checkout -q
	make -C $(TESTS_WORKDIR)/prometheus-alertmanager common-build PROMU_BINARIES=amtool
	mv $(TESTS_WORKDIR)/prometheus-alertmanager/amtool $@

$(BATS_BIN): ## Install BATS testing framework
	@echo "==> Installing bats testing framework"
	git submodule update --init --recursive

$(YQ_BIN): | $(BIN_DIR) ## Install yq binary
	@echo "==> Installing yq binary"
	wget -q https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_amd64 -O $@
	chmod +x $@

.PHONY: bin-dir-clean
bin-dir-clean:
	-rm -rf $(BIN_DIR)

chart-manifest-%: ## Generate Helm chart manifest
	@mkdir -p $(CHART_TEST_OUTPUT_DIR)
	@helm template $* $(CHART_DIR) --values $(CHART_TEST_VALUES_FILE) --output-dir $(CHART_TEST_OUTPUT_DIR) 1>/dev/null

tests-alertmanager-routes-%-alertmanager-config: chart-manifest-% $(YQ_BIN) ## Generate Alertmanager config file
	@mkdir -p $(ALERTMANAGER_TEST_DIR)/$(ALERTMANAGER_TEST_CONFIG_DIR)
	$(YQ_BIN) -0 '.data | keys[]' $(CHART_TEST_OUTPUT_DIR)/$(ALERTMANAGER_CHART_SECRET_PATH) | \
		xargs -0 -I{} $(YQ_BIN) -s '"$(ALERTMANAGER_TEST_DIR)/$(ALERTMANAGER_TEST_CONFIG_DIR)/{}"' '.data["{}"] | @base64d | split_doc' $(CHART_TEST_OUTPUT_DIR)/$(ALERTMANAGER_CHART_SECRET_PATH)

tests-alertmanager-routes-%: tests-alertmanager-routes-%-alertmanager-config $(AMTOOL_BIN) $(BATS_BIN) ## Run Alertmanager routes tests
	@echo "==> $@"
	@ALERTMANAGER_CONFIG_FILE=$(ALERTMANAGER_TEST_DIR)/$(ALERTMANAGER_TEST_CONFIG_DIR)/$(ALERTMANAGER_TEST_CONFIG_FILE) \
		AMTOOL_BIN=$(AMTOOL_BIN) \
		$(BATS_BIN) $(BATS_ARGS) tests/alertmanager-routes/$*

.PHONY: tests-alertmanager-routes
tests-alertmanager-routes: $(subst /,-, $(shell find tests/alertmanager-routes -mindepth 1 -maxdepth 1 -type d -print|sort)) ## Run all Alertmanager routes tests

.PHONY: tests-alertmanager-routes-clean
tests-alertmanager-routes-clean:
	-rm -rf $(TESTS_WORKDIR)/chart-manifest-* tests/alertmanager-routes/*/alertmanager-config

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

clean: tests-alertmanager-routes-clean bin-dir-clean ## Clean up generated test files
