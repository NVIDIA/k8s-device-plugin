#!/bin/bash
# Copyright 2024 NVIDIA CORPORATION
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express orimplied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -x

rpm -qa | sort -u > package-list.original

echo "install_weak_deps=False" >> /etc/dnf/dnf.conf
rm -f /etc/dnf/protected.d/*.conf

rm -f /etc/yum.repos.d/cuda.repo
rm -f /etc/ld.so.conf.d/nvidia.conf

dnf remove -y \
    cuda* \
    systemd

# Remove the CUDA public key
for key in $(rpm -qa gpg-pubkey*); do
    rpm -qi ${key} | grep -o "cudatools <cudatools@nvidia.com>"
    if [[ $? -eq 0 ]]; then
        rpm -e ${key}
    fi
done

dnf clean -y all
rm -rf /var/cache/dnf

dnf install -y microdnf

microdnf remove -y $(rpm -q --whatrequires dnf)
rpm -e dnf

microdnf remove -y \
    $(rpm -q --whatrequires /usr/libexec/platform-python) \
    $(rpm -q --whatrequires 'python(abi)') \
    python* \
    dnf*

microdnf remove -y \
    $(rpm -qa | sort | grep -v -f package-names.minimal -e gpg-pubkey)

# We don't want to add third-party content to the base image and only remove packages.
# We therefore skip running microdnf update here
# microdnf update

microdnf clean all
rpm -e microdnf libdnf libpeas
rm -rf /var/lib/dnf

set +x
rpm -qa | sort -u > package-list.cleaned
for p in $(rpm -qa | sort -u); do
    echo "START $p" >> package-list.cleaned.info
    echo "INFO" >> package-list.cleaned.info
    rpm -qi $p >> package-list.cleaned.info
    echo "REQUIRES" >> package-list.cleaned.info
    rpm -qR $p >> package-list.cleaned.info
    echo "END $p" >>  package-list.cleaned.info
done

rm -rf /var/cache/dnf
