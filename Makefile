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
ifeq ($(IMAGE_NAME),)
REGISTRY ?= nvcr.io/nvidia
IMAGE_NAME := $(REGISTRY)/k8s-device-plugin
endif
VERSION  ?= v0.10.0

GOLANG_VERSION ?= 1.15.8
CUDA_VERSION ?= 11.4.2

##### Public rules #####

DEFAULT_DISTRIBUTION := ubuntu20.04
DISTRIBUTIONS = $(DEFAULT_DISTRIBUTION) ubi8

BUILD_TARGETS := $(patsubst %,build-%,$(DISTRIBUTIONS))
PUSH_TARGETS := $(patsubst %,push-%,$(DISTRIBUTIONS))

.PHONY: $(DISTRIBUTIONS) $(BUILD_TARGETS) $(PUSH_TARGETS)

all: $(BUILD_TARGETS)

IMAGE_VERSION := $(VERSION)

IMAGE_TAG ?= $(IMAGE_VERSION)-$(IMAGE_DISTRIBUTION)
IMAGE ?= $(IMAGE_NAME):$(IMAGE_TAG)
OUT_IMAGE_NAME ?= $(IMAGE_NAME)
OUT_IMAGE_VERSION ?= $(IMAGE_VERSION)
OUT_IMAGE_TAG = $(OUT_IMAGE_VERSION)-$(IMAGE_DISTRIBUTION)
OUT_IMAGE = $(OUT_IMAGE_NAME):$(OUT_IMAGE_TAG)

ifneq ($(BUILDX_CACHE_TO),)
CACHE_TO_OPTIONS = --cache-to=type=local,dest=$(BUILDX_CACHE_TO),mode=max
endif

ifneq ($(BUILDX_CACHE_FROM),)
CACHE_FROM_OPTIONS = --cache-from=type=local,src=$(BUILDX_CACHE_FROM)
endif

BUILDX_CACHE_OPTIONS := $(CACHE_FROM_OPTIONS) $(CACHE_TO_OPTIONS)

push: $(PUSH_TARGETS)
push-%: IMAGE_DISTRIBUTION = $(*)
$(PUSH_TARGETS): push-%:
	$(DOCKER) push "$(IMAGE_NAME):$(IMAGE_VERSION)-$(*)"

push-short:
	$(DOCKER) tag "$(IMAGE_NAME):$(IMAGE_VERSION)-$(DEFAULT_DISTRIBUTION)" "$(IMAGE_NAME):$(IMAGE_VERSION)"
	$(DOCKER) push "$(IMAGE_NAME):$(IMAGE_VERSION)"

$(DISTRIBUTIONS): %: build-%

build-%: IMAGE_DISTRIBUTION = $(*)
$(BUILD_TARGETS): build-%:
	$(DOCKER) build --pull \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg CUDA_VERSION=$(CUDA_VERSION) \
		--build-arg PLUGIN_VERSION=$(VERSION) \
		--build-arg BASE_DIST=$(IMAGE_DISTRIBUTION) \
		--tag $(IMAGE) \
		--file docker/Dockerfile \
			.

# Add multi-arch-builds using docker buildx
BUILD_MULTI_ARCH_TARGETS := $(patsubst %,build-multi-arch-%,$(DISTRIBUTIONS))
PUSH_MULTI_ARCH_TARGETS := $(patsubst %,push-multi-arch-%,$(DISTRIBUTIONS))
RELEASE_MULTI_ARCH_TARGETS := $(patsubst %,release-multi-arch-%,$(DISTRIBUTIONS))

MULTI_ARCH_TARGETS := $(BUILD_MULTI_ARCH_TARGETS) $(PUSH_MULTI_ARCH_TARGETS) $(RELEASE_MULTI_ARCH_TARGETS)
.PHONY: $(MULTI_ARCH_TARGETS)

BUILDX_PLATFORM_OPTIONS := --platform=linux/amd64,linux/arm64
BUILDX_PULL_OPTIONS := --pull
BUILDX_PUSH_ON_BUILD := false

# The build-multi-arch target uses docker buildx to produce a multi-arch image.
# This forms the basis of the push-, and release-multi-arch builds, with each
# of these setting the output and cache options.
build-multi-arch-%: IMAGE_DISTRIBUTION = $(*)
$(BUILD_MULTI_ARCH_TARGETS): build-multi-arch-%:
	$(DOCKER) $(BUILDX) build \
		$(BUILDX_PLATFORM_OPTIONS) \
		$(BUILDX_PULL_OPTIONS) \
		$(BUILDX_CACHE_OPTIONS) \
		--output=type=image,push=$(BUILDX_PUSH_ON_BUILD) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg CUDA_VERSION=$(CUDA_VERSION) \
		--build-arg PLUGIN_VERSION=$(IMAGE_VERSION) \
		--build-arg BASE_DIST=$(IMAGE_DISTRIBUTION) \
		--tag $(OUT_IMAGE) \
		--file docker/Dockerfile \
			.

push-multi-arch-%: BUILDX_PUSH_ON_BUILD := true
$(PUSH_MULTI_ARCH_TARGETS): push-multi-arch-%: build-multi-arch-%

release-multi-arch-%: BUILDX_PULL_OPTIONS :=
release-multi-arch-%: BUILDX_PUSH_ON_BUILD := true
release-multi-arch-%: IMAGE_DISTRIBUTION = $(*)
$(RELEASE_MULTI_ARCH_TARGETS): release-multi-arch-%: build-multi-arch-%

# For the default release target, we also push a short tag equal to the version.
# We skip this for the development release
DEVEL_RELEASE_IMAGE_VERSION ?= devel
ifneq ($(strip $(OUT_IMAGE_VERSION)),$(DEVEL_RELEASE_IMAGE_VERSION))
release-multi-arch-$(DEFAULT_DISTRIBUTION): release-multi-arch-with-version-tag
endif
.PHONY: release-multi-arch-with-version-tag

# We require that the build be completed first
release-multi-arch-with-version-tag: | build-multi-arch-$(DEFAULT_DISTRIBUTION)
	$(DOCKER) $(BUILDX) imagetools create \
		--tag "$(OUT_IMAGE_NAME):$(OUT_IMAGE_VERSION)" \
		$(OUT_IMAGE_NAME):$(OUT_IMAGE_VERSION)-$(DEFAULT_DISTRIBUTION)

# Define local and dockerized golang targets
MODULE := github.com/NVIDIA/k8s-device-plugin

BUILDIMAGE_TAG ?= golang$(GOLANG_VERSION)
BUILDIMAGE ?= $(IMAGE_NAME)-build:$(BUILDIMAGE_TAG)

CHECK_TARGETS := assert-fmt vet lint ineffassign misspell
MAKE_TARGETS := fmt build test check coverage $(CHECK_TARGETS)
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
unit-test: test
test: build
	go test -v -coverprofile=$(COVERAGE_FILE) $(MODULE)/...

coverage: test
	cat $(COVERAGE_FILE) | grep -v "_mock.go" > $(COVERAGE_FILE).no-mocks
	go tool cover -func=$(COVERAGE_FILE).no-mocks
