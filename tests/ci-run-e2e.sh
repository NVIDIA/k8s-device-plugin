#!/bin/bash

set -xe

if [[ $# -ne 3 ]]; then
	echo "E2E_IMAGE_REPO, E2E_IMAGE_TAG, NVIDIA_DRIVER_ENABLED are required"
	exit 1
fi

export E2E_IMAGE_REPO=${1}
export E2E_IMAGE_TAG=${2}
export NVIDIA_DRIVER_ENABLED=${3}

TEST_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

${TEST_DIR}/local.sh
