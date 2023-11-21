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

DRIVER_ENABLED ?= true

E2E_IMAGE_REPO ?= nvcr.io/nvidia/k8s-device-plugin
E2E_IMAGE_TAG ?= v0.14.3
E2E_IMAGE_PULL_POLICY ?= Always

HELM_CHART ?= $(CURDIR)/../../deployments/helm/nvidia-device-plugin

.PHONY: e2e-test
e2e-test:
	@if [ -z ${KUBECONFIG} ]; then \
		echo "[ERR] KUBECONFIG missing, must be defined"; \
		exit 1; \
	fi
	$(GO_CMD) test -v . -args \
		-kubeconfig=$(KUBECONFIG) \
		-driver-enabled=$(DRIVER_ENABLED) \
		-image.repo=$(E2E_IMAGE_REPO) \
		-image.tag=$(E2E_IMAGE_TAG) \
		-image.pull-policy=$(E2E_IMAGE_PULL_POLICY) \
		-helm-chart=$(HELM_CHART) \
		-ginkgo.focus="\[nvidia\]" \
		-test.timeout=1h
