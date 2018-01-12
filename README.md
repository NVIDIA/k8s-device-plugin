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
* Kubernetes version = 1.9
* The `DevicePlugins` feature gate enabled

## Quick Start

### Preparing your GPU Nodes

The following steps need to be executed on all your GPU nodes.
Additionally, this README assumes that the NVIDIA drivers and nvidia-docker has been installed.

First you will need to check and/or enable the nvidia runtime as your default runtime on your node.
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

The second step is to enable the `DevicePlugins` feature gate on all your GPU nodes.

If your Kubernetes cluster is deployed using kubeadm and your nodes are running systemd you will have to open the kubeadm
systemd unit file at `/etc/systemd/system/kubelet.service.d/10-kubeadm.conf` and add the following environment argument:
```
Environment="KUBELET_EXTRA_ARGS=--feature-gates=DevicePlugins=true"
```

> *If you spot the Accelerators feature gate you should remove it as it might interfere with the DevicePlugins feature gate*

Reload and restart the kubelet to pick up the config change:
```shell
$ sudo systemctl daemon-reload
$ sudo systemctl restart kubelet
```

> In this guide we used kubeadm and kubectl as the method for setting up and administering the Kubernetes cluster,
> but there are many ways to deploy a Kubernetes cluster.
> To enable the `DevicePlugins` feature gate if you are not using the kubeadm + systemd configuration, you will need
> to make sure that the arguments that are passed to Kubelet include the following `--feature-gates=DevicePlugins=true`.

### Enabling GPU Support in Kubernetes

Once you have enabled this option on *all* the GPU nodes you wish to use,
you can then enable GPU support in your cluster by deploying the following Daemonset:

```shell
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v1.9/nvidia-device-plugin.yml
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
- the device plugin feature is still alpha which is why it requires the feature gate to be enabled.
- the NVIDIA device plugin is still considered alpha and is missing
    - Security features
    - More comprehensive GPU health checking features
    - GPU cleanup features
    - ...
- support will only be provided for the official NVIDIA device plugin.

The next sections are focused on building the device plugin and running it.

### With Docker

#### Build
Option 1, pull the prebuilt image from [Docker Hub](https://hub.docker.com/r/nvidia/k8s-device-plugin):
```shell
$ docker pull nvidia/k8s-device-plugin:1.9
```

Option 2, build without cloning the repository:
```shell
$ docker build -t nvidia/k8s-device-plugin:1.9 https://github.com/NVIDIA/k8s-device-plugin.git#v1.9
```

Option 3, if you want to modify the code:
```shell
$ git clone https://github.com/NVIDIA/k8s-device-plugin.git && cd k8s-device-plugin
$ docker build -t nvidia/k8s-device-plugin:1.9 .
```

#### Run locally
```shell
$ docker run --security-opt=no-new-privileges --cap-drop=ALL --network=none -it -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins nvidia/k8s-device-plugin:1.9
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

### Version 1.9

- The device Plugin API changed and is no longer compatible with 1.8
- Error messages were added

# Issues and Contributing

* You can report a bug by [filing a new issue](https://github.com/NVIDIA/k8s-device-plugin/issues/new)
* You can contribute by opening a [pull request](https://help.github.com/articles/using-pull-requests/)
