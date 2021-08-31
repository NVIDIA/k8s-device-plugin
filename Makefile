# Copyright (c) 2020-2021, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


.PHONY: all build builder test
.DEFAULT_GOAL := all

##### Global variables #####

DOCKER   ?= docker
BUILDX   ?= buildx
ifeq ($(IMAGE),)
REGISTRY ?= nvcr.io/nvidia
IMAGE := $(REGISTRY)/k8s-device-plugin
endif
VERSION  ?= v0.9.0

GOLANG_VERSION ?= 1.15.8
CUDA_VERSION ?= 11.4.1

##### Public rules #####

DEFAULT_DISTRIBUTION := ubuntu20.04
DISTRIBUTIONS = $(DEFAULT_DISTRIBUTION) ubi8

BUILD_TARGETS := $(patsubst %,build-%,$(DISTRIBUTIONS))
PUSH_TARGETS := $(patsubst %,push-%,$(DISTRIBUTIONS))

.PHONY: $(DISTRIBUTIONS) $(BUILD_TARGETS) $(PUSH_TARGETS)

all: $(BUILD_TARGETS)

IMAGE_TAG ?= $(IMAGE):$(VERSION)-$(DISTRIBUTION)
OUT_IMAGE ?= $(IMAGE)
OUT_VERSION ?= $(VERSION)
OUT_IMAGE_TAG ?= $(OUT_IMAGE):$(OUT_VERSION)-$(DISTRIBUTION)

push: $(PUSH_TARGETS)
push-%: DISTRIBUTION = $(*)
$(PUSH_TARGETS): push-%:
	$(DOCKER) push "$(IMAGE):$(VERSION)-$(*)"

push-short:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-$(DEFAULT_DISTRIBUTION)" "$(IMAGE):$(VERSION)"
	$(DOCKER) push "$(IMAGE):$(VERSION)"

push-latest:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-$(DEFAULT_DISTRIBUTION)" "$(IMAGE):latest"
	$(DOCKER) push "$(IMAGE):latest"

$(DISTRIBUTIONS): %: build-%

build-%: DISTRIBUTION = $(*)
$(BUILD_TARGETS): build-%:
	$(DOCKER) build --pull \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg CUDA_VERSION=$(CUDA_VERSION) \
		--build-arg PLUGIN_VERSION=$(VERSION) \
		--build-arg BASE_DIST=$(DISTRIBUTION) \
		--tag $(IMAGE_TAG) \
		--file docker/Dockerfile \
			.

# Add multi-arch-builds using docker buildx
BUILD_MULTI_ARCH_TARGETS := $(patsubst %,build-multi-arch-%,$(DISTRIBUTIONS))
PUSH_MULTI_ARCH_TARGETS := $(patsubst %,push-multi-arch-%,$(DISTRIBUTIONS))
RELEASE_MULTI_ARCH_TARGETS := $(patsubst %,release-multi-arch-%,$(DISTRIBUTIONS))

MULTI_ARCH_TARGETS := $(BUILD_MULTI_ARCH_TARGETS) $(PUSH_MULTI_ARCH_TARGETS) $(RELEASE_MULTI_ARCH_TARGETS)
.PHONY: $(MULTI_ARCH_TARGETS)

BUILD_PLATFORM_OPTIONS := --platform=linux/amd64,linux/arm64
BUILD_PULL_OPTIONS := --pull
PUSH_ON_BUILD := false

build-multi-arch-%: DISTRIBUTION = $(*)
$(BUILD_MULTI_ARCH_TARGETS): build-multi-arch-%:
	$(DOCKER) $(BUILDX) build \
		$(BUILD_PLATFORM_OPTIONS) \
		$(BUILD_PULL_OPTIONS) \
		$(CACHE_OPTIONS) \
		--output=type=image,push=$(PUSH_ON_BUILD) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg CUDA_VERSION=$(CUDA_VERSION) \
		--build-arg PLUGIN_VERSION=$(VERSION) \
		--build-arg BASE_DIST=$(DISTRIBUTION) \
		--tag $(OUT_IMAGE_TAG) \
		--file docker/Dockerfile \
			.

push-multi-arch-%: PUSH_ON_BUILD := true
$(PUSH_MULTI_ARCH_TARGETS): push-multi-arch-%: build-multi-arch-%

release-multi-arch-%: DISTRIBUTION = $(*)
$(RELEASE_MULTI_ARCH_TARGETS): release-multi-arch-%:
	$(DOCKER) $(BUILDX) imagetools create \
		-t $(OUT_IMAGE_TAG) \
		$(IMAGE_TAG)

# Define local and dockerized golang targets
MODULE := github.com/NVIDIA/k8s-device-plugin

BUILDIMAGE_TAG ?= golang$(GOLANG_VERSION)
BUILDIMAGE ?= $(IMAGE)-build:$(BUILDIMAGE_TAG)

CHECK_TARGETS := assert-fmt vet lint ineffassign misspell
MAKE_TARGETS := fmt build check coverage $(CHECK_TARGETS)
DOCKER_TARGETS := $(patsubst %,docker-%, $(MAKE_TARGETS))
.PHONY: $(MAKE_TARGETS) $(DOCKER_TARGETS)

# Generate an image for containerized builds
# Note: This image is local only
.PHONY: .build-image .pull-build-image .push-build-image
.build-image: docker/Dockerfile.devel
	if [ x"$(SKIP_IMAGE_BUILD)" = x"" ]; then \
		$(DOCKER) build \
			--progress=plain \
			--build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
			--tag $(BUILDIMAGE) \
			-f $(^) \
			docker; \
	fi

.pull-build-image:
	$(DOCKER) pull $(BUILDIMAGE)

.push-build-image:
	$(DOCKER) push $(BUILDIMAGE)

# Define a docker-* target to run golang targets in a container based on the
# build image.
$(DOCKER_TARGETS): docker-%: .build-image
	@echo "Running 'make $(*)' in docker container $(BUILDIMAGE)"
	$(DOCKER) run \
		--rm \
		-e GOCACHE=/tmp/.cache \
		-v $(PWD):$(PWD) \
		-w $(PWD) \
		--user $$(id -u):$$(id -g) \
		$(BUILDIMAGE) \
			make $(*)

check: $(CHECK_TARGETS)

# Apply go fmt to the codebase
fmt:
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs gofmt -s -l -w

assert-fmt:
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs gofmt -s -l | ( grep -v /vendor/ || true ) > fmt.out
	@if [ -s fmt.out ]; then \
		echo "\nERROR: The following files are not formatted:\n"; \
		cat fmt.out; \
		rm fmt.out; \
		exit 1; \
	else \
		rm fmt.out; \
	fi

ineffassign:
	ineffassign $(MODULE)/...

lint:
# We use `go list -f '{{.Dir}}' $(MODULE)/...` to skip the `vendor` folder.
	go list -f '{{.Dir}}' $(MODULE)/... | xargs golint -set_exit_status

lint-internal:
# We use `go list -f '{{.Dir}}' $(MODULE)/...` to skip the `vendor` folder.
	go list -f '{{.Dir}}' $(MODULE)/internal/... | xargs golint -set_exit_status

misspell:
	misspell $(MODULE)/...

vet:
	go vet $(MODULE)/...

build:
	go build $(MODULE)/...

COVERAGE_FILE := coverage.out
unit-test: build
	go test -v -coverprofile=$(COVERAGE_FILE) $(MODULE)/...

coverage: unit-test
	cat $(COVERAGE_FILE) | grep -v "_mock.go" > $(COVERAGE_FILE).no-mocks
	go tool cover -func=$(COVERAGE_FILE).no-mocks
