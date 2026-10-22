# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Image URL of camunda-operator-cli, the companion image that the operator's
# Jobs run. The manager receives it through --camunda-operator-cli-image;
# build-installer and deploy stamp it into the manifests, helm-deploy into
# the chart values.
CLI_IMG ?= ghcr.io/konsole-is/camunda-operator-cli:latest
# Image URL of the fleet manager, the web app of the chart value
# fleetManager.enabled. helm-generate stamps it into the chart values.
FLEET_IMG ?= ghcr.io/konsole-is/camunda-operator-fleet-manager:latest
# Image URL of plugin-conformance, the conformance binary for the CI of a
# plugin that is not written in Go.
CONFORMANCE_IMG ?= ghcr.io/konsole-is/camunda-operator-plugin-conformance:latest
# CONFORMANCE_PLATFORMS are the platforms of the plugin-conformance binaries
# that dist-conformance builds for the release assets.
CONFORMANCE_PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
# Images of the fake plugin and the Git server of the kind harness of the
# fleet manager. The harness builds them and loads them into kind.
FAKEPLUGIN_IMG ?= fleet-harness-fakeplugin:dev
GITSERVER_IMG ?= fleet-harness-gitserver:dev
# OPERATOR_VERSION is the version that the manager reports, without the v of
# the tag. The release workflow sets it from the tag. Without it the manager
# reports 0.0.0-dev.
OPERATOR_VERSION ?= 0.0.0-dev
VERSION_PACKAGE := github.com/konsole-is/camunda-operator/internal/version
VERSION_LDFLAGS = -X $(VERSION_PACKAGE).version=$(OPERATOR_VERSION)
# The namespace that the manager runs in. It holds the Lease that serializes
# the claim of a logical database. A manager in a cluster reads it from its
# own Pod, so only `make run` needs this default. `make run` creates the
# namespace, because `make install` applies the CRDs alone and every claim
# would fail against a namespace that is not there.
OPERATOR_NAMESPACE ?= camunda-operator-system
# The ServiceAccount that the built-in PluginRegistrations name. A manager in
# a cluster gets it from its own Pod, so only `make run` needs this default.
OPERATOR_SERVICE_ACCOUNT ?= camunda-operator-controller-manager

# Recipes below expand IMG in the shell (e.g. $${IMG%:*}), which reads the
# environment. Make exports command-line variables automatically but not this
# default, so without the export `make helm-deploy` deploys an empty image
# reference. Exporting also keeps registry-with-port values like
# localhost:5000/img:tag correct, since ${IMG%:*} strips only the final colon.
export IMG
export CLI_IMG
export FLEET_IMG

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

# The repository holds three Go modules: the operator at the root, the API
# types in ./api, and the plugin conformance suite in ./conformance. Consumers
# import ./api, and plugin repositories import ./conformance, without the
# operator's dependencies. A `./...` pattern never crosses a module boundary,
# so `go mod tidy`, `go vet`, the linter, and `govulncheck` loop over MODULES,
# and `go test` has one line for each module.
MODULES := . ./api ./conformance

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	cd api && "$(CONTROLLER_GEN)" crd paths="./..." output:crd:artifacts:config=../config/crd/bases
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role webhook paths="./..."

.PHONY: generate
generate: controller-gen ## Generate the DeepCopy methods, and pkg/images/table.json from pkg/images and test/e2e/matrix.
	cd api && "$(CONTROLLER_GEN)" object:headerFile="../hack/boilerplate.go.txt" paths="./..."
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."
	go run ./hack/imagestable -matrix test/e2e/matrix -out pkg/images/table.json

.PHONY: observability
observability: ## Render the dashboards and alert rules of the framework for the metric namespace of the operator.
	go tool ocf observability render --metric-namespace camunda_operator --out config/prometheus/observability

.PHONY: merge-fixtures
merge-fixtures: ## Write the merged field of each case under test/fixtures/merge from the merge functions.
	go test ./test/fixtures/merge -count=1 -update

.PHONY: claimpolicy-fixtures
claimpolicy-fixtures: ## Write the result of each case under test/fixtures/claimpolicy from the functions of pkg/claimpolicy.
	go test ./pkg/claimpolicy -count=1 -update

.PHONY: tidy
tidy: ## Run go mod tidy in every module.
	@for m in $(MODULES); do (cd $$m && go mod tidy) || exit 1; done

# API_VERSION is the api module version that consumers of the root module get.
# The release workflow checks that it equals the release tag with a v prefix.
.PHONY: api-version
api-version: ## Pin the api module version in the root go.mod: make api-version VERSION=X.Y.Z
	@test -n "$(VERSION)" || { echo "usage: make api-version VERSION=X.Y.Z" >&2; exit 1; }
	go mod edit -require=github.com/konsole-is/camunda-operator/api@v$(VERSION)

.PHONY: fmt
fmt: golangci-lint ## Format code: callsplit, then the formatters configured in .golangci.yml.
	go run ./hack/callsplit ./api ./cmd ./conformance ./internal ./pkg ./test
	@for m in $(MODULES); do (cd $$m && "$(GOLANGCI_LINT)" fmt) || exit 1; done

.PHONY: vet
vet: ## Run go vet against code.
	@for m in $(MODULES); do (cd $$m && go vet ./...) || exit 1; done

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	cd api && go test ./...
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go -C conformance test ./...
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v '/test/e2e$$') -coverprofile cover.out

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager, CloudNativePG with the Barman Cloud plugin, and the ECK
# operator are installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
# - CNPG_INSTALL_SKIP=true
# - ECK_INSTALL_SKIP=true
# CNPG_VERSION and BARMAN_PLUGIN_VERSION pin the two CloudNativePG releases.
# They live in test/e2e/matrix/<minor>.env, which the test-e2e recipe exports,
# so they need no variable here.
# The suite runs an Elasticsearch node through ECK. The node needs
# vm.max_map_count of at least 262144 on the kind host.
KIND_CLUSTER ?= camunda-operator-test-e2e

# ECK_VERSION pins the ECK operator release that the e2e suite installs. Keep
# it on the same minor as the cloud-on-k8s module in go.mod: the operator
# under test renders Elasticsearch resources with the types of that module.
# renovate: datasource=github-releases depName=elastic/cloud-on-k8s extractVersion=^v(?<version>.*)$
ECK_VERSION ?= 3.5.0

# E2E_CAMUNDA_MINOR selects the Camunda minor the suite runs against. Each
# supported minor has a file test/e2e/matrix/<minor>.env with the image
# versions of that minor and the list of spec flows that run for it. The
# recipe exports the file to the suite. The e2e workflow runs three jobs per
# file, each with an E2E_LABEL_FILTER of its own.
E2E_CAMUNDA_MINOR ?= 8.9

# E2E_LABEL_FILTER is a Ginkgo label filter for one run of the suite. It wins
# over the E2E_LABELS list of the matrix entry. The e2e workflow runs each
# minor as three jobs with a filter each. The filters select the
# Elasticsearch flows, the Postgres flows, and the management plane flows.
# Empty runs the flows that the matrix entry names.
E2E_LABEL_FILTER ?=

# E2E_TIMEOUT bounds one `go test` run of the e2e suite. The suite pulls the
# Elasticsearch image and bootstraps a node, which takes longer than the
# default 10 minutes of go test. The Optimize flow adds an Elasticsearch, a
# Keycloak, a cluster, and two Optimize workloads of its own. The restore specs
# of both storage paths add two backups, two restores, and two point-in-time
# restores on top of that. The two management plane flows add a Keycloak, a
# Management Identity, a Console, and two Web Modeler processes each.
E2E_TIMEOUT ?= 180m

# E2E_GINKGO_TIMEOUT is the suite timeout of Ginkgo, one hour by default. It
# ends before E2E_TIMEOUT, so Ginkgo runs the AfterSuite and writes its report
# of the specs before go test panics.
E2E_GINKGO_TIMEOUT ?= 165m

# setup-test-e2e ends with an unconditional context switch. The e2e suite
# drives kubectl through the current context, and only a cluster that kind
# just created sets that context. A run against an existing kind cluster
# therefore applies the CRDs, the operator, and the test resources wherever
# the context points. The switch runs after both branches of the case, so
# neither path can drift away from the guard.
.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac
	@$(KUBECTL) config use-context kind-$(KIND_CLUSTER)

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate vet ## Run the e2e tests. Expected an isolated environment using Kind.
	set -a && . ./test/e2e/matrix/$(E2E_CAMUNDA_MINOR).env && set +a && \
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) ECK_VERSION=$(ECK_VERSION) \
		go test -tags=e2e ./test/e2e/ -v -ginkgo.v -timeout $(E2E_TIMEOUT) -ginkgo.timeout $(E2E_GINKGO_TIMEOUT) \
		$(if $(E2E_LABEL_FILTER),-ginkgo.label-filter="$(E2E_LABEL_FILTER)")
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint-examples
lint-examples: ## Check the version pins of config/example against the matrix entry of E2E_CAMUNDA_MINOR.
	go run ./hack/exampleversions -matrix test/e2e/matrix/$(E2E_CAMUNDA_MINOR).env config/example

.PHONY: lint
lint: golangci-lint lint-examples ## Check the example version pins and run callsplit, then golangci-lint.
	go run ./hack/callsplit -check ./api ./cmd ./conformance ./internal ./pkg ./test
	@for m in $(MODULES); do (cd $$m && "$(GOLANGCI_LINT)" run) || exit 1; done

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	go run ./hack/callsplit ./api ./cmd ./conformance ./internal ./pkg ./test
	@for m in $(MODULES); do (cd $$m && "$(GOLANGCI_LINT)" fmt && "$(GOLANGCI_LINT)" run --fix) || exit 1; done

.PHONY: lint-config
lint-config: golangci-lint golangci-lint-schema ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify --schema "$(GOLANGCI_LINT_SCHEMA)"

.PHONY: lint-renovate
lint-renovate: ## Verify renovate.json5 with the validator of RENOVATE_VERSION. Needs npx.
	npx --yes --package renovate@$(RENOVATE_VERSION) renovate-config-validator --strict renovate.json5

# govulncheck checks the stdlib of the Go that runs it. The golang builder image
# of the Dockerfiles is newer than the go line of go.mod, and .tool-versions
# holds the same Go release line, so run this target with the Go of
# .tool-versions.
.PHONY: vulncheck
vulncheck: govulncheck ## Fail on a known vulnerability that the code of a module can reach.
	@for m in $(MODULES); do (cd $$m && "$(GOVULNCHECK)" ./...) || exit 1; done

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -ldflags "$(VERSION_LDFLAGS)" -o bin/manager cmd/main.go

.PHONY: build-cli
build-cli: fmt vet ## Build the camunda-operator-cli binary.
	go build -o bin/camunda-operator-cli ./cmd/camunda-operator-cli

.PHONY: build-conformance
build-conformance: ## Build the plugin-conformance binary.
	go -C conformance build -o ../bin/plugin-conformance ./cmd/plugin-conformance

.PHONY: dist-conformance
dist-conformance: ## Build the plugin-conformance binary of each of CONFORMANCE_PLATFORMS into dist/.
	@mkdir -p dist
	@for p in $(CONFORMANCE_PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "Building dist/plugin-conformance-$$os-$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go -C conformance build -o ../dist/plugin-conformance-$$os-$$arch ./cmd/plugin-conformance || exit 1; \
	done

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host. The backup Jobs it renders run CLI_IMG.
	@"$(KUBECTL)" create namespace "$(OPERATOR_NAMESPACE)" --dry-run=client -o yaml | "$(KUBECTL)" apply -f -
	go run ./cmd/main.go --camunda-operator-cli-image=${CLI_IMG} --namespace=${OPERATOR_NAMESPACE} \
		--service-account=${OPERATOR_SERVICE_ACCOUNT}

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build --build-arg OPERATOR_VERSION=$(OPERATOR_VERSION) -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: docker-build-cli
docker-build-cli: ## Build docker image with camunda-operator-cli.
	$(CONTAINER_TOOL) build -t ${CLI_IMG} -f Dockerfile.cli .

.PHONY: docker-push-cli
docker-push-cli: ## Push docker image with camunda-operator-cli.
	$(CONTAINER_TOOL) push ${CLI_IMG}

.PHONY: docker-build-conformance
docker-build-conformance: ## Build docker image with plugin-conformance.
	$(CONTAINER_TOOL) build -t ${CONFORMANCE_IMG} -f Dockerfile.conformance .

.PHONY: docker-build-fakeplugin
docker-build-fakeplugin: ## Build the image of the fake plugin of the fleet manager harness on kind.
	$(CONTAINER_TOOL) build -t ${FAKEPLUGIN_IMG} -f test/e2e/fakeplugin/Dockerfile .

.PHONY: docker-build-gitserver
docker-build-gitserver: ## Build the image of the Git server of the fleet manager harness on kind.
	$(CONTAINER_TOOL) build -t ${GITSERVER_IMG} -f test/e2e/gitserver/Dockerfile .

.PHONY: docker-push-conformance
docker-push-conformance: ## Push docker image with plugin-conformance.
	$(CONTAINER_TOOL) push ${CONFORMANCE_IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/amd64,linux/arm64
# BUILDX_OUTPUT is where docker-buildx, docker-buildx-cli, docker-buildx-conformance, and docker-buildx-fleet send the image. The release workflow sets it to
# --load, with one platform in PLATFORMS, to scan the image of each platform before it pushes them.
BUILDX_OUTPUT ?= --push
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name camunda-operator-builder
	$(CONTAINER_TOOL) buildx use camunda-operator-builder
	$(CONTAINER_TOOL) buildx build $(BUILDX_OUTPUT) --platform=$(PLATFORMS) --build-arg OPERATOR_VERSION=$(OPERATOR_VERSION) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm camunda-operator-builder
	rm Dockerfile.cross

.PHONY: docker-buildx-cli
docker-buildx-cli: ## Build and push docker image for camunda-operator-cli for cross-platform support
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile.cli > Dockerfile.cli.cross
	- $(CONTAINER_TOOL) buildx create --name camunda-operator-builder
	$(CONTAINER_TOOL) buildx use camunda-operator-builder
	$(CONTAINER_TOOL) buildx build $(BUILDX_OUTPUT) --platform=$(PLATFORMS) --tag ${CLI_IMG} -f Dockerfile.cli.cross .
	- $(CONTAINER_TOOL) buildx rm camunda-operator-builder
	rm Dockerfile.cli.cross

.PHONY: docker-buildx-conformance
docker-buildx-conformance: ## Build and push docker image for plugin-conformance for cross-platform support
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile.conformance > Dockerfile.conformance.cross
	- $(CONTAINER_TOOL) buildx create --name camunda-operator-builder
	$(CONTAINER_TOOL) buildx use camunda-operator-builder
	$(CONTAINER_TOOL) buildx build $(BUILDX_OUTPUT) --platform=$(PLATFORMS) --tag ${CONFORMANCE_IMG} -f Dockerfile.conformance.cross .
	- $(CONTAINER_TOOL) buildx rm camunda-operator-builder
	rm Dockerfile.conformance.cross

.PHONY: test-version-stamp
test-version-stamp: OPERATOR_VERSION = 0.3.0
test-version-stamp: ## Check that a build with OPERATOR_VERSION reports it, and that the image builds stamp it too.
	WANT_OPERATOR_VERSION=$(OPERATOR_VERSION) go test -count=1 -ldflags "$(VERSION_LDFLAGS)" \
		-run '^TestVersionReportsStampedTag$$' ./internal/version
	@grep -qx 'ARG OPERATOR_VERSION=0.0.0-dev' Dockerfile && \
		grep -qF -- '-X $(VERSION_PACKAGE).version=$${OPERATOR_VERSION}' Dockerfile || \
		{ echo "Dockerfile does not stamp $(VERSION_PACKAGE).version from ARG OPERATOR_VERSION" >&2; exit 1; }
	@for target in docker-build docker-buildx; do \
		$(MAKE) -s -n $$target OPERATOR_VERSION=$(OPERATOR_VERSION) | \
			grep -qF -- '--build-arg OPERATOR_VERSION=$(OPERATOR_VERSION)' || \
			{ echo "make $$target does not pass OPERATOR_VERSION to the Dockerfile" >&2; exit 1; }; \
	done

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	cd config/manager && "$(KUSTOMIZE)" edit set image ghcr.io/konsole-is/camunda-operator-cli=${CLI_IMG}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml
	"$(KUSTOMIZE)" build config/crd > dist/crds.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --server-side --force-conflicts -f -; else echo "No CRDs to install; skipping."; fi

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	cd config/manager && "$(KUSTOMIZE)" edit set image ghcr.io/konsole-is/camunda-operator-cli=${CLI_IMG}
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply --server-side --force-conflicts -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOVULNCHECK = $(LOCALBIN)/govulncheck
# The JSON schema that `golangci-lint config verify` checks .golangci.yml against. golangci-lint fetches it itself
# with a 2 s HTTP timeout and no retry, which fails CI on a slow CDN answer, so lint-config downloads it with
# retries and passes it in. The file name carries the major.minor of GOLANGCI_LINT_VERSION (v2.8.0 -> v2.8).
GOLANGCI_LINT_SCHEMA = $(LOCALBIN)/golangci.$(basename $(GOLANGCI_LINT_VERSION)).jsonschema.json

## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.20.1
# renovate: datasource=go depName=github.com/elastic/crd-ref-docs
CRD_REF_DOCS_VERSION ?= v0.3.0

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.8.0
# RENOVATE_VERSION is the Renovate release whose validator lint-renovate runs
# against renovate.json5.
# renovate: datasource=npm depName=renovate
RENOVATE_VERSION ?= 44.148.1
# renovate: datasource=go depName=golang.org/x/vuln
GOVULNCHECK_VERSION ?= v1.8.0
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	$(call go-install-tool,$(CRD_REF_DOCS),github.com/elastic/crd-ref-docs,$(CRD_REF_DOCS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

# govulncheck cannot load code that needs a newer Go than the Go that built it, so a change of Go builds it again.
.PHONY: govulncheck
govulncheck: | $(LOCALBIN) ## Download govulncheck locally if necessary.
	@want="$(GOVULNCHECK_VERSION) $$(go env GOVERSION)"; \
	[ -x "$(GOVULNCHECK)" ] && [ "$$(cat "$(GOVULNCHECK).stamp" 2>/dev/null)" = "$$want" ] || { \
		set -e; \
		echo "Downloading golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)"; \
		GOBIN="$(LOCALBIN)" go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); \
		echo "$$want" > "$(GOVULNCHECK).stamp"; \
	}

.PHONY: golangci-lint-schema
golangci-lint-schema: $(GOLANGCI_LINT_SCHEMA) ## Download the golangci-lint JSON schema locally if necessary.
$(GOLANGCI_LINT_SCHEMA): | $(LOCALBIN)
	curl -fsSL --retry 5 --retry-all-errors --retry-delay 2 --max-time 60 \
		-o "$@.tmp" "https://golangci-lint.run/jsonschema/$(notdir $@)" && mv -f "$@.tmp" "$@"

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
	@test -f .custom-gcl.yml && { \
		echo "Building custom golangci-lint with plugins..." && \
		$(GOLANGCI_LINT) custom --destination $(LOCALBIN) --name golangci-lint-custom && \
		mv -f $(LOCALBIN)/golangci-lint-custom $(GOLANGCI_LINT); \
	} || true

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

##@ Helm Deployment

## Helm binary to use for deploying the chart
HELM ?= helm
## Namespace to deploy the Helm release
HELM_NAMESPACE ?= camunda-operator-system
## Name of the Helm release
HELM_RELEASE ?= camunda-operator
## Path to the Helm chart directory
HELM_CHART_DIR ?= dist/chart
## Additional arguments to pass to helm commands
HELM_EXTRA_ARGS ?=
## Helm version installed by install-helm. Renovate moves it with .tool-versions.
# renovate: datasource=github-releases depName=helm/helm
HELM_VERSION ?= v4.3.0

.PHONY: install-helm
install-helm: ## Install the pinned version of Helm if it is missing.
	@command -v $(HELM) >/dev/null 2>&1 || { \
		echo "Installing Helm $(HELM_VERSION)..." && \
		curl -fsSL https://raw.githubusercontent.com/helm/helm/$(HELM_VERSION)/scripts/get-helm-4 \
			| bash -s -- --version $(HELM_VERSION); \
	}

.PHONY: helm-generate
helm-generate: build-installer ## Regenerate the Helm chart from kustomize output. Specify images with IMG, CLI_IMG, and FLEET_IMG.
	kubebuilder edit --plugins=helm/v2-alpha --force
	go run ./hack/helmcrds "$(HELM_CHART_DIR)"
	go run ./hack/helmcli "$(HELM_CHART_DIR)"
	go run ./hack/helmfleet -image "$(FLEET_IMG)" "$(HELM_CHART_DIR)"
	go run ./hack/helmschema "$(HELM_CHART_DIR)"

## Largest Helm release Secret, in bytes, before helm-verify fails. Helm
## stores each release as base64(gzip(json)) in one Secret, and the API server
## rejects a Secret with more than 1048576 bytes of data. The release holds
## every chart template and the rendered manifest, so each CRD counts twice.
## The max is three quarters of that limit. The rest is room for the values
## that a user sets and for the next CRDs. When the check fails, the CRDs must
## move out of the release, for example to a chart of their own.
HELM_MAX_RELEASE_BYTES ?= 786432

.PHONY: helm-verify
helm-verify: install-helm ## Lint and render the Helm chart across value permutations. No cluster required.
	@test -f "$(HELM_CHART_DIR)/values.schema.json" || { \
		echo "$(HELM_CHART_DIR)/values.schema.json not found; run 'make helm-generate' first." >&2; exit 1; }
	$(HELM) lint "$(HELM_CHART_DIR)"
	@# The schema must reject an unknown key and a value of the wrong type.
	@for opts in "--set prometheus.enabeld=true" "--set prometheus.enabled=yes" "--set fleetManager.enabeld=true" \
		"--set fleetManager.extraCACerts.configMapname=x" "--set fleetManager.extraCACerts.configMapName=1"; do \
		err="$$($(HELM) template verify "$(HELM_CHART_DIR)" $$opts 2>&1 >/dev/null || true)"; \
		case "$$err" in *"meet the specifications of the schema"*) ;; *) \
			echo "ERROR: the values schema did not reject: helm template $$opts" >&2; exit 1 ;; \
		esac; \
		echo "  schema rejects: helm template $$opts"; \
	done
	@# Helm passes global to a subchart, and the schema must accept it.
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
	mkdir "$$tmp/charts"; cp -R "$(HELM_CHART_DIR)" "$$tmp/charts/camunda-operator"; \
	printf 'apiVersion: v2\nname: parent\nversion: 0.0.0\ndependencies:\n  - name: camunda-operator\n    version: "*"\n' \
		> "$$tmp/Chart.yaml"; \
	$(HELM) template verify "$$tmp" --set global.example=true > /dev/null; \
	echo "  renders as a subchart with global values"
	@# helm-deploy passes the image tags like this, and a tag can be a number.
	$(HELM) template verify "$(HELM_CHART_DIR)" \
		--set-string manager.image.tag=8 --set-string manager.cliImage.tag=8 > /dev/null
	$(HELM) template verify "$(HELM_CHART_DIR)" --set fleetManager.enabled=true \
		--set-string fleetManager.image.tag=8 > /dev/null
	@# The fleet manager renders only when fleetManager.enabled is true. Then it
	@# renders with the RBAC list of the app and a NetworkPolicy that refuses
	@# all ingress. dist/install.yaml never holds it.
	$(HELM) template verify "$(HELM_CHART_DIR)" | go run ./hack/fleetchart -enabled=false
	$(HELM) template verify "$(HELM_CHART_DIR)" --set fleetManager.enabled=true \
		| go run ./hack/fleetchart -enabled=true -crds config/crd/bases
	go run ./hack/fleetchart -enabled=false < dist/install.yaml
	@# The install and the chart ship the Grant admission policy and its binding.
	@for kind in ValidatingAdmissionPolicy ValidatingAdmissionPolicyBinding; do \
		grep -qx "kind: $$kind" dist/install.yaml || { \
			echo "ERROR: dist/install.yaml has no $$kind" >&2; exit 1; }; \
		$(HELM) template verify "$(HELM_CHART_DIR)" --set crd.enabled=false | grep -qx "kind: $$kind" || { \
			echo "ERROR: the chart does not render a $$kind" >&2; exit 1; }; \
	done
	@echo "  the install and the chart ship the Grant admission policy"
	@$(HELM) template verify "$(HELM_CHART_DIR)" --set fleetManager.enabled=true \
		--set 'manager.imagePullSecrets[0].name=verify-pull' \
		--show-only templates/fleet-manager/deployment.yaml | grep -q 'name: verify-pull' || { \
		echo "ERROR: the fleet manager Pod does not use manager.imagePullSecrets" >&2; exit 1; }
	@echo "  the fleet manager Pod uses manager.imagePullSecrets"
	@out="$$($(HELM) template verify "$(HELM_CHART_DIR)" --set fleetManager.enabled=true \
		--set fleetManager.extraCACerts.configMapName=verify-ca --set fleetManager.extraCACerts.key=bundle.pem \
		--show-only templates/fleet-manager/deployment.yaml)"; \
	for want in 'name: NODE_EXTRA_CA_CERTS' 'value: /etc/fleet-manager/extra-ca/ca.crt' \
		'name: "verify-ca"' 'key: "bundle.pem"' 'mountPath: /etc/fleet-manager/extra-ca'; do \
		printf '%s\n' "$$out" | grep -qF "$$want" || { \
			echo "ERROR: fleetManager.extraCACerts does not render '$$want'" >&2; exit 1; }; \
	done
	@$(HELM) template verify "$(HELM_CHART_DIR)" --set fleetManager.enabled=true \
		--show-only templates/fleet-manager/deployment.yaml | grep -q NODE_EXTRA_CA_CERTS && { \
		echo "ERROR: the fleet manager sets NODE_EXTRA_CA_CERTS without extraCACerts" >&2; exit 1; } || true
	@echo "  fleetManager.extraCACerts mounts the CA and sets NODE_EXTRA_CA_CERTS"
	@# Measure the release Secret that an install writes, for each set of values.
	@echo "  release Secret size (limit 1048576 bytes):"
	@for opts in \
		"" \
		"--set crd.enabled=false" \
		"--set rbac.helpers.enabled=true" \
		"--set prometheus.enabled=true" \
		"--set certManager.enabled=true" \
		"--set fleetManager.enabled=true" \
		"--set rbac.helpers.enabled=true --set prometheus.enabled=true --set certManager.enabled=true --set fleetManager.enabled=true" \
	; do \
		$(HELM) install verify "$(HELM_CHART_DIR)" --namespace verify --dry-run=client -o json $$opts \
			| go run ./hack/helmrelease -max $(HELM_MAX_RELEASE_BYTES) -label "$${opts:-<defaults>}" || exit 1; \
	done

.PHONY: helm-deploy
helm-deploy: install-helm ## Deploy manager to the K8s cluster via Helm. Specify images with IMG, CLI_IMG, and FLEET_IMG.
	$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART_DIR) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--set-string manager.image.repository=$${IMG%:*} \
		--set-string manager.image.tag=$${IMG##*:} \
		--set-string manager.cliImage.repository=$${CLI_IMG%:*} \
		--set-string manager.cliImage.tag=$${CLI_IMG##*:} \
		--set-string fleetManager.image.repository=$${FLEET_IMG%:*} \
		--set-string fleetManager.image.tag=$${FLEET_IMG##*:} \
		--wait \
		--timeout 5m \
		$(HELM_EXTRA_ARGS)

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall the Helm release from the K8s cluster.
	$(HELM) uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-status
helm-status: ## Show Helm release status.
	$(HELM) status $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-history
helm-history: ## Show Helm release history.
	$(HELM) history $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-rollback
helm-rollback: ## Rollback to previous Helm release.
	$(HELM) rollback $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

##@ Fleet manager

FLEET_DIR := fleet-manager
# packageManager in the package.json of FLEET_DIR is the only pin of pnpm.
# Node.js 26 has no corepack, so npm installs that release into LOCALBIN.
FLEET_PNPM_VERSION := $(shell sed -n 's/.*"packageManager": *"pnpm@\([^"]*\)".*/\1/p' $(FLEET_DIR)/package.json)
FLEET_PNPM_BIN := $(LOCALBIN)/pnpm-$(FLEET_PNPM_VERSION)/bin/pnpm
FLEET_PNPM := cd $(FLEET_DIR) && NEXT_TELEMETRY_DISABLED=1 "$(FLEET_PNPM_BIN)"

$(FLEET_PNPM_BIN): | $(LOCALBIN)
	npm install --global --allow-scripts=pnpm --prefix "$(LOCALBIN)/pnpm-$(FLEET_PNPM_VERSION)" pnpm@$(FLEET_PNPM_VERSION)

.PHONY: fleet-install
fleet-install: $(FLEET_PNPM_BIN) ## Install the fleet manager dependencies from the lockfile.
	$(FLEET_PNPM) install --frozen-lockfile

.PHONY: fleet-lint
fleet-lint: fleet-install ## Run eslint, prettier --check, and tsc --noEmit on the fleet manager.
	$(FLEET_PNPM) run lint

# The shell runs with pipefail, so a failed test still fails the target through tee.
.PHONY: fleet-test
fleet-test: fleet-install setup-envtest ## Run the fleet manager tests with Vitest. The tests of fleet-manager/test/k8s run against envtest. The output and a JUnit report go to fleet-manager/test-reports.
	rm -rf $(FLEET_DIR)/test-reports && mkdir -p $(FLEET_DIR)/test-reports
	export KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" && \
		$(FLEET_PNPM) run test --reporter=default --reporter=junit --outputFile.junit=test-reports/junit.xml 2>&1 | tee test-reports/vitest.log

.PHONY: fleet-build
fleet-build: fleet-install ## Build the fleet manager with next build.
	$(FLEET_PNPM) run build

.PHONY: fleet-generate
fleet-generate: fleet-install ## Generate fleet-manager/src/generated from the CRDs and the condition types of api/.
	mkdir -p $(FLEET_DIR)/src/generated
	go run ./hack/tsconditions -api api -out $(FLEET_DIR)/src/generated/conditions.ts
	go run ./hack/tsfielddocs -crds config/crd/bases -out $(FLEET_DIR)/src/generated/fielddocs.ts
	$(FLEET_PNPM) run generate

.PHONY: fleet-scenarios
fleet-scenarios: setup-envtest ## Generate the page scenarios of the fleet manager with the reconcilers on envtest.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" \
		go run ./hack/fleetscenarios -dir $(FLEET_DIR)/e2e/scenarios

# CI passes --with-deps, so Playwright also installs the system libraries of
# the browser. On a workstation they are usually there.
FLEET_PLAYWRIGHT_FLAGS ?=

.PHONY: fleet-playwright
fleet-playwright: fleet-install ## Install the Chromium browser that the fleet manager Playwright tests use.
	$(FLEET_PNPM) exec playwright install chromium $(FLEET_PLAYWRIGHT_FLAGS)

.PHONY: fleet-gallery
fleet-gallery: fleet-build fleet-playwright setup-envtest ## Screenshot each fleet manager gallery story at 1440 px and 390 px into fleet-manager/ui-gallery, fail on a serious or critical axe violation, and check that a page of 8 data hooks opens one watch connection.
	$(FLEET_PNPM) run build:gallery
	export KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" && $(FLEET_PNPM) run gallery

# The kind cluster of the fleet manager journeys. The harness targets run in
# CI only: they create and fill a kind cluster.
FLEET_HARNESS_CLUSTER ?= fleet-harness

.PHONY: fleet-harness-up
fleet-harness-up: ## Create the kind cluster of the fleet manager journeys and install the operator, the app, the fake plugin, and the Git server. CI only.
	KIND_CLUSTER=$(FLEET_HARNESS_CLUSTER) $(FLEET_DIR)/e2e/harness/up.sh

.PHONY: fleet-harness-check
fleet-harness-check: ## Check the fake plugin outputs, the Git apply loop, and the CA trust of the app on the harness cluster.
	$(FLEET_DIR)/e2e/harness/check.sh

.PHONY: fleet-journeys
fleet-journeys: fleet-playwright ## Run the fleet manager journeys on the harness cluster and write fleet-manager/ui-journeys.
	$(FLEET_DIR)/e2e/harness/journeys.sh "$(FLEET_PNPM_BIN)" exec playwright test -c playwright.journeys.config.ts

.PHONY: docker-build-fleet
docker-build-fleet: ## Build the fleet manager image.
	$(CONTAINER_TOOL) build -t ${FLEET_IMG} -f $(FLEET_DIR)/Dockerfile .

.PHONY: docker-push-fleet
docker-push-fleet: ## Push the fleet manager image.
	$(CONTAINER_TOOL) push ${FLEET_IMG}

# docker-buildx-fleet builds each platform in emulation. docker-buildx builds
# on the platform of the builder instead, but pnpm installs the native
# packages of the platform it runs on, and the standalone output copies them.
.PHONY: docker-buildx-fleet
docker-buildx-fleet: ## Build and push the fleet manager image for cross-platform support
	- $(CONTAINER_TOOL) buildx create --name camunda-operator-builder
	$(CONTAINER_TOOL) buildx use camunda-operator-builder
	$(CONTAINER_TOOL) buildx build $(BUILDX_OUTPUT) --platform=$(PLATFORMS) --tag ${FLEET_IMG} -f $(FLEET_DIR)/Dockerfile .
	- $(CONTAINER_TOOL) buildx rm camunda-operator-builder

.PHONY: fleet-image-test
fleet-image-test: docker-build-fleet ## Build the fleet manager image and check that it starts as non-root with its data.
	CONTAINER_TOOL=$(CONTAINER_TOOL) $(FLEET_DIR)/deploy/test-image.sh "$(FLEET_IMG)"

##@ Documentation

.PHONY: docs-serve
docs-serve: ## Serve the documentation site locally with live reload.
	mkdocs serve

.PHONY: docs-build
docs-build: ## Build the documentation site in strict mode.
	mkdocs build --strict

.PHONY: api-docs
api-docs: crd-ref-docs ## Render the API reference in docs/crds/api-reference.md from the types in api/.
	{ cat hack/crd-ref-docs/config.yaml; \
		printf 'render:\n  kubernetesVersion: %s\n' "$(ENVTEST_K8S_VERSION)"; } > "$(LOCALBIN)/crd-ref-docs.yaml"
	"$(CRD_REF_DOCS)" --source-path=api --config="$(LOCALBIN)/crd-ref-docs.yaml" \
		--renderer=markdown --output-path=docs/crds/api-reference.md
	go run ./hack/apireference docs/crds/api-reference.md

.PHONY: docs-sentences
docs-sentences: ## Report the sentences under docs/ that break the simple-english rules: length, semicolons, and the modals would, could, and may.
	go run ./hack/sentencelength docs
