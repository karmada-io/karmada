GOOS ?= $(shell go env GOOS)
SOURCES := $(shell find . -type f  -name '*.go')
GOBUILD_ENV = GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS)

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
	$(GOBUILD_ENV) go build \
		-ldflags $(LDFLAGS) \
		-o bin/karmada-controller-manager \
		cmd/controller-manager/controller-manager.go

karmada-scheduler: $(SOURCES)
	$(GOBUILD_ENV) go build \
		-ldflags $(LDFLAGS) \
		-o bin/karmada-scheduler \
		cmd/scheduler/main.go

karmadactl: $(SOURCES)
	$(GOBUILD_ENV) go build \
		-ldflags $(LDFLAGS) \
		-o bin/karmadactl \
		cmd/karmadactl/karmadactl.go

karmada-webhook: $(SOURCES)
	$(GOBUILD_ENV) go build \
		-ldflags $(LDFLAGS) \
		-o bin/karmada-webhook \
		cmd/webhook/main.go

karmada-agent: $(SOURCES)
	$(GOBUILD_ENV) go build \
		-ldflags $(LDFLAGS) \
		-o bin/karmada-agent \
		cmd/agent/main.go

clean:
	rm -rf bin

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
	docker build -t karmada-controller-manager:$(VERSION) -f ./cluster/images/karmada-controller-manager/Dockerfile ./bin

image-karmada-scheduler: karmada-scheduler
	docker build -t karmada-scheduler:$(VERSION) -f ./cluster/images/karmada-scheduler/Dockerfile ./bin

image-karmada-webhook: karmada-webhook
	docker build -t karmada-webhook:$(VERSION) -f ./cluster/images/karmada-webhook/Dockerfile ./bin

image-karmada-agent: karmada-agent
	docker build -t karmada-agent:$(VERSION) -f ./cluster/images/karmada-agent/Dockerfile ./bin

upload-images: images
	@echo "push images to ${REGISTRY}"
ifneq ($(REGISTRY_USER_NAME), "")
	docker login -u ${REGISTRY_USER_NAME} -p ${REGISTRY_PASSWORD} ${REGISTRY_SERVER_ADDRESS}
endif
	docker tag karmada-controller-manager:${VERSION} ${REGISTRY}/karmada-controller-manager:${VERSION}
	docker tag karmada-scheduler:${VERSION} ${REGISTRY}/karmada-scheduler:${VERSION}
	docker tag karmada-webhook:${VERSION} ${REGISTRY}/karmada-webhook:${VERSION}
	docker tag karmada-agent:${VERSION} ${REGISTRY}/karmada-agent:${VERSION}
	docker push ${REGISTRY}/karmada-controller-manager:${VERSION}
	docker push ${REGISTRY}/karmada-scheduler:${VERSION}
	docker push ${REGISTRY}/karmada-webhook:${VERSION}
	docker push ${REGISTRY}/karmada-agent:${VERSION}
