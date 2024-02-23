#!/usr/bin/env bash

# Copyright 2023 The Kubernetes Authors.
# Copyright 2023 NVIDIA CORPORATION.
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

# A reference to the current directory where this script is located
SCRIPTS_DIR="$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"
PROJECT_DIR="$(cd -- "$( dirname -- "${SCRIPTS_DIR}/../../../../.." )" &> /dev/null && pwd)"

# We extract information from versions.mk
function from_versions_mk() {
    local makevar=$1
    local value=$(grep -E "^\s*${makevar}\s+[\?:]= " ${PROJECT_DIR}/versions.mk)
    echo ${value##*= }
}
DRIVER_NAME=$(from_versions_mk "DRIVER_NAME")
DRIVER_IMAGE_REGISTRY=$(from_versions_mk "REGISTRY")
DRIVER_IMAGE_VERSION=$(from_versions_mk "VERSION")

: ${DRIVER_IMAGE_NAME:=${DRIVER_NAME}}
: ${DRIVER_IMAGE_PLATFORM:="ubuntu20.04"}
: ${DRIVER_IMAGE_TAG:=${DRIVER_IMAGE_VERSION}}
# The derived name of the driver image to build
: ${DRIVER_IMAGE:="${DRIVER_IMAGE_REGISTRY}/${DRIVER_IMAGE_NAME}:${DRIVER_IMAGE_TAG}"}

# The kubernetes tag to build the kind cluster from
# From https://github.com/kubernetes/kubernetes/tags
# See also https://hub.docker.com/r/kindest/node/tags
: ${KIND_K8S_TAG:="v1.29.1"}

# The name of the kind cluster to create
: ${KIND_CLUSTER_NAME:="${DRIVER_NAME}-cluster"}

# The path to kind's cluster configuration file
: ${KIND_CLUSTER_CONFIG_PATH:="${SCRIPTS_DIR}/kind-cluster-config.yaml"}

# The kind image to use. This image will be built if it is not available.
: ${KIND_IMAGE:="kindest/node:${KIND_K8S_TAG}"}

