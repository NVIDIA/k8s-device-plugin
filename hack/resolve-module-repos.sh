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

# Resolves every module in vendor/modules.txt to its upstream repository and
# writes hack/module-repos.tsv.
#
# Needs network; run via 'make third-party-notices-repos'. Keyed by module and
# not by version: a repository normally does not move when a dependency is
# bumped, so this file survives bumps and changes only when a new module enters
# the tree. That is a convenience, not a guarantee — Task 5's content
# verification is what actually enforces correctness, and it fails loudly if a
# mapping has gone stale.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=hack/license-url-lib.sh disable=SC1091
source "${HERE}/license-url-lib.sh"

MODULES_TXT="${MODULES_TXT:-vendor/modules.txt}"
OUTPUT="${OUTPUT:-hack/module-repos.tsv}"
PROXY="${PROXY:-https://proxy.golang.org}"

die() {
    printf 'ERROR: %s\n' "$1" >&2
    shift
    (( $# > 0 )) && printf '%s\n' "$@" >&2
    exit 1
}
log() { printf '%s\n' "$*" >&2; }

# Body-as-string wrapper over the shared fetcher. An empty body is a failure
# here but is NOT retried: absence of Origin is a real, permanent property of
# older proxy cache entries, not transient flakiness.
fetch_retry() {
    local url="$1" body_tmp_file body
    body_tmp_file="$(mktemp "${TMPDIR:-/tmp}/k8s-device-plugin-fetch.XXXXXX")"
    if ! http_fetch_to_file "${url}" "${body_tmp_file}"; then
        rm -f "${body_tmp_file}"
        return 1
    fi
    body="$(cat "${body_tmp_file}")"
    rm -f "${body_tmp_file}"
    [[ -n "${body}" ]] || return 1
    printf '%s' "${body}"
}

origin_field() {
    printf '%s' "$1" | python3 -c '
import json, sys
try:
    origin = json.load(sys.stdin).get("Origin") or {}
except Exception:
    origin = {}
print(origin.get(sys.argv[1], ""))
' "$2" 2>/dev/null || printf ''
}

# go-import content is "<import-prefix> <vcs> <repo-root>". The meta tag is
# frequently split across lines, so newlines are folded before matching.
go_import_meta() {
    fetch_retry "https://$1?go-get=1" 2>/dev/null \
        | tr '\n' ' ' | tr -s ' ' \
        | LC_ALL=C grep -oE 'name="go-import"[^>]*content="[^"]*"' \
        | head -1 \
        | LC_ALL=C sed -E 's/.*content="([^"]*)".*/\1/'
}

main() {
    command -v curl >/dev/null 2>&1 || die "curl is not installed."
    command -v python3 >/dev/null 2>&1 || die "python3 is not installed."
    [[ -f "${MODULES_TXT}" ]] \
        || die "${MODULES_TXT} not found — run 'make third-party-notices-repos' from the repo root."

    local repos_tmp_file unresolved=0
    repos_tmp_file="$(mktemp "${TMPDIR:-/tmp}/k8s-device-plugin-repos.XXXXXX")"
    trap 'rm -f "${repos_tmp_file}"' EXIT

    local module version module_info_json repo import_prefix subdir go_import_meta_content converted_repo_url
    local unresolved_modules=""
    while read -r module version; do
        [[ -z "${module}" ]] && continue

        repo=""; import_prefix=""; subdir=""; module_info_json=""

        if module_info_json="$(fetch_retry "${PROXY}/$(proxy_escape "${module}")/@v/${version}.info")"; then
            repo="$(origin_field "${module_info_json}" URL)"
            subdir="$(origin_field "${module_info_json}" Subdir)"
        fi

        if [[ -z "${repo}" ]]; then
            go_import_meta_content="$(go_import_meta "${module}")" || go_import_meta_content=""
            if [[ -n "${go_import_meta_content}" ]]; then
                import_prefix="$(printf '%s' "${go_import_meta_content}" | awk '{print $1}')"
                repo="$(printf '%s' "${go_import_meta_content}" | awk '{print $3}')"
            fi
        fi

        [[ -z "${repo}" ]] && repo="$(github_repo_from_path "${module}")"
        repo="$(normalize_repo_url "${repo}")"

        # gopkg.in points at itself and serves no blobs.
        case "${repo}" in
            https://gopkg.in/*|"")
                converted_repo_url="$(gopkg_in_repo "${module}")"
                [[ -n "${converted_repo_url}" ]] && repo="${converted_repo_url}"
                ;;
        esac

        if [[ -z "${repo}" ]]; then
            log "UNRESOLVED ${module}: no repository could be determined"
            unresolved=$(( unresolved + 1 ))
            unresolved_modules="${unresolved_modules}${unresolved_modules:+ }${module}"
            continue
        fi

        # Subdir precedence: proxy Origin, then the go-import prefix, then the
        # github path shape. The last matters because GitHub serves no
        # go-import, so a github submodule with no Origin would otherwise lose
        # its tag prefix and never verify.
        if [[ -z "${subdir}" && -n "${import_prefix}" ]]; then
            subdir="$(derived_subdir "${module}" "${import_prefix}")"
        fi
        if [[ -z "${subdir}" ]]; then
            subdir="$(github_subdir_from_path "${module}")"
        fi

        printf '%s\t%s\t%s\n' "${module}" "${repo}" "${subdir}" >> "${repos_tmp_file}"
    done < <(LC_ALL=C grep '^# ' "${MODULES_TXT}" | awk '{print $2, $3}')

    # A warning, not a die: this resolves every module in modules.txt, including
    # the ten-odd build/test-only ones out of scope for the notices document, so
    # an unreachable vanity host on one of those must not block refreshing
    # notices for an unrelated shipped bump. Fail-closed is still preserved —
    # hack/verify-license-urls.sh dies when an IN-SCOPE module has no entry in
    # this map. Do not turn this back into a die without also scoping the loop
    # above to shipped modules only.
    if (( unresolved > 0 )); then
        log "WARNING: ${unresolved} module(s) could not be resolved to a repository: ${unresolved_modules}"
        log "Re-run; if the warning persists the module's vanity host is unreachable."
    fi

    {
        printf '# Upstream repository for each vendored module.\n'
        printf '# Generated by hack/resolve-module-repos.sh from the module proxy Origin,\n'
        printf '# the go-import meta tag, and the github.com path shape. Not hand-edited.\n'
        printf '# module\trepo-url\tsubdir\n'
        LC_ALL=C sort "${repos_tmp_file}"
    } > "${OUTPUT}"

    log "Wrote ${OUTPUT} ($(LC_ALL=C grep -vc '^#' "${OUTPUT}") modules)"

    # Exit here, not by falling off the end: the EXIT trap above references
    # repos_tmp_file, a variable local to this function. If main merely
    # returns, the process's implicit exit fires that trap after
    # repos_tmp_file has gone out of scope, and 'set -u' turns the cleanup
    # itself into an unbound-variable failure that clobbers this function's
    # success with exit 1.
    exit 0
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
