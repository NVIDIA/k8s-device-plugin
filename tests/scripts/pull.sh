#!/bin/bash

if [[ $# -ne 2 ]]; then
    echo "Pull requires a source and destination"
    exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source ${SCRIPT_DIR}/.definitions.sh
source ${SCRIPT_DIR}/.local.sh

${SCRIPT_DIR}/sync.sh ${instance_hostname}:${1} ${2}
