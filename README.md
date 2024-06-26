# NVIDIA device plugin for Kubernetes

> gh-pages branch

This branch is automatically updated by GitHub actions.
To manually update, please use the scriptd located  on the `main` or `release-*` branches.

Example:

```bash
VERSION=v0.15.0
DOWNLOAD_URL=<github release assets url>
# Update the Helm package index on the `gh-pages`
./hack/update-helm-index.sh --version {{ .VERSION }}
#Push the change to the `gh-pages` branch
git -C releases/{{ .VERSION }} remote set-url origin git@github.com:NVIDIA/k8s-device-plugin.git
git -C releases/{{ .VERSION }} push origin gh-pages
```
