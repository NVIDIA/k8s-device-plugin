# NVIDIA device plugin for Kubernetes

This repository contains NVIDIA's implementation of the [Kubernetes device plugin](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/resource-management/device-plugin.md) alpha feature from version 1.8.

It requires nvidia-docker 2.0 with our runtime configured as the [default runtime](https://github.com/NVIDIA/nvidia-docker/wiki/Advanced-topics#default-runtime).

## Usage
Please make sure that the Kubelet has been started with the `--feature-gates=DevicePlugins=true`
before running the device plugin.

### With Docker

#### Build
Option 1, pull the prebuilt image from [Docker Hub](https://hub.docker.com/r/nvidia/k8s-device-plugin):
```
docker pull nvidia/k8s-device-plugin:1.8
```

Option 2, build without cloning the repository:
```
docker build -t nvidia/k8s-device-plugin:1.8 https://github.com/NVIDIA/k8s-device-plugin.git
```

Option 3, if you want to modify the code:
```
git clone https://github.com/NVIDIA/k8s-device-plugin.git && cd k8s-device-plugin
docker build -t nvidia/k8s-device-plugin:1.8 .
```

#### Run locally
```
docker run --security-opt=no-new-privileges --cap-drop=ALL --network=none -it -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins nvidia/k8s-device-plugin:1.8
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

# Issues and Contributing

* You can report a bug by [filing a new issue](https://github.com/NVIDIA/k8s-device-plugin/issues/new)
* You can contribute by opening a [pull request](https://help.github.com/articles/using-pull-requests/)
