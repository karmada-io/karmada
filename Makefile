GOOS ?= $(shell go env GOOS)
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

all: karmada-controller-manager karmada-scheduler karmadactl karmada-webhook karmada-agent

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

clean:
	rm -rf karmada-controller-manager karmada-scheduler karmadactl karmada-webhook karmada-agent

.PHONY: update
update:
	hack/update-all.sh

.PHONY: verify
verify:
	hack/verify-all.sh

.PHONY: test
test:
	go test --race --v ./pkg/...

images: image-karmada-controller-manager image-karmada-scheduler image-karmada-webhook image-karmada-agent

image-karmada-controller-manager: karmada-controller-manager
	cp karmada-controller-manager cluster/images/karmada-controller-manager && \
	docker build -t $(REGISTRY)/karmada-controller-manager:$(VERSION) cluster/images/karmada-controller-manager && \
	rm cluster/images/karmada-controller-manager/karmada-controller-manager

image-karmada-scheduler: karmada-scheduler
	cp karmada-scheduler cluster/images/karmada-scheduler && \
	docker build -t $(REGISTRY)/karmada-scheduler:$(VERSION) cluster/images/karmada-scheduler && \
	rm cluster/images/karmada-scheduler/karmada-scheduler

image-karmada-webhook: karmada-webhook
	cp karmada-webhook cluster/images/karmada-webhook && \
	docker build -t $(REGISTRY)/karmada-webhook:$(VERSION) cluster/images/karmada-webhook && \
	rm cluster/images/karmada-webhook/karmada-webhook

image-karmada-agent: karmada-agent
	cp karmada-agent cluster/images/karmada-agent && \
	docker build -t $(REGISTRY)/karmada-agent:$(VERSION) cluster/images/karmada-agent && \
	rm cluster/images/karmada-agent/karmada-agent

upload-images: images
	@echo "push images to $(REGISTRY)"
ifneq ($(REGISTRY_USER_NAME), "")
	docker login -u ${REGISTRY_USER_NAME} -p ${REGISTRY_PASSWORD} ${REGISTRY_SERVER_ADDRESS}
endif
	docker push ${REGISTRY}/karmada-controller-manager:${VERSION}
	docker push ${REGISTRY}/karmada-scheduler:${VERSION}
	docker push ${REGISTRY}/karmada-webhook:${VERSION}
	docker push ${REGISTRY}/karmada-agent:${VERSION}
