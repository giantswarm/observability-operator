###############################################################################
# Testing & Coverage
###############################################################################

generate-golden-files: ## Generate golden files for tests
	@echo "Generating golden files"
	@UPDATE_GOLDEN_FILES=true go test -v ./...

.PHONY: coverage-html
coverage-html: test ## Generate HTML coverage report from merged profile.
	go tool cover -html coverprofile.out

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
