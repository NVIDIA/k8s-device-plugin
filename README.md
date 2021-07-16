# vGPU device plugin for Kubernetes

[![build status](https://github.com/4paradigm/k8s-device-plugin/actions/workflows/build.yml/badge.svg)](https://github.com/4paradigm/k8s-device-plugin/actions/workflows/build.yml)
[![docker pulls](https://img.shields.io/docker/pulls/4pdosc/k8s-device-plugin.svg)](https://hub.docker.com/r/4pdosc/k8s-device-plugin)
[![slack](https://img.shields.io/badge/Slack-Join%20Slack-blue)](https://join.slack.com/t/k8s-device-plugin/shared_invite/zt-oi9zkr5c-LsMzNmNs7UYg6usc0OiWKw)
[![discuss](https://img.shields.io/badge/Discuss-Ask%20Questions-blue)](https://github.com/4paradigm/k8s-device-plugin/discussions)

English version|[中文版](README_cn.md)


## Table of Contents

- [About](#about)
- [When to use](#when-to-use)
- [Benchmarks](#Benchmarks)
- [Features](#Features)
- [Experimental Features](#Experimental-Features)
- [Known Issues](#Known-Issues)
- [TODO](#TODO)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
  - [Preparing your GPU Nodes](#preparing-your-gpu-nodes)
  - [Enabling vGPU Support in Kubernetes](#enabling-vGPU-support-in-kubernetes)
  - [Running GPU Jobs](#running-gpu-jobs)
- [Tests](#Tests)
- [Issues and Contributing](#issues-and-contributing)

## About

The **vGPU device plugin** is based on NVIDIA device plugin([NVIDIA/k8s-device-plugin](https://github.com/NVIDIA/k8s-device-plugin)), and on the basis of retaining the official features, it splits the physical GPU, and limits the memory and computing unit, thereby simulating multiple small vGPU cards. In the k8s cluster, scheduling is performed based on these splited vGPUs, so that different containers can safely share the same physical GPU and improve GPU utilization. In addition, the plug-in can virtualize the device memory (the used device memory can exceed the physical device memory), run some tasks with large device memory requirements, or increase the number of shared tasks. You can refer to [the benchmarks report](#benchmarks).

## When to use

1. Low utilization of device memory and computing units, such as running 10 tf-servings on one GPU.
2. Situations that require a large number of small GPUs, such as teaching scenarios where one GPU is provided for multiple students to use, and the cloud platform provides small GPU instances.
3. In the case of insufficient physical device memory, virtual device memory can be turned on, such as training of large batches and large models.

## Benchmarks

Three instances from ai-benchmark have been used to evaluate vGPU-device-plugin performance as follows

| Test Environment | description                                              |
| ---------------- | :------------------------------------------------------: |
| Kubernetes version | v1.12.9                                                |
| Docker  version    | 18.09.1                                                |
| GPU Type           | Tesla V100                                             |
| GPU Num            | 2                                                      |

| Test instance |                         description                         |
| ------------- | :---------------------------------------------------------: |
| nvidia-device-plugin      |               k8s + nvidia k8s-device-plugin                |
| vGPU-device-plugin        | k8s + VGPU k8s-device-plugin，without virtual device memory |
| vGPU-device-plugin(virtual device memory) |  k8s + VGPU k8s-device-plugin，with virtual device memory   |

Test Cases:

| test id |     case      |   type    |         params          |
| ------- | :-----------: | :-------: | :---------------------: |
| 1.1     | Resnet-V2-50  | inference |  batch=50,size=346*346  |
| 1.2     | Resnet-V2-50  | training  |  batch=20,size=346*346  |
| 2.1     | Resnet-V2-152 | inference |  batch=10,size=256*256  |
| 2.2     | Resnet-V2-152 | training  |  batch=10,size=256*256  |
| 3.1     |    VGG-16     | inference |  batch=20,size=224*224  |
| 3.2     |    VGG-16     | training  |  batch=2,size=224*224   |
| 4.1     |    DeepLab    | inference |  batch=2,size=512*512   |
| 4.2     |    DeepLab    | training  |  batch=1,size=384*384   |
| 5.1     |     LSTM      | inference | batch=100,size=1024*300 |
| 5.2     |     LSTM      | training  | batch=10,size=1024*300  |

Test Result: ![img](./imgs/benchmark_inf.png)

![img](./imgs/benchmark_train.png)

To reproduce:

1. install vGPU-nvidia-device-plugin，and configure properly
2. run benchmark job

```
$ kubectl apply -f benchmarks/ai-benchmark/ai-benchmark.yml
```

3. View the result by using kubctl logs

```
$ kubectl logs [pod id]
```

## Features

- Specify the number of vGPUs divided by each physical GPU.
- Limit vGPU's Device Memory.
- Limit vGPU's Streaming Multiprocessor.
- Zero changes to existing programs

## Limitaions

- The VGPUs assigned to this node can't exceed the number of physical GPU card, otherwise task could fail

## Experimental Features

- Virtual Device Memory

  The device memory of the vGPU can exceed the physical device memory of the GPU. At this time, the excess part will be put in the RAM, which will have a certain impact on the performance.

## Known Issues

- When virtual device memory is turned on, if the device memory of a physical GPU is used up and there are vacant vGPUs on this GPU, the tasks assigned to these vGPUs may fail.
- Currently, only computing tasks are supported, and video codec processing is not supported.

## TODO

- Support video codec processing
- Support Multi-Instance GPUs (MIG)

## Prerequisites

The list of prerequisites for running the NVIDIA device plugin is described below:
* NVIDIA drivers ~= 384.81
* nvidia-docker version > 2.0
* Kubernetes version >= 1.10

## Quick Start

### Preparing your GPU Nodes

The following steps need to be executed on all your GPU nodes.
This README assumes that the NVIDIA drivers and `nvidia-docker` have been installed.

Note that you need to install the `nvidia-docker2` package and not the `nvidia-container-toolkit`.
This is because the new `--gpus` options hasn't reached kubernetes yet. Example:

```
# Add the package repositories
$ distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
$ curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
$ curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list

$ sudo apt-get update && sudo apt-get install -y nvidia-docker2
$ sudo systemctl restart docker
```

You will need to enable the nvidia runtime as your default runtime on your node.
We will be editing the docker daemon config file which is usually present at `/etc/docker/daemon.json`:

```
{
    "default-runtime": "nvidia",
    "runtimes": {
        "nvidia": {
            "path": "/usr/bin/nvidia-container-runtime",
            "runtimeArgs": []
        }
    }
    "default-shm-size": "2G"
}
```

> *if `runtimes` is not already present, head to the install page of [nvidia-docker](https://github.com/NVIDIA/nvidia-docker)*

### Enabling vGPU Support in Kubernetes

Once you have configured the options above on all the GPU nodes in your
cluster, remove existing NVIDIA device plugin for Kubernetes if it already exists. Then, you can download our Daemonset yaml file by following command:

```
$ wget https://raw.githubusercontent.com/4paradigm/k8s-device-plugin/master/nvidia-device-plugin.yml
```

In this Daemonset file, you can see the container `nvidia-device-plugin-ctr` takes four optional arguments to customize your vGPU support:

* `fail-on-init-error:`  
  Boolean type, by default: true. When set to true, the failOnInitError flag fails the plugin if an error is encountered during initialization. When set to false, it prints an error message and blocks the plugin indefinitely instead of failing. Blocking indefinitely follows legacy semantics that allow the plugin to deploy successfully on nodes that don't have GPUs on them (and aren't supposed to have GPUs on them) without throwing an error. In this way, you can blindly deploy a daemonset with the plugin on all nodes in your cluster, whether they have GPUs on them or not, without encountering an error. However, doing so means that there is no way to detect an actual error on nodes that are supposed to have GPUs on them. Failing if an initilization error is encountered is now the default and should be adopted by all new deployments.
* `device-split-count:` 
  Integer type, by default: 2. The number for NVIDIA device split. For a Kubernetes with *N* NVIDIA GPUs, if we set `device-split-count` argument to *K​*, this Kubernetes with our device plugin will have *K \* N* allocatable vGPU resources. Notice that we suggest not to set device-split-count argument over 5 on NVIDIA 1080 ti/NVIDIA 2080 ti, over 7 on NVIDIA  T4, and over 15 on NVIDIA A100.
* `device-memory-scaling:` 
  Float type, by default: 1. The ratio for NVIDIA device memory scaling, can be greater than 1 (enable virtual device memory, experimental feature). For NVIDIA GPU with *M* memory, if we set `device-memory-scaling` argument to *S*, vGPUs splitted by this GPU will totaly get *S \* M* memory in Kubernetes with our device plugin. The memory of each vGPU is also affected by argument `device-split-count`. For previous example, if `device-split-count` argument is set to *K*, each vGPU finally get *S \* M / K* memory.
* `device-cores-scaling:` 
  Float type, by default: 1. The ratio for NVIDIA device cores scaling, can be greater than 1. If the `device-cores-scaling` parameter is configured as *S* and the `device-split-count` parameter is configured as *K*, then the average upper limit of sm utilization within **a period of time** corresponding to each vGPU is *S / K*. The sum of the utilization rates of all vGPU sm belonging to the same physical GPU does not exceed 1.
* `enable-legacy-preferred:` Boolean type, by default: false. For kublet (<1.19) that does not support PreferredAllocation, you can set it to true. It is better to choose a preferred device. When it is turned on, this plugin needs to have read permission to pod, please refer to legacy-preferred-nvidia-device-plugin.yml . For kubelet >= 1.9, it is recommended turn off it.

After configure those optional arguments, you can enable the vGPU support by following command:

```
$ kubectl apply -f nvidia-device-plugin.yml
```

### Running GPU Jobs

NVIDIA vGPUs can now be requested by a container
using the `nvidia.com/gpu` resource type:

```
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  containers:
    - name: ubuntu-container
      image: ubuntu:18.04
      command: ["bash", "-c", "sleep 86400"]
      resources:
        limits:
          nvidia.com/gpu: 2 # requesting 2 vGPUs
```

You can now execute `nvidia-smi` command in the container and see the difference of GPU memory between vGPU and real GPU.

> **WARNING:** *if you don't request vGPUs when using the device plugin with NVIDIA images all
> the vGPUs on the machine will be exposed inside your container.*

## Tests

- TensorFlow 1.14.0-1.15.0/2.2.0-2.6.2
- torch 1.1.0-1.8.0
- mxnet 1.4.0
- mindspore 1.1.1
- xgboost 1.0-1.4
- nccl 2.4.8-2.9.9

The above frameworks have passed the test.

## Logging

Enable logging：add environment variable in pod using vGPU utility
```
LIBCUDA_LOG_LEVEL=5
```

Get vGPU log：
```
kubectl logs xxx | grep libvgpu.so
```

## Issues and Contributing

* You can report a bug, a doubt or modify by [filing a new issue](https://github.com/NVIDIA/k8s-device-plugin/issues/new)
* If you want to know more or have ideas, you can participate in [Discussions](https://github.com/4paradigm/k8s-device-plugin/discussions) and [slack](https://join.slack.com/t/k8s-device-plugin/shared_invite/zt-oi9zkr5c-LsMzNmNs7UYg6usc0OiWKw) exchanges


