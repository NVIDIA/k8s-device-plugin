# Release Process

The device plugin consists in two artifacts:
- The Device Plugin container
- The Device Plugin Daemonset Manifest

Publishing the container is automated through gitlab-ci and only requires on to tag the commit and push it to gitlab.

# Release Process Checklist
- [ ] Update the README to change occurances of the old version (e.g: `v0.6.0`) with the new version
- [ ] Update the README changelog
- [ ] Commit, Tag and Push to Gitlab
- [ ] Build a new helm package with `helm package ./deployments/helm/nvidia-device-plugin`
- [ ] Switch to the `gh-pages` branch and move the newly generated package to either the `stable` or `experimental` helm repo as appropriate
- [ ] Run the `./build-index.sh` script to rebuild the indices for each repo
- [ ] Commit and push the `gh-pages` branch to GitHub
- [ ] Wait for the [CI job associated with your tag] (https://gitlab.com/nvidia/kubernetes/device-plugin/-/pipelines) to complete
- [ ] Create a new release on Github
