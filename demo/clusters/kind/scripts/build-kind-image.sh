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
CURRENT_DIR="$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)"

set -ex
set -o pipefail

source "${CURRENT_DIR}/common.sh"

# If an image ID already exists for the image we plan to build, we are done.
EXISTING_IMAGE_ID="$(docker images --filter "reference=${KIND_IMAGE}" -q)"
if [ "${EXISTING_IMAGE_ID}" != "" ]; then
	exit 0
fi

PREBUILT_IMAGE="$( ( docker manifest inspect ${KIND_IMAGE} > /dev/null && echo "available" ) || ( echo "not-available" ) )"
if [ "${PREBUILT_IMAGE}" = "available" ] && [ "${KIND_IMAGE_BASE}" = "" ]; then
    exit 0
fi

# Create a temorary directory to hold all the artifacts we need for building the image
TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

# Set some build variables
KIND_K8S_REPO="https://github.com/kubernetes/kubernetes.git "
KIND_K8S_DIR="${TMP_DIR}/kubernetes-${KIND_K8S_TAG}"

# Checkout the version of kubernetes we want to build our kind image from
git clone --depth 1 --branch ${KIND_K8S_TAG} ${KIND_K8S_REPO} ${KIND_K8S_DIR}

# Build the kind node image.
# The default base image will be used unless it is specified using the
# KIND_BASE_IMAGE environment variable.
# This could be needed if new container runtime features are required.
# Examples are:
#   gcr.io/k8s-staging-kind/base:v20240213-749005b2
#   docker.io/kindest/base:v20240202-8f1494ea
kind build node-image \
    ${KIND_IMAGE_BASE:+--base-image "${KIND_IMAGE_BASE}"} \
    --image "${KIND_IMAGE}" \
    "${KIND_K8S_DIR}"
