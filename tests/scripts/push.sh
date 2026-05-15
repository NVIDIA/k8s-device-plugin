#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source ${SCRIPT_DIR}/.definitions.sh
source ${SCRIPT_DIR}/.local.sh

REMOTE_PROJECT_FOLDER="~/${PROJECT}"

# Copy over the contents of the project folder
${SCRIPT_DIR}/sync.sh \
        "${PROJECT_DIR}/" \
        "${instance_hostname}:${REMOTE_PROJECT_FOLDER}"
