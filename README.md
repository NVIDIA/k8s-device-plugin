# NVIDIA device plugin for Kubernetes

This repository contains NVIDIA's implementation of the [Kubernetes device plugin](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/resource-management/device-plugin.md) alpha feature from version 1.8.  

It requires nvidia-docker 2.0 with our runtime configured as the [default runtime](https://github.com/NVIDIA/nvidia-docker/tree/2.0#default-runtime).

## Usage

#### Build
```
docker build -t nvidia-device-plugin:1.0.0 .
```

#### Run locally
```
docker run -it -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins nvidia-device-plugin:1.0.0
```

#### Deploy as Daemon Set:
```
kubectl create -f nvidia-device-plugin.yml
```
