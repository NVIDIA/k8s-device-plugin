# Copyright (c) 2020, NVIDIA CORPORATION.  All rights reserved.
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
REGISTRY ?= nvidia
IMAGE := $(REGISTRY)/k8s-device-plugin
endif
VERSION  ?= v0.7.0

##### Public rules #####

all: ubuntu16.04 centos7 ubi8

push:
	$(DOCKER) push "$(IMAGE):$(VERSION)-ubuntu16.04"
	$(DOCKER) push "$(IMAGE):$(VERSION)-centos7"
	$(DOCKER) push "$(IMAGE):$(VERSION)-ubi8"

push-short:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-ubuntu16.04" "$(IMAGE):$(VERSION)"
	$(DOCKER) push "$(IMAGE):$(VERSION)"

push-latest:
	$(DOCKER) tag "$(IMAGE):$(VERSION)-ubuntu16.04" "$(IMAGE):latest"
	$(DOCKER) push "$(IMAGE):latest"

ubuntu16.04:
	$(DOCKER) build --pull \
		--tag $(IMAGE):$(VERSION)-ubuntu16.04 \
		--file docker/amd64/Dockerfile.ubuntu16.04 .

ubi8:
	$(DOCKER) build --pull \
		--build-arg PLUGIN_VERSION=$(VERSION) \
		--tag $(IMAGE):$(VERSION)-ubi8 \
		--file docker/amd64/Dockerfile.ubi8 .

centos7:
	$(DOCKER) build --pull \
		--tag $(IMAGE):$(VERSION)-centos7 \
		--file docker/amd64/Dockerfile.centos7 .

