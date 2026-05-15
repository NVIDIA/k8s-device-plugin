#!/bin/bash

if [[ "${SKIP_PREREQUISITES}" == "true" ]]; then
    echo "Skipping prerequisites: SKIP_PREREQUISITES=${SKIP_PREREQUISITES}"
    exit 0
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}"/.definitions.sh

echo "Create log dir ${LOG_ARTIFACTS_DIR}"
mkdir -p "${LOG_ARTIFACTS_DIR}"

export DEBIAN_FRONTEND=noninteractive

echo "Install dependencies"
sudo apt update && sudo apt install -y make

sudo rm -rf /usr/local/go
arch="$(uname -m)"
case "${arch##*-}" in \
    x86_64 | amd64) ARCH='amd64' ;; \
    ppc64el | ppc64le) ARCH='ppc64le' ;; \
    aarch64 | arm64) ARCH='arm64' ;; \
    *) echo "unsupported architecture" ; exit 1 ;; \
esac;
wget -nv -O - https://go.dev/dl/go${GOLANG_VERSION}.linux-${ARCH}.tar.gz | sudo tar -C /usr/local -xz

# Non-interactive SSH sessions do not source ~/.bashrc; put go on the default PATH.
sudo ln -sf /usr/local/go/bin/go /usr/local/bin/go
