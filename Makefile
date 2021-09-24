GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
SOURCES := $(shell find . -type f  -name '*.go')

# Git information
GIT_VERSION ?= $(shell git describe --tags --dirty)
GIT_COMMIT_HASH ?= $(shell git rev-parse HEAD)
GIT_TREESTATE = "clean"
GIT_DIFF = $(shell git diff --quiet >/dev/null 2>&1; if [ $$? -eq 1 ]; then echo "1"; fi)
ifeq ($(GIT_DIFF), 1)
    GIT_TREESTATE = "dirty"
endif
BUILDDATE = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := "-X github.com/karmada-io/karmada/pkg/version.gitVersion=$(GIT_VERSION) \
                      -X github.com/karmada-io/karmada/pkg/version.gitCommit=$(GIT_COMMIT_HASH) \
                      -X github.com/karmada-io/karmada/pkg/version.gitTreeState=$(GIT_TREESTATE) \
                      -X github.com/karmada-io/karmada/pkg/version.buildDate=$(BUILDDATE)"

# Images management
REGISTRY_REGION?="ap-southeast-1"
ACCESS_KEY?=""
REGISTRY_LOGIN_KEY?=""
SWR_SERVICE_ADDRESS?="swr.ap-southeast-1.myhuaweicloud.com"
REGISTRY?="swr.ap-southeast-1.myhuaweicloud.com/karmada"
REGISTRY_USER_NAME?=""
REGISTRY_PASSWORD?=""
REGISTRY_SERVER_ADDRESS?=""

# Set your version by env or using latest tags from git
VERSION?=""
ifeq ($(VERSION), "")
    LATEST_TAG=$(shell git describe --tags)
    ifeq ($(LATEST_TAG),)
        # Forked repo may not sync tags from upstream, so give it a default tag to make CI happy.
        VERSION="unknown"
    else
        VERSION=$(LATEST_TAG)
    endif
endif

all: karmada-controller-manager karmada-scheduler karmadactl kubectl-karmada karmada-webhook karmada-agent karmada-scheduler-estimator

karmada-controller-manager: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmada-controller-manager \
		cmd/controller-manager/controller-manager.go

karmada-scheduler: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmada-scheduler \
		cmd/scheduler/main.go

karmadactl: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmadactl \
		cmd/karmadactl/karmadactl.go

kubectl-karmada: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o kubectl-karmada \
		cmd/kubectl-karmada/kubectl-karmada.go

karmada-webhook: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmada-webhook \
		cmd/webhook/main.go

karmada-agent: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmada-agent \
		cmd/agent/main.go

karmada-scheduler-estimator: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmada-scheduler-estimator \
		cmd/scheduler-estimator/main.go

clean:
	rm -rf karmada-controller-manager karmada-scheduler karmadactl kubectl-karmada karmada-webhook karmada-agent karmada-scheduler-estimator

.PHONY: update
update:
	hack/update-all.sh

.PHONY: verify
verify:
	hack/verify-all.sh

.PHONY: test
test:
	go test --race --v ./pkg/...

images: image-karmada-controller-manager image-karmada-scheduler image-karmada-webhook image-karmada-agent image-karmada-scheduler-estimator

image-karmada-controller-manager: karmada-controller-manager
	VERSION=$(VERSION) hack/docker.sh karmada-controller-manager

image-karmada-scheduler: karmada-scheduler
	VERSION=$(VERSION) hack/docker.sh karmada-scheduler

image-karmada-webhook: karmada-webhook
	VERSION=$(VERSION) hack/docker.sh karmada-webhook

image-karmada-agent: karmada-agent
	VERSION=$(VERSION) hack/docker.sh karmada-agent

image-karmada-scheduler-estimator: karmada-scheduler-estimator
	VERSION=$(VERSION) hack/docker.sh karmada-scheduler-estimator

upload-images: images
	@echo "push images to $(REGISTRY)"
ifneq ($(REGISTRY_USER_NAME), "")
	docker login -u ${REGISTRY_USER_NAME} -p ${REGISTRY_PASSWORD} ${REGISTRY_SERVER_ADDRESS}
endif
	docker push ${REGISTRY}/karmada-controller-manager:${VERSION}
	docker push ${REGISTRY}/karmada-scheduler:${VERSION}
	docker push ${REGISTRY}/karmada-webhook:${VERSION}
	docker push ${REGISTRY}/karmada-agent:${VERSION}
	docker push ${REGISTRY}/karmada-scheduler-estimator:${VERSION}
