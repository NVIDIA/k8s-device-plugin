# Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

GO_CMD ?= go
GO_FMT ?= gofmt

# The default go tests timeout is 10 minutes which is too for this case.
GO_TEST_TIMEOUT ?= 30m

include $(CURDIR)/versions.mk

DRIVER_ENABLED ?= true

E2E_IMAGE_REPO ?= $(REGISTRY)/$(DRIVER_NAME)
E2E_IMAGE_TAG ?= $(VERSION)
E2E_IMAGE_PULL_POLICY ?= IfNotPresent
HELM_CHART ?= $(CURDIR)/deployments/helm/nvidia-device-plugin
LOG_ARTIFACTS ?= $(CURDIR)/e2e_logs

.PHONY: test
test:
	@if [ -z ${KUBECONFIG} ]; then \
		echo "[ERR] KUBECONFIG missing, must be defined"; \
		exit 1; \
	fi
	cd $(CURDIR)/tests/e2e && $(GO_CMD) test -timeout $(GO_TEST_TIMEOUT) -v . -args \
		-kubeconfig=$(KUBECONFIG) \
		-driver-enabled=$(DRIVER_ENABLED) \
		-image.repo=$(E2E_IMAGE_REPO) \
		-image.tag=$(E2E_IMAGE_TAG) \
		-image.pull-policy=$(E2E_IMAGE_PULL_POLICY) \
		-log-artifacts=$(LOG_ARTIFACTS) \
		-helm-chart=$(HELM_CHART) \
		-helm-log-file=$(LOG_ARTIFACTS)/helm.log \
		-ginkgo.focus="\[nvidia\]" \
		-test.timeout=1h \
		-ginkgo.v
