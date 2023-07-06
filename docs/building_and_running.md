## Building and Running Locally

The next sections are focused on building the device plugin locally and running it.
It is intended purely for development and testing, and not required by most users.
It assumes you are pinning to the latest release tag (i.e. `v0.14.0`), but can
easily be modified to work with any available tag or branch.

### With Docker

#### Build
Option 1, pull the prebuilt image from [Docker Hub](https://hub.docker.com/r/nvidia/k8s-device-plugin):

```shell
$ docker pull nvcr.io/nvidia/k8s-device-plugin:v0.14.0
$ docker tag nvcr.io/nvidia/k8s-device-plugin:v0.14.0 nvcr.io/nvidia/k8s-device-plugin:devel
```

Option 2, build without cloning the repository:

```shell
$ docker build \
    -t nvcr.io/nvidia/k8s-device-plugin:devel \
    -f deployments/container/Dockerfile.ubuntu \
    https://github.com/NVIDIA/k8s-device-plugin.git#v0.14.0
```

Option 3, if you want to modify the code:

```shell
$ git clone https://github.com/NVIDIA/k8s-device-plugin.git && cd k8s-device-plugin
$ make -f deployments/container/Makefile build-ubuntu20.04
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
    nvcr.io/nvidia/k8s-device-plugin:devel
```

With compatibility for the `CPUManager` static policy:

```shell
$ docker run \
    -it \
    --privileged \
    --network=none \
    -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins \
    nvcr.io/nvidia/k8s-device-plugin:devel --pass-device-specs
```

### Without Docker

#### Build


```shell
$ make cmds 
```

#### Run
Without compatibility for the `CPUManager` static policy:

```shell
$ ./gpu-feature-discovery --output=$(pwd)/gfd
$ ./k8s-device-plugin
```

With compatibility for the `CPUManager` static policy:

```shell
$ ./k8s-device-plugin --pass-device-specs
```
