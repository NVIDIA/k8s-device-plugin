# NVIDIA device plugin for Kubernetes

## Table of Contents

- [About](#about)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
  - [Preparing your GPU Nodes](#preparing-your-gpu-nodes)
  - [Enabling GPU Support in Kubernetes](#enabling-gpu-support-in-kubernetes)
  - [Running GPU Jobs](#running-gpu-jobs)
- [Deployment via `helm`](#deployment-via-helm)
- [Building and Running Locally](#building-and-running-locally)
- [Changelog](#changelog)
- [Issues and Contributing](#issues-and-contributing)
- [Versioning](#versioning)
- [Upgrading Kubernetes with the Device Plugin](#upgrading-kubernetes-with-the-device-plugin)


## About

The NVIDIA device plugin for Kubernetes is a Daemonset that allows you to automatically:
- Expose the number of GPUs on each nodes of your cluster
- Keep track of the health of your GPUs
- Run GPU enabled containers in your Kubernetes cluster.

This repository contains NVIDIA's official implementation of the [Kubernetes device plugin](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/resource-management/device-plugin.md).

Please note that:
- The NVIDIA device plugin API is beta as of Kubernetes v1.10.
- The NVIDIA device plugin is still considered beta and is missing
    - More comprehensive GPU health checking features
    - GPU cleanup features
    - ...
- Support will only be provided for the official NVIDIA device plugin (and not
  for forks or other variants of this plugin).

## Prerequisites

The list of prerequisites for running the NVIDIA device plugin is described below:
* NVIDIA drivers ~= 384.81
* nvidia-docker version > 2.0 (see how to [install](https://github.com/NVIDIA/nvidia-docker) and it's [prerequisites](https://github.com/nvidia/nvidia-docker/wiki/Installation-\(version-2.0\)#prerequisites))
* docker configured with nvidia as the [default runtime](https://github.com/NVIDIA/nvidia-docker/wiki/Advanced-topics#default-runtime).
* Kubernetes version >= 1.10

## Quick Start

### Preparing your GPU Nodes

The following steps need to be executed on all your GPU nodes.
This README assumes that the NVIDIA drivers and `nvidia-docker` have been installed.

Note that you need to install the `nvidia-docker2` package and not the `nvidia-container-toolkit`.
This is because the new `--gpus` options hasn't reached kubernetes yet. Example:
```bash
# Add the package repositories
$ distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
$ curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
$ curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list

$ sudo apt-get update && sudo apt-get install -y nvidia-docker2
$ sudo systemctl restart docker
```

You will need to enable the nvidia runtime as your default runtime on your node.
We will be editing the docker daemon config file which is usually present at `/etc/docker/daemon.json`:
```json
{
    "default-runtime": "nvidia",
    "runtimes": {
        "nvidia": {
            "path": "/usr/bin/nvidia-container-runtime",
            "runtimeArgs": []
        }
    }
}
```
> *if `runtimes` is not already present, head to the install page of [nvidia-docker](https://github.com/NVIDIA/nvidia-docker)*

### Enabling GPU Support in Kubernetes

Once you have configured the options above on all the GPU nodes in your
cluster, you can enable GPU support by deploying the following Daemonset:

```shell
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.7.0/nvidia-device-plugin.yml
```

**Note:** This is a simple static daemonset meant to demonstrate the basic
features of the `nvidia-device-plugin`. Please see the instructions below for
[Deployment via `helm`](#deployment-via-helm) when deploying the plugin in a
production setting.

### Running GPU Jobs

With the daemonset deployed, NVIDIA GPUs can now be requested by a container
using the `nvidia.com/gpu` resource type:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  containers:
    - name: cuda-container
      image: nvidia/cuda:9.0-devel
      resources:
        limits:
          nvidia.com/gpu: 2 # requesting 2 GPUs
    - name: digits-container
      image: nvidia/digits:6.0
      resources:
        limits:
          nvidia.com/gpu: 2 # requesting 2 GPUs
```

> **WARNING:** *if you don't request GPUs when using the device plugin with NVIDIA images all
> the GPUs on the machine will be exposed inside your container.*

## Deployment via `helm`

The preferred method to deploy the device plugin is as a daemonset using `helm`.
Instructions for installing `helm` can be found
[here](https://helm.sh/docs/intro/install/).

The `helm` chart for the latest release of the plugin (`v0.7.0`) includes
a number of customizable values. The most commonly overridden ones are:

```
  failOnInitError:
      fail the plugin if an error is encountered during initialization, otherwise block indefinitely
      (default 'true')
  compatWithCPUManager:
      run with escalated privileges to be compatible with the static CPUManager policy
      (default 'false')
  legacyDaemonsetAPI:
      use the legacy daemonset API version 'extensions/v1beta1'
      (default 'false')
  migStrategy:
      the desired strategy for exposing MIG devices on GPUs that support it
      [none | single | mixed] (default "none")
  deviceListStrategy:
      the desired strategy for passing the device list to the underlying runtime
      [envvar | volume-mounts] (default "envvar")
```

When set to true, the `failOnInitError` flag fails the plugin if an error is
encountered during initialization. When set to false, it prints an error
message and blocks the plugin indefinitely instead of failing. Blocking
indefinitely follows legacy semantics that allow the plugin to deploy
successfully on nodes that don't have GPUs on them (and aren't supposed to have
GPUs on them) without throwing an error. In this way, you can blindly deploy a
daemonset with the plugin on all nodes in your cluster, whether they have GPUs
on them or not, without encountering an error.  However, doing so means that
there is no way to detect an actual error on nodes that are supposed to have
GPUs on them. Failing if an initilization error is encountered is now the
default and should be adopted by all new deployments.

The `compatWithCPUManager` flag configures the daemonset to be able to
interoperate with the static `CPUManager` of the `kubelet`.  Setting this flag
requires one to deploy the daemonset with elevated privileges, so only do so if
you know you need to interoperate with the `CPUManager`.

The `legacyDaemonsetAPI` flag configures the daemonset to use version
`extensions/v1beta1` of the DaemonSet API. This API version was removed in
Kubernetes `v1.16`, so is only intended to allow newer plugins to run on older
versions of Kubernetes.

The `migStrategy` flag configures the daemonset to be able to expose
Multi-Instance GPUs (MIG) on GPUs that support them. More information on what
these strategies are and how they should be used can be found in [Supporting
Multi-Instance GPUs (MIG) in
Kubernetes](https://docs.google.com/document/d/1mdgMQ8g7WmaI_XVVRrCvHPFPOMCm5LQD5JefgAh6N8g).

**Note:** With a `migStrategy` of mixed, you will have additional resources
available to you of the form `nvidia.com/mig-<slice_count>g.<memory_size>gb`
that you can set in your pod spec to get access to a specific MIG device.

The `deviceListStrategy` flag allows one to choose which strategy the plugin
will use to advertise the list of GPUs allocated to a container. This is
traditionally done by setting the `NVIDIA_VISIBLE_DEVICES` environment variable
as described
[here](https://github.com/NVIDIA/nvidia-container-runtime#nvidia_visible_devices).
This strategy can be selected via the (default) `envvar` option. Support was
recently added to the `nvidia-container-toolkit` to also allow passing the list
of devices as a set of volume mounts instead of as an environment variable.
This strategy can be selected via the `volume-mounts` option. Details for the
rationale behind this strategy can be found
[here](https://docs.google.com/document/d/1uXVF-NWZQXgP1MLb87_kMkQvidpnkNWicdpO2l9g-fw/edit#heading=h.b3ti65rojfy5).

Please take a look in the following `values.yaml` file to see the full set of
overridable parameters for the device plugin.

* https://github.com/NVIDIA/k8s-device-plugin/blob/v0.7.0/deployments/helm/nvidia-device-plugin/values.yaml

#### Installing via `helm install`from the `nvidia-device-plugin` `helm` repository

The preferred method of deployment is with `helm install` via the
`nvidia-device-plugin` `helm` repository.

This repository can be installed as follows:
```shell
$ helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
$ helm repo update
```

Once this repo is updated, you can begin installing packages from it to depoloy
the `nvidia-device-plugin` daemonset. Below are some examples of deploying the
plugin with the various flags from above.

**Note:** Since this is a pre-release version, you will need to pass the
`--devel` flag to `helm search repo` in order to see this release listed.

Using the default values for the flags:
```shell
$ helm install \
    --version=0.7.0 \
    --generate-name \
    nvdp/nvidia-device-plugin
```

Enabling compatibility with the `CPUManager` and running with a request for
100ms of CPU time and a limit of 512MB of memory.
```shell
$ helm install \
    --version=0.7.0 \
    --generate-name \
    --set compatWithCPUManager=true \
    --set resources.requests.cpu=100m \
    --set resources.limits.memory=512Mi \
    nvdp/nvidia-device-plugin
```

Use the legacy Daemonset API (only available on Kubernetes < `v1.16`):
```shell
$ helm install \
    --version=0.7.0 \
    --generate-name \
    --set legacyDaemonsetAPI=true \
    nvdp/nvidia-device-plugin
```

Enabling compatibility with the `CPUManager` and the `mixed` `migStrategy`
```shell
$ helm install \
    --version=0.7.0 \
    --generate-name \
    --set compatWithCPUManager=true \
    --set migStrategy=mixed \
    nvdp/nvidia-device-plugin
```

#### Deploying via `helm install` with a direct URL to the `helm` package

If you prefer not to install from the `nvidia-device-plugin` `helm` repo, you can
run `helm install` directly against the tarball of the plugin's `helm` package.
The examples below install the same daemonsets as the method above, except that
they use direct URLs to the `helm` package instead of the `helm` repo.

Using the default values for the flags:
```shell
$ helm install \
    --generate-name \
    https://nvidia.github.com/k8s-device-plugin/stable/nvidia-device-plugin-0.7.0.tgz
```

Enabling compatibility with the `CPUManager` and running with a request for
100ms of CPU time and a limit of 512MB of memory.
```shell
$ helm install \
    --generate-name \
    --set compatWithCPUManager=true \
    --set resources.requests.cpu=100m \
    --set resources.limits.memory=512Mi \
    https://nvidia.github.com/k8s-device-plugin/stable/nvidia-device-plugin-0.7.0.tgz
```

Use the legacy Daemonset API (only available on Kubernetes < `v1.16`):
```shell
$ helm install \
    --generate-name \
    --set legacyDaemonsetAPI=true \
    https://nvidia.github.com/k8s-device-plugin/stable/nvidia-device-plugin-0.7.0.tgz
```

Enabling compatibility with the `CPUManager` and the `mixed` `migStrategy`
```shell
$ helm install \
    --generate-name \
    --set compatWithCPUManager=true \
    --set migStrategy=mixed \
    https://nvidia.github.com/k8s-device-plugin/stable/nvidia-device-plugin-0.7.0.tgz
```

## Building and Running Locally

The next sections are focused on building the device plugin locally and running it.
It is intended purely for development and testing, and not required by most users.
It assumes you are pinning to the latest release tag (i.e. `v0.7.0`), but can
easily be modified to work with any available tag or branch.

### With Docker

#### Build
Option 1, pull the prebuilt image from [Docker Hub](https://hub.docker.com/r/nvidia/k8s-device-plugin):
```shell
$ docker pull nvidia/k8s-device-plugin:v0.7.0
$ docker tag nvidia/k8s-device-plugin:v0.7.0 nvidia/k8s-device-plugin:devel
```

Option 2, build without cloning the repository:
```shell
$ docker build \
    -t nvidia/k8s-device-plugin:devel \
    -f docker/amd64/Dockerfile.ubuntu16.04 \
    https://github.com/NVIDIA/k8s-device-plugin.git#v0.7.0
```

Option 3, if you want to modify the code:
```shell
$ git clone https://github.com/NVIDIA/k8s-device-plugin.git && cd k8s-device-plugin
$ docker build \
    -t nvidia/k8s-device-plugin:devel \
    -f docker/amd64/Dockerfile.ubuntu16.04 \
    .
```

#### Run
Without compatibility for the `CPUManager` static policy:
```shell
$ docker run \
    -it \
    --security-opt=no-new-privileges \
    --cap-drop=ALL \
    --network=none \
    -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins \
    nvidia/k8s-device-plugin:devel
```

With compatibility for the `CPUManager` static policy:
```shell
$ docker run \
    -it \
    --privileged \
    --network=none \
    -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins \
    nvidia/k8s-device-plugin:devel --pass-device-specs
```

### Without Docker

#### Build
```shell
$ C_INCLUDE_PATH=/usr/local/cuda/include LIBRARY_PATH=/usr/local/cuda/lib64 go build
```

#### Run
Without compatibility for the `CPUManager` static policy:
```shell
$ ./k8s-device-plugin
```

With compatibility for the `CPUManager` static policy:
```shell
$ ./k8s-device-plugin --pass-device-specs
```

## Changelog

### Version v0.7.0

- Promote v0.7.0-rc.8 to v0.7.0

### Version v0.7.0-rc.8

- Permit configuration of alternative container registry through environment variables.
- Add an alternate set of gitlab-ci directives under .nvidia-ci.yml
- Update all k8s dependencies to v1.19.1
- Update vendoring for NVML Go bindings
- Move restart loop to force recreate of plugins on SIGHUP

### Version v0.7.0-rc.7

- Fix bug which only allowed running the plugin on machines with CUDA 10.2+ installed

### Version v0.7.0-rc.6

- Add logic to skip / error out when unsupported MIG device encountered
- Fix bug treating memory as multiple of 1000 instead of 1024
- Switch to using CUDA base images
- Add a set of standard tests to the .gitlab-ci.yml file

### Version v0.7.0-rc.5

- Add deviceListStrategyFlag to allow device list passing as volume mounts

### Version v0.7.0-rc.4

- Allow one to override selector.matchLabels in the helm chart
- Allow one to override the udateStrategy in the helm chart

### Version v0.7.0-rc.3

- Fail the plugin if NVML cannot be loaded
- Update logging to print to stderr on error
- Add best effort removal of socket file before serving
- Add logic to implement GetPreferredAllocation() call from kubelet

### Version v0.7.0-rc.2

- Add the ability to set 'resources' as part of a helm install
- Add overrides for name and fullname in helm chart
- Add ability to override image related parameters helm chart
- Add conditional support for overriding secutiryContext in helm chart

### Version v0.7.0-rc.1

- Added `migStrategy` as a parameter to select the MIG strategy to the helm chart
- Add support for MIG with different strategies {none, single, mixed}
- Update vendored NVML bindings to latest (to include MIG APIs)
- Add license in UBI image
- Update UBI image with certification requirements

### Version v0.6.0

- Update CI, build system, and vendoring mechanism
- Change versioning scheme to v0.x.x instead of v1.0.0-betax
- Introduced helm charts as a mechanism to deploy the plugin

### Version v0.5.0

- Add a new plugin.yml variant that is compatible with the CPUManager
- Change CMD in Dockerfile to ENTRYPOINT
- Add flag to optionally return list of device nodes in Allocate() call
- Refactor device plugin to eventually handle multiple resource types
- Move plugin error retry to event loop so we can exit with a signal
- Update all vendored dependencies to their latest versions
- Fix bug that was inadvertently *always* disabling health checks
- Update minimal driver version to 384.81

### Version v0.4.0

- Fixes a bug with a nil pointer dereference around `getDevices:CPUAffinity`

### Version v0.3.0

- Manifest is updated for Kubernetes 1.16+ (apps/v1)
- Adds more logging information

### Version v0.2.0

- Adds the Topology field for Kubernetes 1.16+

### Version v0.1.0

- If gRPC throws an error, the device plugin no longer ends up in a non responsive state.

### Version v0.0.0

- Reversioned to SEMVER as device plugins aren't tied to a specific version of kubernetes anymore.

### Version v1.11

- No change.

### Version v1.10

- The device Plugin API is now v1beta1

### Version v1.9

- The device Plugin API changed and is no longer compatible with 1.8
- Error messages were added

# Issues and Contributing
[Checkout the Contributing document!](CONTRIBUTING.md)

* You can report a bug by [filing a new issue](https://github.com/NVIDIA/k8s-device-plugin/issues/new)
* You can contribute by opening a [pull request](https://help.github.com/articles/using-pull-requests/)

## Versioning

Before v1.10 the versioning scheme of the device plugin had to match exactly the version of Kubernetes.
After the promotion of device plugins to beta this condition was was no longer required.
We quickly noticed that this versioning scheme was very confusing for users as they still expected to see
a version of the device plugin for each version of Kubernetes.

This versioning scheme applies to the tags `v1.8`, `v1.9`, `v1.10`, `v1.11`, `v1.12`.

We have now changed the versioning to follow [SEMVER](https://semver.org/). The
first version following this scheme has been tagged `v0.0.0`.

Going forward, the major version of the device plugin will only change
following a change in the device plugin API itself. For example, version
`v1beta1` of the device plugin API corresponds to version `v0.x.x` of the
device plugin. If a new `v2beta2` version of the device plugin API comes out,
then the device plugin will increase its major version to `1.x.x`.

As of now, the device plugin API for Kubernetes >= v1.10 is `v1beta1`.  If you
have a version of Kubernetes >= 1.10 you can deploy any device plugin version >
`v0.0.0`.

## Upgrading Kubernetes with the Device Plugin

Upgrading Kubernetes when you have a device plugin deployed doesn't require you
to do any, particular changes to your workflow.  The API is versioned and is
pretty stable (though it is not guaranteed to be non breaking). Starting with
Kubernetes version 1.10, you can use `v0.3.0` of the device plugin to perform
upgrades, and Kubernetes won't require you to deploy a different version of the
device plugin. Once a node comes back online after the upgrade, you will see
GPUs re-registering themselves automatically.

Upgrading the device plugin itself is a more complex task. It is recommended to
drain GPU tasks as we cannot guarantee that GPU tasks will survive a rolling
upgrade. However we make best efforts to preserve GPU tasks during an upgrade.
