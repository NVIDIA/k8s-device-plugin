# Release Process

The device plugin consists in two artifacts:
- The Device Plugin container
- The Device Plugin Daemonset Manifest

Publishing the container is automated through gitlab-ci and only requires on to tag the commit and push it to gitlab.

# Release Process Checklist
- [ ] Update the README to change occurances of the old version (e.g: 1.0.0-beta) with the new version
- [ ] Update the README changelog

- [ ] Update the device plugin (1.16+) to use the new container version (nvidia-device-plugin.yml)
- [ ] Update the legacy device plugin (pre 1.16) to use the new container version (extensions-v1beta1-nvidia-device-plugin.yml)
- [ ] Commit, Tag and Push to Gitlab
- [ ] Trigger the [multi arch manifest CI](https://gitlab.com/nvidia/container-images/dockerhub-manifests)

- [ ] Create a new release on Github
