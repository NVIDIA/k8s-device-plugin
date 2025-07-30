# SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved. 
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

# Test configuration
GINKGO_PARALLEL_PROCS ?= 4
GINKGO_FLAKE_ATTEMPTS ?= 2
GINKGO_COMMON_ARGS := -v --fail-on-pending --randomize-all --trace
GINKGO_REPORT_ARGS := --json-report=$(LOG_ARTIFACTS)/report.json --junit-report=$(LOG_ARTIFACTS)/junit.xml

.PHONY: ginkgo e2e-test e2e-test-parallel e2e-test-serial e2e-test-all clean-artifacts

ginkgo:
	mkdir -p $(CURDIR)/bin
	GOBIN=$(CURDIR)/bin go install github.com/onsi/ginkgo/v2/ginkgo@latest

# Create artifacts directory
$(LOG_ARTIFACTS):
	mkdir -p $(LOG_ARTIFACTS)

# Clean artifacts
clean-artifacts:
	rm -rf $(LOG_ARTIFACTS)

# Run all tests (default)
test-e2e: ginkgo $(LOG_ARTIFACTS)
	$(CURDIR)/bin/ginkgo $(GINKGO_COMMON_ARGS) $(GINKGO_REPORT_ARGS) $(GINKGO_ARGS) ./tests/e2e/...

# Run only parallel-safe tests
test-e2e-parallel: ginkgo $(LOG_ARTIFACTS)
	$(CURDIR)/bin/ginkgo $(GINKGO_COMMON_ARGS) $(GINKGO_REPORT_ARGS) \
		--procs=$(GINKGO_PARALLEL_PROCS) \
		--label-filter='!serial' \
		$(GINKGO_ARGS) ./tests/e2e/...

# Run only serial tests (cluster-mutating)
test-e2e-serial: ginkgo $(LOG_ARTIFACTS)
	$(CURDIR)/bin/ginkgo $(GINKGO_COMMON_ARGS) $(GINKGO_REPORT_ARGS) \
		--label-filter='serial' \
		$(GINKGO_ARGS) ./tests/e2e/...

# Run parallel tests first, then serial tests
test-e2e-all: test-e2e-parallel test-e2e-serial

# Run tests with flake attempts
test-e2e-flaky: ginkgo $(LOG_ARTIFACTS)
	$(CURDIR)/bin/ginkgo $(GINKGO_COMMON_ARGS) $(GINKGO_REPORT_ARGS) \
		--flake-attempts=$(GINKGO_FLAKE_ATTEMPTS) \
		$(GINKGO_ARGS) ./tests/e2e/...

# Run specific test by label
test-e2e-label: ginkgo $(LOG_ARTIFACTS)
	$(CURDIR)/bin/ginkgo $(GINKGO_COMMON_ARGS) $(GINKGO_REPORT_ARGS) \
		--label-filter='$(LABEL)' \
		$(GINKGO_ARGS) ./tests/e2e/...

# Run tests and generate coverage
test-e2e-coverage: ginkgo $(LOG_ARTIFACTS)
	$(CURDIR)/bin/ginkgo $(GINKGO_COMMON_ARGS) $(GINKGO_REPORT_ARGS) \
		--cover --coverprofile=$(LOG_ARTIFACTS)/coverage.out \
		$(GINKGO_ARGS) ./tests/e2e/...
