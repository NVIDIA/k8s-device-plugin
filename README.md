# NVIDIA device plugin for Kubernetes

## Table of Contents

- [About](#about)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
  - [Preparing your GPU Nodes](#preparing-your-gpu-nodes)
  - [Enabling GPU Support in Kubernetes](#enabling-gpu-support-in-kubernetes)
  - [Running GPU Jobs](#running-gpu-jobs)
- [Docs](#docs)
- [Changelog](#changelog)
- [Issues and Contributing](#issues-and-contributing)


## About

The NVIDIA device plugin for Kubernetes is a Daemonset that allows you to automatically:
- Expose the number of GPUs on each nodes of your cluster
- Keep track of the health of your GPUs
- Run GPU enabled containers in your Kubernetes cluster.

This repository contains NVIDIA's official implementation of the [Kubernetes device plugin](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/resource-management/device-plugin.md).

## Prerequisites

The list of prerequisites for running the NVIDIA device plugin is described below:
* NVIDIA drivers ~= 361.93
* nvidia-docker version > 2.0 (see how to [install](https://github.com/NVIDIA/nvidia-docker) and it's [prerequisites](https://github.com/nvidia/nvidia-docker/wiki/Installation-\(version-2.0\)#prerequisites))
* docker configured with nvidia as the [default runtime](https://github.com/NVIDIA/nvidia-docker/wiki/Advanced-topics#default-runtime).
* Kubernetes version >= 1.10

## Quick Start

### Preparing your GPU Nodes

The following steps need to be executed on all your GPU nodes.
This README assumes that the NVIDIA drivers and nvidia-docker have been installed.

Note that you need to install the nvidia-docker2 package and not the nvidia-container-toolkit.
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

Once you have enabled this option on *all* the GPU nodes you wish to use,
you can then enable GPU support in your cluster by deploying the following Daemonset:

```shell
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/1.0.0-beta4/nvidia-device-plugin.yml
```

### Running GPU Jobs

NVIDIA GPUs can now be consumed via container level resource requirements using the resource name nvidia.com/gpu:
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

## Docs

Please note that:
- the device plugin feature is beta as of Kubernetes v1.11.
- the NVIDIA device plugin is still considered beta and is missing
    - More comprehensive GPU health checking features
    - GPU cleanup features
    - ...
- support will only be provided for the official NVIDIA device plugin.

The next sections are focused on building the device plugin and running it.

### With Docker

#### Build
Option 1, pull the prebuilt image from [Docker Hub](https://hub.docker.com/r/nvidia/k8s-device-plugin):
```shell
$ docker pull nvidia/k8s-device-plugin:1.0.0-beta4
```

Option 2, build without cloning the repository:
```shell
$ docker build -t nvidia/k8s-device-plugin:1.0.0-beta4 https://github.com/NVIDIA/k8s-device-plugin.git#1.0.0-beta4
```

Option 3, if you want to modify the code:
```shell
$ git clone https://github.com/NVIDIA/k8s-device-plugin.git && cd k8s-device-plugin
$ git checkout 1.0.0-beta4
$ docker build -t nvidia/k8s-device-plugin:1.0.0-beta4 .
```

#### Run locally
```shell
$ docker run --security-opt=no-new-privileges --cap-drop=ALL --network=none -it -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins nvidia/k8s-device-plugin:1.0.0-beta4
```

#### Deploy as Daemon Set:
```shell
$ kubectl create -f nvidia-device-plugin.yml
```

### Without Docker

#### Build
```shell
$ C_INCLUDE_PATH=/usr/local/cuda/include LIBRARY_PATH=/usr/local/cuda/lib64 go build
```

#### Run locally
```shell
$ ./k8s-device-plugin
```

## Changelog

### Version 1.0.0-beta4

- Fixes a bug with a nil pointer dereference around `getDevices:CPUAffinity`

### Version 1.0.0-beta3

- Manifest is updated for Kubernetes 1.16+ (apps/v1)
- Adds more logging information

### Version 1.0.0-beta2

- Adds the Topology field for Kubernetes 1.16+

### Version 1.0.0-beta1

- If gRPC throws an error, the device plugin no longer ends up in a non responsive state.

### Version 1.0.0-beta

- Reversioned to SEMVER as device plugins aren't tied to a specific version of kubernetes anymore.

### Version 1.11

- No change.

### Version 1.10

- The device Plugin API is now v1beta1

### Version 1.9

- The device Plugin API changed and is no longer compatible with 1.8
- Error messages were added

# Issues and Contributing
[Checkout the Contributing document!](CONTRIBUTING.md)

* You can report a bug by [filing a new issue](https://github.com/NVIDIA/k8s-device-plugin/issues/new)
* You can contribute by opening a [pull request](https://help.github.com/articles/using-pull-requests/)

## Versioning

Before 1.10 the versioning scheme of the device plugin had to match exactly the version of Kubernetes.
After the promotion of device plugins to beta this condition was was no longer required.
We quickly noticed that this versioning scheme was very confusing for users as they still expected to see
a version of the device plugin for each version of Kubernetes.

We recently decided to reversion to follow a SEMVER scheme. This means that we are currently a
beta project (as we depend on the device plugin API which is beta).
If you have a version of Kubernetes > 1.10 you can deploy this device plugin.

## Upgrading Kubernetes with the device plugin

Upgrading Kubernetes when you have a device plugin deployed doesn't require you to do any,
particular changes to your workflow.
The API is versioned and is pretty stable (though it is not guaranteed to be non breaking),
you can therefore use the 1.0.0-beta3 version starting from kubernetes version 1.10, upgrading
kubernetes won't require you to deploy a different version of the device plugin and you will
see GPUs re-registering themselves after your node comes back online.


Upgrading the device plugin is a more complex task. It is recommended to drain GPU tasks as
we cannot guarantee that GPU tasks will survive a rolling upgrade.
However we make best efforts to preserve GPU tasks during an upgrade.
