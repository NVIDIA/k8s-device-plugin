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
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

if [ -z "$1" ]; then
  VERSION=$(awk -F= '/^VERSION/ { print $2 }' versions.mk | tr -d '[:space:]')
else
  VERSION=$1
fi

PRERELEASE_FLAG=""
if [[ ${VERSION} == v*-rc.* ]]; then
    PRERELEASE_FLAG="--prerelease"
fi

REPOSITORY=NVIDIA/k8s-device-plugin

echo "Creating draft release"
./hack/generate-changelog.sh --version ${VERSION} --since ${REFERENCE} | \
    grep -v "### Version v" | \
        gh release create ${VERSION} --notes-file "-" \
                --draft \
                --title "${VERSION}" \
                -R "${REPOSITORY}" \
                --verify-tag \
                --prerelease

HELM_PACKAGE_VERSION=${VERSION#v}
echo "Uploading release artifacts"

# THIRD_PARTY_NOTICES.md is attached as a release asset rather than copied into
# the image: the notices may be distributed alongside the artifact, and this
# keeps the image size unchanged. It is committed and the notices job in CI
# proves it matches the tree, so it is uploaded as-is from the tagged commit.
if [ ! -s THIRD_PARTY_NOTICES.md ]; then
    echo "THIRD_PARTY_NOTICES.md is missing or empty at ${VERSION}" >&2
    exit 1
fi

gh release upload ${VERSION} \
    ./nvidia-device-plugin-${HELM_PACKAGE_VERSION}.tgz \
    ./gpu-feature-discovery-${HELM_PACKAGE_VERSION}.tgz \
    ./THIRD_PARTY_NOTICES.md \
    -R ${REPOSITORY}
