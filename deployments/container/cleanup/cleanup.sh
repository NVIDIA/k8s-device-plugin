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

echo "install_weak_deps=False" >> /etc/dnf/dnf.conf
rm -f /etc/dnf/protected.d/*.conf

rm -f /etc/yum.repos.d/cuda.repo
rm -f /etc/ld.so.conf.d/nvidia.conf

dnf remove -y \
    cuda* \
    systemd

dnf clean all
rm -rf /var/cache/dnf

dnf install -y microdnf

microdnf remove $(rpm -q --whatrequires dnf)
rpm -e dnf

microdnf remove \
    $(rpm -q --whatrequires /usr/libexec/platform-python) \
    $(rpm -q --whatrequires 'python(abi)') \
    python* \
    dnf*

microdnf remove \
    $(rpm -qa | sort | grep -v -f minimal-list.txt -e gpg-pubkey)

microdnf update
microdnf clean all

rpm -e microdnf
rpm -qa | sort -u > package-list.versions

rm -rf /var/cache/dnf
