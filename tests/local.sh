#! /bin/bash

export TEST_JOB="make -f tests/e2e/Makefile test"
export PROJECT="k8s-device-plugin"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/scripts && pwd )"
source ${SCRIPT_DIR}/.definitions.sh
source ${SCRIPT_DIR}/.local.sh

# Sync the project folder to the remote
${SCRIPT_DIR}/push.sh

# We trigger the installation of prerequisites on the remote instance
remote \
    SKIP_PREREQUISITES="${SKIP_PREREQUISITES}" \
    GOLANG_VERSION="${GOLANG_VERSION}" \
       ./tests/scripts/prerequisites.sh

# We trigger the specified test case on the remote instance.
# Note: We need to ensure that the required environment variables
# are forwarded to the remote shell.
remote \
    PROJECT="${PROJECT}" \
    HELM_CHART="~/${PROJECT}/deployments/helm/nvidia-device-plugin" \
    E2E_IMAGE_REPO=${E2E_IMAGE_REPO} \
    E2E_IMAGE_TAG="${E2E_IMAGE_TAG}" \
    E2E_IMAGE_PULL_POLICY="${E2E_IMAGE_PULL_POLICY}" \
    NVIDIA_DRIVER_ENABLED="${NVIDIA_DRIVER_ENABLED}" \
    LOG_ARTIFACTS="${LOG_ARTIFACTS}" \
    LOG_ARTIFACTS_DIR="${LOG_ARTIFACTS_DIR}" \
        ${TEST_JOB}
