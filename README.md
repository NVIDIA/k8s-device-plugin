# NVIDIA device plugin for Kubernetes

## Table of Contents

- [About](#about)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
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

First you will need to enable the `DevicePlugins` feature gate on GPU nodes.
If you are using kubeadm and systemd open `/etc/systemd/system/kubelet.service.d/10-kubeadm.conf` you will then need to add the following environment argument:
```
Environment="KUBELET_EXTRA_ARGS=--feature-gates=DevicePlugins=true"
```

Reload and restart the kubelet to pick up the config change:
```
sudo systemctl daemon-reload
sudo systemctl restart kubelet
```

Once you have enabled this option on *all* the GPU nodes you wish to use you can then deploy the Daemonset.

```
kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v1.9/nvidia-device-plugin.yml
```

NVIDIA GPUs can now be consumed via container level resource requirements using the resource name nvidia.com/gpu:
```
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  containers:
    - name: cuda-container
      image: nvidia/cuda:9.0
      resources:
        limits:
          nvidia.com/gpu: 2 # requesting 2 GPUs
    - name: digits-container
      image: nvidia/digits:6.0
      resources:
        limits:
          nvidia.com/gpu: 2 # requesting 2 GPUs
```

WARNING: note that if you don't request GPUs when using the device plugin with NVIDIA images all
the GPUs on the machine will be exposed inside your container.

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
```
docker pull nvidia/k8s-device-plugin:1.9
```

Option 2, build without cloning the repository:
```
docker build -t nvidia/k8s-device-plugin:1.9 https://github.com/NVIDIA/k8s-device-plugin.git
```

Option 3, if you want to modify the code:
```
git clone https://github.com/NVIDIA/k8s-device-plugin.git && cd k8s-device-plugin
docker build -t nvidia/k8s-device-plugin:1.9 .
```

#### Run locally
```
docker run --security-opt=no-new-privileges --cap-drop=ALL --network=none -it -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins nvidia/k8s-device-plugin:1.9
```

#### Deploy as Daemon Set:
```
kubectl create -f nvidia-device-plugin.yml
```

### Without Docker

#### Build
```shell
C_INCLUDE_PATH=/usr/local/cuda/include LIBRARY_PATH=/usr/local/cuda/lib64 go build
```

#### Run locally
```shell
./k8s-device-plugin
```

## Changelog

### Version 1.9

- The device Plugin API changed and is no longer compatible with 1.8
- Error messages were added

# Issues and Contributing

* You can report a bug by [filing a new issue](https://github.com/NVIDIA/k8s-device-plugin/issues/new)
* You can contribute by opening a [pull request](https://help.github.com/articles/using-pull-requests/)
