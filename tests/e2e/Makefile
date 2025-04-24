# SPDX-FileCopyrightText: Copyright (c) 2023 NVIDIA CORPORATION & AFFILIATES. All rights reserved. 
# SPDX-License-Identifier: Apache-2.0
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

.PHONY: ginkgo e2e-test
ginkgo:
	mkdir -p $(CURDIR)/bin
	GOBIN=$(CURDIR)/bin go install github.com/onsi/ginkgo/v2/ginkgo@latest

test-e2e: ginkgo
	$(CURDIR)/bin/ginkgo $(GINKGO_ARGS) -v --json-report ginkgo.json ./tests/e2e/...
