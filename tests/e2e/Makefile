# Copyright (c) 2023, NVIDIA CORPORATION. All rights reserved.
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

NVIDIA_DRIVER_ENABLED ?= true

E2E_IMAGE_REPO ?= $(REGISTRY)/$(DRIVER_NAME)
E2E_IMAGE_TAG ?= $(VERSION)
E2E_IMAGE_PULL_POLICY ?= IfNotPresent
HELM_CHART ?= $(CURDIR)/deployments/helm/nvidia-device-plugin
LOG_ARTIFACTS ?= $(CURDIR)/e2e_logs

# Test configuration
GINKGO_COMMON_ARGS := -v --fail-on-pending --randomize-all --trace
GINKGO_REPORT_ARGS := --json-report=$(LOG_ARTIFACTS)/report.json --junit-report=$(LOG_ARTIFACTS)/junit.xml

.PHONY: ginkgo test clean-artifacts

GINKGO_VERSION = $(shell grep -Eo "github.com/onsi/ginkgo/v2.*$$" ./tests/go.mod | sed -e 's&github.com/onsi/ginkgo/v2[[:space:]]&&g')
ginkgo:
	mkdir -p $(CURDIR)/bin
	GOBIN=$(CURDIR)/bin go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

# Create artifacts directory
$(LOG_ARTIFACTS):
	mkdir -p $(LOG_ARTIFACTS)

# Clean artifacts
clean-artifacts:
	rm -rf $(LOG_ARTIFACTS)

# Run tests
test: ginkgo $(LOG_ARTIFACTS)
	$(CURDIR)/bin/ginkgo $(GINKGO_COMMON_ARGS) $(GINKGO_REPORT_ARGS) $(GINKGO_ARGS) ./tests/e2e/...
