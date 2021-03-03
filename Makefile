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
ifeq ($(IMAGE),)
REGISTRY ?= nvcr.io/nvidia
IMAGE := $(REGISTRY)/k8s-device-plugin
endif
VERSION  ?= v0.9.0

GOLANG_VERSION ?= 1.15.8

##### Public rules #####

DISTRIBUTIONS = ubuntu16.04 ubi8
DEFAULT_DISTRIBUTION := ubuntu16.04

BUILD_TARGETS := $(patsubst %,build-%,$(DISTRIBUTIONS))
PUSH_TARGETS := $(patsubst %,push-%,$(DISTRIBUTIONS))

.PHONY: $(DISTRIBUTIONS) $(BUILD_TARGETS) $(PUSH_TARGETS)

all: $(BUILD_TARGETS)

push: $(PUSH_TARGETS)
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
		--build-arg PLUGIN_VERSION=$(VERSION) \
		--tag $(IMAGE):$(VERSION)-$(DISTRIBUTION) \
		--file docker/amd64/Dockerfile.$(DISTRIBUTION) \
			.
