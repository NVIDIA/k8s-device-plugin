# Release Process

The device plugin consists in two artifacts:
- The Device Plugin container
- The Device Plugin helm chart

Publishing the container is automated through gitlab-ci and only requires one to tag the commit and push it to gitlab.
Publishing the helm chart is currently manual, and we should move to an automated process ASAP

# Release Process Checklist
- [ ] Update the README changelog
- [ ] Update the README to change occurances of the old version (e.g: `v0.15.0-rc.1`) with the new version
- [ ] Commit, Tag and Push to Gitlab
- [ ] Build a new helm package with `./hack/package-helm-charts.sh`
- [ ] Switch to the `gh-pages` branch and move the newly generated package to the `stable` helm repo
- [ ] Run the `./build-index.sh` script to rebuild the indices for each repo
- [ ] Commit and push the `gh-pages` branch to GitHub
- [ ] Wait for the [CI job associated with your tag] (https://gitlab.com/nvidia/kubernetes/device-plugin/-/pipelines) to complete
- [ ] Create a [new release](https://github.com/NVIDIA/k8s-device-plugin/releases) on Github with the changelog
