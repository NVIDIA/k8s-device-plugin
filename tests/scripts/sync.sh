#!/bin/bash

if [[ "${SKIP_SYNC}" == "true" ]]; then
    echo "Skipping sync: SKIP_SYNC=${SKIP_SYNC}"
    exit 0
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source ${SCRIPT_DIR}/.definitions.sh

source ${SCRIPT_DIR}/.local.sh

rsync -e "ssh -i ${private_key} -o StrictHostKeyChecking=no" \
    -avz --delete \
    --exclude-from="${SCRIPT_DIR}/.rsync-excludes" \
        ${@}
