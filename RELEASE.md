# Release Process

The device plugin consists in two artifacts:
- The Device Plugin container
- The Device Plugin helm chart

Publishing the container is automated through gitlab-ci and only requires one to tag the commit and push it to gitlab.

# Release Process Checklist
- [ ] Run the `./hack/prepare-release.sh` script to update the version in all the needed files
- [ ] Run the `./hack/package-helm-charts.sh` script to generate the helm charts
- [ ] Run the `./hack/generate-changelog.sh` script to generate the a draft changelog
- [ ] Update the CHANGELOG.md file with the generated changelog
- [ ] Commit, Tag and Push to Gitlab
- [ ] Wait for the `Release` GitHub Action to complete
- [ ] Publish the [draft release](https://github.com/NVIDIA/k8s-device-plugin/releases) created by the GitHub Action
- [ ] Wait for the `Publish Helm Chart` GitHub Action to complete

## Troubleshooting
- If the `Release` GitHub Action fails:
    - check the logs for the error first.
    - Manually run the `Release` GitHub Action locally with the same inputs.
    - If the action fails, manually run `./hack/package-helm-charts.sh` and upload the generated charts to the release assets.
- If the `Publish Helm Chart` GitHub Action fails:
    - Check the logs for the error.
    - Manually run `./hack/update-helm-index.sh` with the generated charts from the `Release` GitHub Action or the `./hack/package-helm-charts.sh` script.
