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

.PHONY: karmada-controller-manager
karmada-controller-manager:
	hack/build.sh controller-manager

.PHONY: karmada-scheduler
karmada-scheduler:
	hack/build.sh scheduler

.PHONY: karmadactl
karmadactl:
	hack/build.sh ctl

.PHONY: karmada-webhook
karmada-webhook:
	hack/build.sh webhook

.PHONY: karmada-agent
karmada-agent:
	hack/build.sh agent

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
	VERSION=$(VERSION) hack/docker.sh karmada-controller-manager

image-karmada-scheduler: karmada-scheduler
	VERSION=$(VERSION) hack/docker.sh karmada-scheduler

image-karmada-webhook: karmada-webhook
	VERSION=$(VERSION) hack/docker.sh karmada-webhook

image-karmada-agent: karmada-agent
	VERSION=$(VERSION) hack/docker.sh karmada-agent

upload-images: images
	@echo "push images to $(REGISTRY)"
ifneq ($(REGISTRY_USER_NAME), "")
	docker login -u ${REGISTRY_USER_NAME} -p ${REGISTRY_PASSWORD} ${REGISTRY_SERVER_ADDRESS}
endif
	docker push ${REGISTRY}/karmada-controller-manager:${VERSION}
	docker push ${REGISTRY}/karmada-scheduler:${VERSION}
	docker push ${REGISTRY}/karmada-webhook:${VERSION}
	docker push ${REGISTRY}/karmada-agent:${VERSION}
