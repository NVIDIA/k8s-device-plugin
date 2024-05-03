# Release Process

The device plugin consists in two artifacts:
- The Device Plugin container
- The Device Plugin helm chart

Publishing the container is automated through gitlab-ci and only requires one to tag the commit and push it to gitlab.
Publishing the helm chart is currently manual, and we should move to an automated process ASAP

# Release Process Checklist
- [ ] Run the `./hack/prepare-release.sh` script to update the version in all the needed files
- [ ] Run the `./hack/package-helm-charts.sh` script to generate the helm charts
- [ ] Run the `./hack/generate-changelog.sh` script to generate the a draft changelog
- [ ] Update the CHANGELOG.md file with the generated changelog
- [ ] Commit, Tag and Push to Gitlab
- [ ] Switch to the `gh-pages` branch and move the newly generated package to the `stable` helm repo
- [ ] While on the `gh-pages` branch, run the `./build-index.sh` script to rebuild the indices for each repo
- [ ] Commit and push the `gh-pages` branch to GitHub
- [ ] Wait for the [CI job associated with your tag] (https://gitlab.com/nvidia/kubernetes/device-plugin/-/pipelines) to complete
- [ ] Create a [new release](https://github.com/NVIDIA/k8s-device-plugin/releases) on Github with the changelog
