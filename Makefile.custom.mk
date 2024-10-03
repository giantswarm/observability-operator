.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	go generate ./...
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test-unit
test-unit: ginkgo generate fmt vet envtest ## Run unit tests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GINKGO) -p --nodes 4 -r -randomize-all --randomize-suites --skip-package=tests --cover --coverpkg=`go list ./... | grep -v fakes | tr '\n' ','` ./...

.PHONY: test-all
test-all: test-unit ## Run all tests

.PHONY: coverage-html
coverage-html: test-unit
	go tool cover -html coverprofile.out

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@latest)

ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

GINKGO = $(shell pwd)/bin/ginkgo
.PHONY: ginkgo
ginkgo: ## Download ginkgo locally if necessary.
	$(call go-get-tool,$(GINKGO),github.com/onsi/ginkgo/v2/ginkgo@latest)

DOCKER_COMPOSE = $(shell pwd)/bin/docker-compose
.PHONY: docker-compose
docker-compose: ## Download docker-compose locally if necessary.
	$(eval LATEST_RELEASE = $(shell curl -s https://api.github.com/repos/docker/compose/releases/latest | jq -r '.tag_name'))
	curl -sL "https://github.com/docker/compose/releases/download/$(LATEST_RELEASE)/docker-compose-linux-x86_64" -o $(DOCKER_COMPOSE)
	chmod +x $(DOCKER_COMPOSE)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
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
