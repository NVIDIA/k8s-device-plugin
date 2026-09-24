#!/bin/bash
# Setup a GitHub Actions self-hosted runner on a MicroShift host.
#
# Prerequisites:
#   - RHEL 10+ with MicroShift running
#   - kubectl / oc / helm available
#   - A GitHub personal access token (classic) with "repo" scope,
#     or a fine-grained token with "Administration" read/write on the repo
#
# Usage:
#   export GITHUB_TOKEN=ghp_...
#   ./hack/setup-github-runner.sh
#
# The runner registers with labels: self-hosted, linux, x64, microshift
# so workflows can target it via: runs-on: [self-hosted, microshift]

set -euo pipefail

REPO="arthur-r-oliveira/k8s-device-plugin"
RUNNER_DIR="${HOME}/actions-runner"
RUNNER_VERSION="2.325.0"
RUNNER_ARCH="linux-x64"
RUNNER_LABELS="self-hosted,linux,x64,microshift"

if [ -z "${GITHUB_TOKEN:-}" ]; then
  echo "Error: set GITHUB_TOKEN to a PAT with repo scope"
  exit 1
fi

echo "==> Checking prerequisites..."
for cmd in kubectl helm; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "Error: $cmd not found in PATH"
    exit 1
  fi
done

echo "==> MicroShift status:"
kubectl get nodes || { echo "Error: cannot reach MicroShift API"; exit 1; }

echo "==> Getting runner registration token..."
REG_TOKEN=$(curl -s -X POST \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  "https://api.github.com/repos/${REPO}/actions/runners/registration-token" \
  | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$REG_TOKEN" ]; then
  echo "Error: failed to get registration token. Check GITHUB_TOKEN permissions."
  exit 1
fi

if [ -d "$RUNNER_DIR" ]; then
  echo "==> Runner directory exists, checking for existing runner..."
  if [ -f "$RUNNER_DIR/.runner" ]; then
    echo "    Removing previous registration..."
    (cd "$RUNNER_DIR" && ./config.sh remove --token "$REG_TOKEN") || true
  fi
else
  echo "==> Downloading GitHub Actions runner ${RUNNER_VERSION}..."
  mkdir -p "$RUNNER_DIR"
  curl -sL "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-${RUNNER_ARCH}-${RUNNER_VERSION}.tar.gz" \
    | tar xz -C "$RUNNER_DIR"
fi

echo "==> Configuring runner..."
cd "$RUNNER_DIR"
./config.sh \
  --url "https://github.com/${REPO}" \
  --token "$REG_TOKEN" \
  --name "$(hostname)" \
  --labels "$RUNNER_LABELS" \
  --unattended \
  --replace

echo "==> Installing runner as systemd service..."
if [ "$(id -u)" -eq 0 ]; then
  ./svc.sh install
  ./svc.sh start
  echo "Runner service installed and started."
else
  echo ""
  echo "Run these commands as root to install the systemd service:"
  echo "  cd $RUNNER_DIR"
  echo "  sudo ./svc.sh install $(whoami)"
  echo "  sudo ./svc.sh start"
  echo ""
  echo "Or run interactively with: ./run.sh"
fi

echo ""
echo "==> Done. Runner '$(hostname)' registered for ${REPO}"
echo "    Labels: ${RUNNER_LABELS}"
echo "    Workflows can target it with: runs-on: [self-hosted, microshift]"
