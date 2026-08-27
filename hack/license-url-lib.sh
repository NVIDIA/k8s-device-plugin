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

# Pure string transforms shared by the resolvers and the notices generator.
# No network and (except base64_decode, which reads stdin) no I/O, so every
# rule here is testable offline.

# The module proxy case-encodes an uppercase letter as '!' plus its lowercase
# form: github.com/NVIDIA -> github.com/!n!v!i!d!i!a. This MUST NOT be done with
# sed: 's/\([A-Z]\)/!\l\1/g' yields '!lN!lV...' on BSD sed, which the proxy
# rejects as an invalid escaped module path.
proxy_escape() {
    printf '%s' "$1" | awk '{
        n = split($0, chars, "")
        out = ""
        for (i = 1; i <= n; i++) {
            c = chars[i]
            out = out (c ~ /[A-Z]/ ? "!" tolower(c) : c)
        }
        print out
    }'
}

strip_major_suffix() {
    if [[ "$1" =~ ^(.*)/v[0-9]+$ ]]; then
        printf '%s' "${BASH_REMATCH[1]}"
    else
        printf '%s' "$1"
    fi
}

normalize_version() {
    printf '%s' "${1%+incompatible}"
}

# A pseudo-version ends in <14-digit UTC timestamp>-<12-hex commit>. That
# trailing hash is the only ref such a module has; there is no tag.
# Pre-release versions have an optional 0. prefix before the timestamp.
pseudo_version_hash() {
    if [[ "$1" =~ -[0-9.]*[0-9]{14}-([0-9a-f]{12})$ ]]; then
        printf '%s' "${BASH_REMATCH[1]}"
    fi
}

normalize_repo_url() {
    local url="${1%/}"
    printf '%s' "${url%.git}"
}

github_repo_from_path() {
    local module rest org repo
    module="$(strip_major_suffix "$1")"
    case "${module}" in github.com/*) ;; *) return 0 ;; esac
    rest="${module#github.com/}"
    org="${rest%%/*}"
    rest="${rest#*/}"
    repo="${rest%%/*}"
    [[ -n "${org}" && -n "${repo}" && "${org}" != "${module}" ]] || return 0
    printf 'https://github.com/%s/%s' "${org}" "${repo}"
}

# The module's directory inside a github repository, derived from the path
# alone. GitHub serves no go-import meta, so when the proxy has no Origin this
# is the only source of the submodule tag prefix; without it a module such as
# github.com/Mellanox/maintenance-operator/api loses its 'api/' tag and no
# candidate URL can match.
github_subdir_from_path() {
    local module rest
    module="$(strip_major_suffix "$1")"
    case "${module}" in github.com/*) ;; *) return 0 ;; esac
    rest="${module#github.com/}"
    [[ "${rest}" == */* ]] || return 0
    rest="${rest#*/}"                 # drop org
    [[ "${rest}" == */* ]] || return 0
    printf '%s' "${rest#*/}"          # drop repo
}

# gopkg.in publishes a go-import pointing at itself, which serves no blobs.
# Its documented convention maps onto GitHub.
gopkg_in_repo() {
    local rest user pkg
    case "$1" in gopkg.in/*) ;; *) return 0 ;; esac
    rest="${1#gopkg.in/}"
    if [[ "${rest}" == */* ]]; then
        user="${rest%%/*}"
        pkg="${rest#*/}"
        printf 'https://github.com/%s/%s' "${user}" "${pkg%.v*}"
    else
        pkg="${rest%.v*}"
        printf 'https://github.com/go-%s/%s' "${pkg}" "${pkg}"
    fi
}

derived_subdir() {
    local module prefix
    module="$(strip_major_suffix "$1")"
    prefix="$(strip_major_suffix "$2")"
    [[ "${module}" == "${prefix}" ]] && return 0
    [[ "${module}" == "${prefix}/"* ]] || return 0
    printf '%s' "${module#"${prefix}/"}"
}

# Gerrit serves blobs under /+/<ref>/, not /blob/<ref>/.
blob_url() {
    local repo="$1" ref="$2" path="$3"
    case "${repo}" in
        https://go.googlesource.com/*) printf '%s/+/%s/%s' "${repo}" "${ref}" "${path}" ;;
        *)                             printf '%s/blob/%s/%s' "${repo}" "${ref}" "${path}" ;;
    esac
}

raw_url_for() {
    local blob="$1" rest owner repo
    case "${blob}" in
        https://github.com/*)
            rest="${blob#https://github.com/}"
            owner="${rest%%/*}"; rest="${rest#*/}"
            repo="${rest%%/*}";  rest="${rest#*/}"
            rest="${rest#blob/}"
            printf 'https://raw.githubusercontent.com/%s/%s/%s' "${owner}" "${repo}" "${rest}"
            ;;
        https://go.googlesource.com/*)
            printf '%s?format=TEXT' "${blob}"
            ;;
    esac
}

raw_is_base64() {
    case "$1" in https://go.googlesource.com/*) return 0 ;; *) return 1 ;; esac
}

# GNU coreutils spells the decode flag -d; BSD documents -D. Probe once rather
# than assuming, or every Gerrit-hosted module fails to hash on a strict BSD.
base64_decode() {
    if [[ -z "${BASE64_DECODE_FLAG:-}" ]]; then
        if printf '' | base64 -d >/dev/null 2>&1; then
            BASE64_DECODE_FLAG="-d"
        else
            BASE64_DECODE_FLAG="-D"
        fi
    fi
    base64 "${BASE64_DECODE_FLAG}"
}
