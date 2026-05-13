#!/usr/env bash

function remote() {
    ${SCRIPT_DIR}/remote.sh "cd ${PROJECT} && "$@""
}
