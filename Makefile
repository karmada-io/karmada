GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
SOURCES := $(shell find . -type f  -name '*.go')

# to developers: should keep here empty by default.
# if user didn't set this env, scripts after this should know.
VERSION?=""

# Git information
GIT_VERSION ?= $(VERSION)
ifeq ($(GIT_VERSION), "")
    GIT_VERSION=$(shell git describe --tags --dirty)
endif

GIT_COMMIT_HASH ?= $(shell git rev-parse HEAD)
GIT_TREESTATE = "clean"
GIT_DIFF = $(shell git diff --quiet >/dev/null 2>&1; if [ $$? -eq 1 ]; then echo "1"; fi)
ifeq ($(GIT_DIFF), 1)
    GIT_TREESTATE = "dirty"
endif
BUILDDATE = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

IMAGE_PULL_VERSION ?= $(VERSION)
ifeq ($(IMAGE_PULL_VERSION), "")
    IMAGE_PULL_VERSION="latest"
endif

LDFLAGS := "-X github.com/karmada-io/karmada/pkg/version.gitVersion=$(GIT_VERSION) \
                      -X github.com/karmada-io/karmada/pkg/version.gitCommit=$(GIT_COMMIT_HASH) \
                      -X github.com/karmada-io/karmada/pkg/version.gitTreeState=$(GIT_TREESTATE) \
                      -X github.com/karmada-io/karmada/pkg/version.buildDate=$(BUILDDATE) \
                      -X github.com/karmada-io/karmada/pkg/version.imageVersion=$(IMAGE_PULL_VERSION)"

# Images management
REGISTRY_REGION?="ap-southeast-1"
ACCESS_KEY?=""
REGISTRY_LOGIN_KEY?=""
SWR_SERVICE_ADDRESS?="swr.ap-southeast-1.myhuaweicloud.com"
REGISTRY?="swr.ap-southeast-1.myhuaweicloud.com/karmada"
REGISTRY_USER_NAME?=""
REGISTRY_PASSWORD?=""
REGISTRY_SERVER_ADDRESS?=""

IMAGE_PUSH_VERSION?=$(VERSION)
# Set your version by env or using latest tags from git
ifeq ($(IMAGE_PUSH_VERSION), "")
    LATEST_TAG=$(shell git describe --tags)
    ifeq ($(LATEST_TAG),)
        # Forked repo may not sync tags from upstream, so give it a default tag to make CI happy.
        IMAGE_PUSH_VERSION="unknown"
    else
        IMAGE_PUSH_VERSION=$(LATEST_TAG)
    endif
endif

all: karmada-aggregated-apiserver karmada-controller-manager karmada-scheduler karmada-descheduler karmadactl kubectl-karmada karmada-webhook karmada-agent karmada-scheduler-estimator karmada-interpreter-webhook-example

karmada-aggregated-apiserver: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmada-aggregated-apiserver \
		cmd/aggregated-apiserver/main.go

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

karmada-descheduler: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmada-descheduler \
		cmd/descheduler/main.go

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

karmada-interpreter-webhook-example: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o karmada-interpreter-webhook-example \
		examples/customresourceinterpreter/webhook/main.go

clean:
	rm -rf karmada-aggregated-apiserver karmada-controller-manager karmada-scheduler karmada-descheduler karmadactl kubectl-karmada karmada-webhook karmada-agent karmada-scheduler-estimator karmada-interpreter-webhook-example

.PHONY: update
update:
	hack/update-all.sh

.PHONY: verify
verify:
	hack/verify-all.sh

.PHONY: test
test:
	go test --race --v ./pkg/...
	go test --race --v ./cmd/...
	go test --race --v ./examples/...

images: image-karmada-aggregated-apiserver image-karmada-controller-manager image-karmada-scheduler image-karmada-descheduler image-karmada-webhook image-karmada-agent image-karmada-scheduler-estimator image-karmada-interpreter-webhook-example

image-karmada-aggregated-apiserver: karmada-aggregated-apiserver
	VERSION=$(IMAGE_PUSH_VERSION) hack/docker.sh karmada-aggregated-apiserver

image-karmada-controller-manager: karmada-controller-manager
	VERSION=$(IMAGE_PUSH_VERSION) hack/docker.sh karmada-controller-manager

image-karmada-scheduler: karmada-scheduler
	VERSION=$(IMAGE_PUSH_VERSION) hack/docker.sh karmada-scheduler

image-karmada-descheduler: karmada-descheduler
	VERSION=$(IMAGE_PUSH_VERSION) hack/docker.sh karmada-descheduler

image-karmada-webhook: karmada-webhook
	VERSION=$(IMAGE_PUSH_VERSION) hack/docker.sh karmada-webhook

image-karmada-agent: karmada-agent
	VERSION=$(IMAGE_PUSH_VERSION) hack/docker.sh karmada-agent

image-karmada-scheduler-estimator: karmada-scheduler-estimator
	VERSION=$(IMAGE_PUSH_VERSION) hack/docker.sh karmada-scheduler-estimator

image-karmada-interpreter-webhook-example: karmada-interpreter-webhook-example
	VERSION=$(IMAGE_PUSH_VERSION) hack/docker.sh karmada-interpreter-webhook-example

upload-images: images
	@echo "push images to $(REGISTRY)"
ifneq ($(REGISTRY_USER_NAME), "")
	docker login -u ${REGISTRY_USER_NAME} -p ${REGISTRY_PASSWORD} ${REGISTRY_SERVER_ADDRESS}
endif
	docker push ${REGISTRY}/karmada-controller-manager:${IMAGE_PUSH_VERSION}
	docker push ${REGISTRY}/karmada-scheduler:${IMAGE_PUSH_VERSION}
	docker push ${REGISTRY}/karmada-descheduler:${IMAGE_PUSH_VERSION}
	docker push ${REGISTRY}/karmada-webhook:${IMAGE_PUSH_VERSION}
	docker push ${REGISTRY}/karmada-agent:${IMAGE_PUSH_VERSION}
	docker push ${REGISTRY}/karmada-scheduler-estimator:${IMAGE_PUSH_VERSION}
	docker push ${REGISTRY}/karmada-interpreter-webhook-example:${IMAGE_PUSH_VERSION}
	docker push ${REGISTRY}/karmada-aggregated-apiserver:${IMAGE_PUSH_VERSION}

