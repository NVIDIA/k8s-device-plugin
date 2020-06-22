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
REGISTRY ?= nvidia
VERSION  ?= v0.7.0-rc.1

##### Public rules #####

all: ubuntu16.04 centos7 ubi8

push:
	$(DOCKER) push "$(REGISTRY)/k8s-device-plugin:$(VERSION)-ubuntu16.04"
	$(DOCKER) push "$(REGISTRY)/k8s-device-plugin:$(VERSION)-centos7"
	$(DOCKER) push "$(REGISTRY)/k8s-device-plugin:$(VERSION)-ubi8"

push-short:
	$(DOCKER) tag "$(REGISTRY)/k8s-device-plugin:$(VERSION)-ubuntu16.04" "$(REGISTRY)/k8s-device-plugin:$(VERSION)"
	$(DOCKER) push "$(REGISTRY)/k8s-device-plugin:$(VERSION)"

push-latest:
	$(DOCKER) tag "$(REGISTRY)/k8s-device-plugin:$(VERSION)-ubuntu16.04" "$(REGISTRY)/k8s-device-plugin:latest"
	$(DOCKER) push "$(REGISTRY)/k8s-device-plugin:latest"

ubuntu16.04:
	$(DOCKER) build --pull \
		--tag $(REGISTRY)/k8s-device-plugin:$(VERSION)-ubuntu16.04 \
		--file docker/amd64/Dockerfile.ubuntu16.04 .

ubi8:
	$(DOCKER) build --pull \
		--build-arg PLUGIN_VERSION=$(VERSION) \
		--tag $(REGISTRY)/k8s-device-plugin:$(VERSION)-ubi8 \
		--file docker/amd64/Dockerfile.ubi8 .

centos7:
	$(DOCKER) build --pull \
		--tag $(REGISTRY)/k8s-device-plugin:$(VERSION)-centos7 \
		--file docker/amd64/Dockerfile.centos7 .

