#!/bin/bash -e

# Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

set -o pipefail

this=`basename $0`

usage () {
cat << EOF
Usage: $this [-h]

Options:
  --version             specify the version for the release.
  --helm-repo-path      specify the path to the Helm repo (defaults to HELM_REPO_PATH)
                        If unspecified, the version is used to construct a path.
  --help/-h             show help for this command and exit

Example:
  - from a local path:
	$this /path/to/nvidia-device-plugin-{{ VERSION }}.tgz
  - from a URL:
    $this https://github.com/NVIDIA/k8s-device-plugin/archive/refs/tags/nvidia-device-plugin-{{ VERSION }}.tgz

EOF
}

#
# Parse command line
#
while [[ $# -gt 0 ]]; do
	key="$1"
	case $key in
		--helm-repo-path)
			HELM_REPO_PATH="$2"
			shift 2
			;;
		--version)
			VERSION=$2
			shift 2
			;;
		--help/h) usage
			exit 0
			;;
		*)
			break
			;;
	esac
done


if [[ -z "${HELM_REPO_PATH}" ]]; then
	if [[ -z "${VERSION}" ]]; then
		echo "Either helm repo path or version must be specified"
		usage
		exit 1
	else
		HELM_REPO_PATH="releases/${VERSION}"
	fi
fi


if [[ -n "${VERSION}" ]]; then
	if [[ $# -gt 0 ]]; then
		echo "If a version is specified, then no assets should be specified"
		usage
		exit 1
	fi
	gh release download ${VERSION} \
		--pattern '*.tgz' \
		--dir /tmp/${VERSION}-assets/ \
		--clobber
	asset_path="$(ls /tmp/${VERSION}-assets/*.tgz)"
else
	# now we take the input from the user and check if is a path or url http/https
	asset_path="$@"
fi

if [ -z "$asset_path" ]; then
	echo "No assets provided"
	usage
	exit 1
fi

if [[ $asset_path =~ ^https?:// ]]; then
	asset_urls=$asset_path
else
	asset_local=$asset_path
fi

GH_REPO_PULL_URL="https://github.com/NVIDIA/k8s-device-plugin.git"
git clone --depth=1 --branch=gh-pages ${GH_REPO_PULL_URL} ${HELM_REPO_PATH}
mkdir -p ${HELM_REPO_PATH}/stable

# Charts are local, no need to download
if [ -n "$asset_local" ]; then
	echo "Copying $asset_local..."
	cp -f $asset_local $HELM_REPO_PATH/stable
else
	for asset_url in $asset_urls; do
		if ! echo "$asset_url" | grep -q '.*\.tgz$'; then
			echo "Skipping $asset_url, does not look like a Helm chart archive"
			continue
		fi
		echo "Downloading $asset_url..."
		curl -sSfLO --output-dir "${HELM_REPO_PATH}/stable" ${asset_url}
	done
fi

echo "Updating helm index"
helm repo index $HELM_REPO_PATH/stable --merge $HELM_REPO_PATH/stable/index.yaml --url https://nvidia.github.io/k8s-device-plugin/stable
cp -f $HELM_REPO_PATH/stable/index.yaml $HELM_REPO_PATH/index.yaml

changes=$( git -C $HELM_REPO_PATH status --short )

# Check if there were any changes in the repo
if [ -z "${changes}" ]; then
    echo "No changes in Helm repo index, gh-pages branch already up-to-date"
    exit 0
fi

if [[ -z ${VERSION} ]]; then
	VERSION=$( echo "${changes}" | grep -v index | grep -oE "\-[0-9\.]+(\-[\-\.rc0-9]+)?.tgz" | sort -u )
	VERSION=${VERSION#-}
	VERSION=${VERSION%.tgz}

	if [ -z "${VERSION}" ]; then
		echo "Could not extract version information"
		exit 1
	fi

	VERSION="v$VERSION"
fi

# Create a new commit
echo "Committing changes: \n${changes}"
git -C $HELM_REPO_PATH add index.yaml stable

cat <<EOF | git -C $HELM_REPO_PATH commit --signoff -F -
Add packages for ${VERSION} release

This adds the following packages to the NVIDIA GPU Device Plugin Helm repo:
$( git -C $HELM_REPO_PATH diff HEAD --name-only | grep -v index | sed 's#stable/#* #g' | sort -r )

Note: This is an automated commit.

EOF

echo "gh-pages branch successfully updated"
