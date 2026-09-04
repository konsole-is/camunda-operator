# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Image URL of camunda-operator-cli, the companion image that the operator's
# Jobs run. The manager receives it through --camunda-operator-cli-image;
# build-installer and deploy stamp it into the manifests, helm-deploy into
# the chart values.
CLI_IMG ?= ghcr.io/konsole-is/camunda-operator-cli:latest
# The namespace that the manager runs in. It holds the Lease that serializes
# the claim of a logical database. A manager in a cluster reads it from its
# own Pod, so only `make run` needs this default. `make run` creates the
# namespace, because `make install` applies the CRDs alone and every claim
# would fail against a namespace that is not there.
OPERATOR_NAMESPACE ?= camunda-operator-system

# Recipes below expand IMG in the shell (e.g. $${IMG%:*}), which reads the
# environment. Make exports command-line variables automatically but not this
# default, so without the export `make helm-deploy` deploys an empty image
# reference. Exporting also keeps registry-with-port values like
# localhost:5000/img:tag correct, since ${IMG%:*} strips only the final colon.
export IMG
export CLI_IMG

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

# The repository holds two Go modules: the operator at the root and the API
# types in ./api, which consumers import without the operator's dependencies.
# A `./...` pattern never crosses a module boundary, so every Go tool below
# runs once per module.
MODULES := . ./api

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	cd api && "$(CONTROLLER_GEN)" crd paths="./..." output:crd:artifacts:config=../config/crd/bases
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role webhook paths="./..."

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	cd api && "$(CONTROLLER_GEN)" object:headerFile="../hack/boilerplate.go.txt" paths="./..."
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: observability
observability: ## Render the dashboards and alert rules of the framework for the metric namespace of the operator.
	go tool ocf observability render --metric-namespace camunda_operator --out config/prometheus/observability

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
	go run ./hack/callsplit ./api ./cmd ./internal ./pkg ./test
	@for m in $(MODULES); do (cd $$m && "$(GOLANGCI_LINT)" fmt) || exit 1; done

.PHONY: vet
vet: ## Run go vet against code.
	@for m in $(MODULES); do (cd $$m && go vet ./...) || exit 1; done

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	cd api && go test ./...
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

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
# Management Identity, a Console, and two Web Modeler processes each. Ginkgo
# has a suite timeout of its own, one hour by default, so the same value goes
# to -ginkgo.timeout.
E2E_TIMEOUT ?= 180m

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
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	set -a && . ./test/e2e/matrix/$(E2E_CAMUNDA_MINOR).env && set +a && \
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) ECK_VERSION=$(ECK_VERSION) \
		go test -tags=e2e ./test/e2e/ -v -ginkgo.v -timeout $(E2E_TIMEOUT) -ginkgo.timeout $(E2E_TIMEOUT) \
		$(if $(E2E_LABEL_FILTER),-ginkgo.label-filter="$(E2E_LABEL_FILTER)")
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint ## Run callsplit in check mode, then golangci-lint.
	go run ./hack/callsplit -check ./api ./cmd ./internal ./pkg ./test
	@for m in $(MODULES); do (cd $$m && "$(GOLANGCI_LINT)" run) || exit 1; done

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	go run ./hack/callsplit ./api ./cmd ./internal ./pkg ./test
	@for m in $(MODULES); do (cd $$m && "$(GOLANGCI_LINT)" fmt && "$(GOLANGCI_LINT)" run --fix) || exit 1; done

.PHONY: lint-config
lint-config: golangci-lint golangci-lint-schema ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify --schema "$(GOLANGCI_LINT_SCHEMA)"

.PHONY: lint-renovate
lint-renovate: ## Verify renovate.json5 with the validator of RENOVATE_VERSION. Needs npx.
	npx --yes --package renovate@$(RENOVATE_VERSION) renovate-config-validator --strict renovate.json5

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: build-cli
build-cli: fmt vet ## Build the camunda-operator-cli binary.
	go build -o bin/camunda-operator-cli ./cmd/camunda-operator-cli

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host. The backup Jobs it renders run CLI_IMG.
	@"$(KUBECTL)" create namespace "$(OPERATOR_NAMESPACE)" --dry-run=client -o yaml | "$(KUBECTL)" apply -f -
	go run ./cmd/main.go --camunda-operator-cli-image=${CLI_IMG} --namespace=${OPERATOR_NAMESPACE}

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: docker-build-cli
docker-build-cli: ## Build docker image with camunda-operator-cli.
	$(CONTAINER_TOOL) build -t ${CLI_IMG} -f Dockerfile.cli .

.PHONY: docker-push-cli
docker-push-cli: ## Push docker image with camunda-operator-cli.
	$(CONTAINER_TOOL) push ${CLI_IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/amd64,linux/arm64
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name camunda-operator-builder
	$(CONTAINER_TOOL) buildx use camunda-operator-builder
	$(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm camunda-operator-builder
	rm Dockerfile.cross

.PHONY: docker-buildx-cli
docker-buildx-cli: ## Build and push docker image for camunda-operator-cli for cross-platform support
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile.cli > Dockerfile.cli.cross
	- $(CONTAINER_TOOL) buildx create --name camunda-operator-builder
	$(CONTAINER_TOOL) buildx use camunda-operator-builder
	$(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${CLI_IMG} -f Dockerfile.cli.cross .
	- $(CONTAINER_TOOL) buildx rm camunda-operator-builder
	rm Dockerfile.cli.cross

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
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
# The JSON schema that `golangci-lint config verify` checks .golangci.yml against. golangci-lint fetches it itself
# with a 2 s HTTP timeout and no retry, which fails CI on a slow CDN answer, so lint-config downloads it with
# retries and passes it in. The file name carries the major.minor of GOLANGCI_LINT_VERSION (v2.8.0 -> v2.8).
GOLANGCI_LINT_SCHEMA = $(LOCALBIN)/golangci.$(basename $(GOLANGCI_LINT_VERSION)).jsonschema.json

## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.20.1

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
RENOVATE_VERSION ?= 44.62.0
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

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
## Helm version installed by install-helm. Pinned to .tool-versions.
HELM_VERSION ?= v4.1.4

.PHONY: install-helm
install-helm: ## Install the pinned version of Helm if it is missing.
	@command -v $(HELM) >/dev/null 2>&1 || { \
		echo "Installing Helm $(HELM_VERSION)..." && \
		curl -fsSL https://raw.githubusercontent.com/helm/helm/$(HELM_VERSION)/scripts/get-helm-4 \
			| bash -s -- --version $(HELM_VERSION); \
	}

.PHONY: helm-generate
helm-generate: build-installer ## Regenerate the Helm chart from kustomize output. Specify images with IMG and CLI_IMG.
	kubebuilder edit --plugins=helm/v2-alpha --force
	go run ./hack/helmcli "$(HELM_CHART_DIR)"

## Maximum gzipped rendered chart size in bytes before helm-verify fails.
## The real bound is the gzipped Helm release Secret against etcd's ~1MB object
## limit. The check measures `helm template ... | gzip -9 | wc -c` for every
## rendered permutation and keeps the worst case. The limit is deliberately half
## of the etcd bound, so the tripwire fires well before an install would fail.
## It is an early warning to trigger the CRD-split conversation, not a
## correctness check. When it trips, the CRD set has outgrown in-chart delivery
## and the CRDs must move to a chart of their own.
HELM_MAX_RENDER_GZIP_BYTES ?= 524288

.PHONY: helm-verify
helm-verify: install-helm ## Lint and render the Helm chart across value permutations. No cluster required.
	@test -d "$(HELM_CHART_DIR)" || { \
		echo "$(HELM_CHART_DIR) not found; run 'make helm-generate' first." >&2; exit 1; }
	$(HELM) lint "$(HELM_CHART_DIR)"
	@set -e; \
	max=0; worst=""; \
	for opts in \
		"" \
		"--set crd.enable=false" \
		"--set rbacHelpers.enable=true" \
		"--set prometheus.enable=true" \
		"--set certManager.enable=true" \
		"--set crd.enable=false --set rbacHelpers.enable=true --set prometheus.enable=true --set certManager.enable=true" \
		"--set rbacHelpers.enable=true --set prometheus.enable=true --set certManager.enable=true" \
	; do \
		label="$${opts:-<defaults>}"; \
		tmp="$$(mktemp)"; \
		$(HELM) template verify "$(HELM_CHART_DIR)" $$opts > "$$tmp"; \
		bytes="$$(wc -c < "$$tmp")"; \
		gz="$$(gzip -9 -c "$$tmp" | wc -c)"; \
		rm -f "$$tmp"; \
		printf '  %9s bytes (%7s gzipped)  helm template %s\n' "$$bytes" "$$gz" "$$label"; \
		if [ "$$gz" -gt "$$max" ]; then max=$$gz; worst="$$label"; fi; \
	done; \
	echo "  worst case: $$max bytes gzipped ($$worst), limit $(HELM_MAX_RENDER_GZIP_BYTES)"; \
	if [ "$$max" -gt "$(HELM_MAX_RENDER_GZIP_BYTES)" ]; then \
		echo "ERROR: gzipped rendered chart exceeds $(HELM_MAX_RENDER_GZIP_BYTES) bytes in configuration: $$worst" >&2; \
		echo "The CRD set has outgrown in-chart delivery. Consider splitting CRDs" >&2; \
		echo "into a separate chart shipped alongside this one." >&2; \
		exit 1; \
	fi

.PHONY: helm-deploy
helm-deploy: install-helm ## Deploy manager to the K8s cluster via Helm. Specify images with IMG and CLI_IMG.
	$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART_DIR) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--set manager.image.repository=$${IMG%:*} \
		--set manager.image.tag=$${IMG##*:} \
		--set manager.cliImage.repository=$${CLI_IMG%:*} \
		--set manager.cliImage.tag=$${CLI_IMG##*:} \
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

##@ Documentation

.PHONY: docs-serve
docs-serve: ## Serve the documentation site locally with live reload.
	mkdocs serve

.PHONY: docs-build
docs-build: ## Build the documentation site in strict mode.
	mkdocs build --strict
