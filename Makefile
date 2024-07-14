GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= '$(shell hack/version.sh)'

# Images management
REGISTRY?="docker.io/karmada"
REGISTRY_USER_NAME?=""
REGISTRY_PASSWORD?=""
REGISTRY_SERVER_ADDRESS?=""

TARGETS := karmada-aggregated-apiserver \
			karmada-controller-manager \
			karmada-scheduler \
			karmada-descheduler \
			karmada-webhook \
			karmada-agent \
			karmada-scheduler-estimator \
			karmada-interpreter-webhook-example \
			karmada-search \
			karmada-operator \
			karmada-metrics-adapter

CTL_TARGETS := karmadactl kubectl-karmada

# Build code.
#
# Args:
#   GOOS:   OS to build.
#   GOARCH: Arch to build.
#
# Example:
#   make
#   make all
#   make karmada-aggregated-apiserver
#   make karmada-aggregated-apiserver GOOS=linux
CMD_TARGET=$(TARGETS) $(CTL_TARGETS)

.PHONY: all
all: $(CMD_TARGET)

.PHONY: $(CMD_TARGET)
$(CMD_TARGET):
	BUILD_PLATFORMS=$(GOOS)/$(GOARCH) hack/build.sh $@

# Build image.
#
# Args:
#   GOARCH:      Arch to build.
#   OUTPUT_TYPE: Destination to save image(docker/registry).
#
# Example:
#   make images
#   make image-karmada-aggregated-apiserver
#   make image-karmada-aggregated-apiserver GOARCH=arm64
IMAGE_TARGET=$(addprefix image-, $(TARGETS))
.PHONY: $(IMAGE_TARGET)
$(IMAGE_TARGET):
	set -e;\
	target=$$(echo $(subst image-,,$@));\
	make $$target GOOS=linux;\
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) BUILD_PLATFORMS=linux/$(GOARCH) hack/docker.sh $$target

images: $(IMAGE_TARGET)

# Build and push multi-platform image to DockerHub
#
# Example
#   make multi-platform-images
#   make mp-image-karmada-aggregated-apiserver
MP_TARGET=$(addprefix mp-image-, $(TARGETS))
.PHONY: $(MP_TARGET)
$(MP_TARGET):
	set -e;\
	target=$$(echo $(subst mp-image-,,$@));\
	make $$target GOOS=linux GOARCH=amd64;\
	make $$target GOOS=linux GOARCH=arm64;\
	VERSION=$(VERSION) REGISTRY=$(REGISTRY) \
		OUTPUT_TYPE=registry \
		BUILD_PLATFORMS=linux/amd64,linux/arm64 \
		hack/docker.sh $$target

multi-platform-images: $(MP_TARGET)

.PHONY: clean
clean:
	rm -rf _tmp _output

.PHONY: update
update:
	hack/update-all.sh

.PHONY: verify
verify:
	hack/verify-all.sh

.PHONY: package-chart
package-chart:
	hack/package-helm-chart.sh $(VERSION)

.PHONY: push-chart
push-chart:
	helm push _output/charts/karmada-chart-${VERSION}.tgz oci://docker.io/karmada
	helm push _output/charts/karmada-operator-chart-${VERSION}.tgz oci://docker.io/karmada

GOTESTSUM_REGISTRY:=gotest.tools/gotestsum
GOTESTSUM_VERSION:=ec99a250836f069a524bb9d9b5de0a7a96334ea7 # v1.11.0
GOTESTSUM_FLAGS?=--format testname
GOTESTSUM_ENABLED?=
GOTEST=go test

.PHONY: install_gotestsum
install_gotestsum:
ifdef GOTESTSUM_ENABLED
	go install ${GOTESTSUM_REGISTRY}@${GOTESTSUM_VERSION}
GOTEST=gotestsum ${GOTESTSUM_FLAGS} --
endif

.PHONY: test
test: GO_TEST_FLAGS ?= --race --v -covermode=atomic
test: install_gotestsum
	mkdir -p ./_output/coverage/
	$(GOTEST) $(GO_TEST_FLAGS) ./pkg/... -coverprofile=./_output/coverage/coverage_pkg.txt
	$(GOTEST) $(GO_TEST_FLAGS) ./cmd/... -coverprofile=./_output/coverage/coverage_cmd.txt
	$(GOTEST) $(GO_TEST_FLAGS) ./examples/... -coverprofile=./_output/coverage/coverage_examples.txt
	$(GOTEST) $(GO_TEST_FLAGS) ./operator/... -coverprofile=./_output/coverage/coverage_operator.txt

upload-images: images
	@echo "push images to $(REGISTRY)"
ifneq ($(REGISTRY_USER_NAME), "")
	docker login -u ${REGISTRY_USER_NAME} -p ${REGISTRY_PASSWORD} ${REGISTRY_SERVER_ADDRESS}
endif
	docker push ${REGISTRY}/karmada-controller-manager:${VERSION}
	docker push ${REGISTRY}/karmada-scheduler:${VERSION}
	docker push ${REGISTRY}/karmada-descheduler:${VERSION}
	docker push ${REGISTRY}/karmada-webhook:${VERSION}
	docker push ${REGISTRY}/karmada-agent:${VERSION}
	docker push ${REGISTRY}/karmada-scheduler-estimator:${VERSION}
	docker push ${REGISTRY}/karmada-interpreter-webhook-example:${VERSION}
	docker push ${REGISTRY}/karmada-aggregated-apiserver:${VERSION}
	docker push ${REGISTRY}/karmada-search:${VERSION}
	docker push ${REGISTRY}/karmada-operator:${VERSION}
	docker push ${REGISTRY}/karmada-metrics-adapter:${VERSION}

# Build and package binary
#
# Example
#   make release-karmadactl
#   make release-kubectl-karmada
#   make release-kubectl-karmada GOOS=darwin GOARCH=amd64
RELEASE_TARGET=$(addprefix release-, $(CTL_TARGETS))
.PHONY: $(RELEASE_TARGET)
$(RELEASE_TARGET):
	@set -e;\
	target=$$(echo $(subst release-,,$@));\
	make $$target;\
	hack/release.sh $$target $(GOOS) $(GOARCH)

# Build and package binary for all platforms
#
# Example
#   make release
release:
	@make release-karmadactl GOOS=linux GOARCH=amd64
	@make release-karmadactl GOOS=linux GOARCH=arm64
	@make release-karmadactl GOOS=darwin GOARCH=amd64
	@make release-karmadactl GOOS=darwin GOARCH=arm64
	@make release-kubectl-karmada GOOS=linux GOARCH=amd64
	@make release-kubectl-karmada GOOS=linux GOARCH=arm64
	@make release-kubectl-karmada GOOS=darwin GOARCH=amd64
	@make release-kubectl-karmada GOOS=darwin GOARCH=arm64
