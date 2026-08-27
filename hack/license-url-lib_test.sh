#!/usr/bin/env bash
# Copyright (c) NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=hack/test-helpers.sh disable=SC1091
source "${HERE}/test-helpers.sh"
# shellcheck source=hack/license-url-lib.sh disable=SC1091
source "${HERE}/license-url-lib.sh"

# The bug that silently broke proxy resolution for all four uppercase modules.
assert_eq "github.com/!n!v!i!d!i!a/go-nvml" \
    "$(proxy_escape github.com/NVIDIA/go-nvml)" "proxy_escape lowercases with a bang"
assert_eq "github.com/!n!v!i!d!i!a/nvidia-container-toolkit" \
    "$(proxy_escape github.com/NVIDIA/nvidia-container-toolkit)" \
    "proxy_escape leaves an identically spelled lowercase segment alone"
assert_eq "k8s.io/api" "$(proxy_escape k8s.io/api)" "proxy_escape leaves lowercase alone"

assert_eq "github.com/urfave/cli" \
    "$(strip_major_suffix github.com/urfave/cli/v2)" "strip /v2"
assert_eq "go.yaml.in/yaml" \
    "$(strip_major_suffix go.yaml.in/yaml/v3)" "strip /v3 from a vanity path"
assert_eq "gopkg.in/inf.v0" \
    "$(strip_major_suffix gopkg.in/inf.v0)" "gopkg.in .vN is not a /vN suffix"

assert_eq "v2.0.1" "$(normalize_version 'v2.0.1+incompatible')" "strip +incompatible"
assert_eq "43fb72c5454a" \
    "$(pseudo_version_hash v0.0.0-20260317180543-43fb72c5454a)" "pseudo-version hash"
assert_eq "d8f796af33cc" \
    "$(pseudo_version_hash v1.1.2-0.20180830191138-d8f796af33cc)" "pre-release pseudo-version"
assert_eq "" "$(pseudo_version_hash v1.10.0)" "a tagged version has no hash"

assert_eq "https://github.com/sirupsen/logrus" \
    "$(github_repo_from_path github.com/sirupsen/logrus)" "github root module"
assert_eq "https://github.com/moby/sys" \
    "$(github_repo_from_path github.com/moby/sys/capability)" "github submodule"
assert_eq "" "$(github_repo_from_path k8s.io/api)" "non-github yields empty"

# Without this the github fallback loses the submodule tag prefix entirely.
assert_eq "capability" \
    "$(github_subdir_from_path github.com/moby/sys/capability)" "github subdir"
assert_eq "internal/spew" \
    "$(github_subdir_from_path github.com/stretchr/testify/internal/spew)" \
    "multi-segment github subdir"
assert_eq "" "$(github_subdir_from_path github.com/urfave/cli/v2)" "/vN is not a subdir"
assert_eq "" "$(github_subdir_from_path github.com/sirupsen/logrus)" "root module has no subdir"

assert_eq "https://github.com/go-inf/inf" "$(gopkg_in_repo gopkg.in/inf.v0)" "gopkg.in single segment"
assert_eq "https://github.com/go-yaml/yaml" "$(gopkg_in_repo gopkg.in/yaml.v3)" "gopkg.in yaml"
assert_eq "https://github.com/evanphx/json-patch" \
    "$(gopkg_in_repo gopkg.in/evanphx/json-patch.v4)" "gopkg.in user/pkg"
assert_eq "" "$(gopkg_in_repo github.com/foo/bar)" "non-gopkg.in yields empty"

assert_eq "https://github.com/cyphar/go-pathrs" \
    "$(normalize_repo_url 'https://github.com/cyphar/go-pathrs.git')" "strip .git"
assert_eq "https://github.com/foo/bar" \
    "$(normalize_repo_url 'https://github.com/foo/bar/')" "strip trailing slash"

assert_eq "api/nfd" \
    "$(derived_subdir sigs.k8s.io/node-feature-discovery/api/nfd sigs.k8s.io/node-feature-discovery)" \
    "subdir from prefix"
assert_eq "" "$(derived_subdir github.com/sirupsen/logrus github.com/sirupsen/logrus)" "root module"

assert_eq "https://github.com/sirupsen/logrus/blob/v1.10.0/LICENSE" \
    "$(blob_url https://github.com/sirupsen/logrus v1.10.0 LICENSE)" "github blob template"
assert_eq "https://go.googlesource.com/net/+/refs/tags/v0.56.0/LICENSE" \
    "$(blob_url https://go.googlesource.com/net refs/tags/v0.56.0 LICENSE)" "gerrit blob template"

assert_eq "https://raw.githubusercontent.com/stretchr/testify/v1.12.0/internal/spew/LICENSE" \
    "$(raw_url_for https://github.com/stretchr/testify/blob/v1.12.0/internal/spew/LICENSE)" \
    "github raw URL"
assert_eq "https://go.googlesource.com/net/+/refs/tags/v0.56.0/LICENSE?format=TEXT" \
    "$(raw_url_for https://go.googlesource.com/net/+/refs/tags/v0.56.0/LICENSE)" "gerrit raw URL"
assert_fails "github raw is not base64" raw_is_base64 https://github.com/a/b/blob/v1/LICENSE

assert_eq "hello" "$(printf 'aGVsbG8=' | base64_decode)" "base64_decode works on this host"

finish
