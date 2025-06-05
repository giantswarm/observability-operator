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
