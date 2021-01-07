GOOS ?= $(shell go env GOOS)
SOURCES := $(shell find . -type f  -name '*.go')
LDFLAGS := ""

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
    $(info "Guessing version from git latest tags...")
    LATEST_TAG=$(shell git describe --tags)
    ifeq ($(LATEST_TAG),)
        # Forked repo may not sync tags from upstream, so give it a default tag to make CI happy.
        $(info "no tags found, set version with unknown")
        VERSION="unknown"
    else
        $(info "using latest git tag($(LATEST_TAG)) as version")
        VERSION=$(LATEST_TAG)
    endif
endif

all: karmada-controller-manager karmada-scheduler karmadactl

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

clean:
	rm -rf karmada-controller-manager karmada-scheduler karmadactl

.PHONY: test
test:
	go test --race --v ./pkg/...

images: image-karmada-controller-manager image-karmada-scheduler

image-karmada-controller-manager: karmada-controller-manager
	cp karmada-controller-manager cluster/images/karmada-controller-manager && \
	docker build -t $(REGISTRY)/karmada-controller-manager:$(VERSION) cluster/images/karmada-controller-manager && \
	rm cluster/images/karmada-controller-manager/karmada-controller-manager

image-karmada-scheduler: karmada-scheduler
	cp karmada-scheduler cluster/images/karmada-scheduler && \
	docker build -t $(REGISTRY)/karmada-scheduler:$(VERSION) cluster/images/karmada-scheduler && \
	rm cluster/images/karmada-scheduler/karmada-scheduler

upload-images: images
	@echo "push images to $(REGISTRY)"
ifneq ($(REGISTRY_USER_NAME), "")
	docker login -u ${REGISTRY_USER_NAME} -p ${REGISTRY_PASSWORD} ${REGISTRY_SERVER_ADDRESS}
endif
	docker push ${REGISTRY}/karmada-controller-manager:${VERSION}
