#!/bin/bash
set -e

[[ -z "${DEBUG:-}" ]] || set -x

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TEST_DIR="$( cd "${SCRIPT_DIR}/.." && pwd )"
PROJECT_DIR="$( cd "${TEST_DIR}/.." && pwd )"

# Set default values if not defined
: ${PROJECT:="$(basename "${PROJECT_DIR}")"}
: ${GOLANG_VERSION:="1.26.3"}
: ${E2E_IMAGE_PULL_POLICY:="Always"}
: ${LOG_ARTIFACTS:=/tmp/logs}
: ${LOG_ARTIFACTS_DIR:="${LOG_ARTIFACTS}/e2e-logs"}
