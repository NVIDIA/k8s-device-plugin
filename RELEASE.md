# Release Process

The device plugin consists in two artifacts:
- The Device Plugin container
- The Device Plugin helm chart

# Release Process Checklist:
- [ ] Create a release PR:
    - [ ] Run the `./hack/prepare-release.sh` script to update the version in all the needed files. This also creates a [release issue](https://github.com/NVIDIA/cloud-native-team/issues?q=is%3Aissue+is%3Aopen+label%3Arelease)
    - [ ] Run the `./hack/generate-changelog.sh` script to generate the draft changelog and update `CHANGELOG.md` with the changes.
    - [ ] Create a PR from the created `bump-release-{{ .VERSION }}` branch.
- [ ] Merge the release PR
- [ ] Tag the release and push the tag to the `internal` mirror:
    - [ ] Image release pipeline:
- [ ] Wait for the image release to complete.
- [ ] Push the tag to the upstream GitHub repo.
- [ ] Wait for the [`Release`](https://github.com/NVIDIA/k8s-device-plugin/actions/workflows/release.yaml) GitHub Action to complete
- [ ] Publish the [draft release](https://github.com/NVIDIA/k8s-device-plugin/releases) created by the GitHub Action
- [ ] Wait for the [`Publish Helm Chart`](https://github.com/NVIDIA/k8s-device-plugin/actions/workflows/helm.yaml) GitHub Action to complete

## Troubleshooting

*Note*: This assumes that we have the release tag checked out locally.

- If the `Release` GitHub Action fails:
    - Check the logs for the error first.
    - Create the helm packages locally by running:
      ```bash
      ./hack/package-helm-charts.sh {{ .VERSION }}
      ```
    - Create the draft release by running:
      ```bash
      ./hack/create-release.sh {{ .VERSION }}
      ```
- If the `Publish Helm Chart` GitHub Action fails:
    - Check the logs for the error.
    - Update the Helm package index on the `gh-pages` branch by running:
      ```
      ./hack/update-helm-index.sh --version {{ .VERSION }}
      ```
      (this pulls the packages from the release created in the previous step)
    - Push the change to the `gh-pages` branch:
      ```
      git -C releases/{{ .VERSION }} remote set-url origin git@github.com:NVIDIA/k8s-device-plugin.git
      git -C releases/{{ .VERSION }} push origin gh-pages
      ```
