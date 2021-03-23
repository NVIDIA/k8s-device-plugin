# VGPU device plugin for Kubernetes
[English version](README.md)

## 安装要求

* NVIDIA drivers >= 384.81
* nvidia-docker version > 2.0 
* docker已配置nvidia作为默认runtime
* Kubernetes version >= 1.10



## 快速入门
### 准备工作

在所有GPU节点上安装`nvidia-docker2`（不是`nvidia-container-toolkit`）

```
# Add the package repositories
$ distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
$ curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
$ curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list

$ sudo apt-get update && sudo apt-get install -y nvidia-docker2
$ sudo systemctl restart docker
```

配置docker的默认`runtime`为`nvidia`

```
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

*其它系统可参考 [nvidia-docker](https://github.com/NVIDIA/nvidia-docker)*

### 安装

作为k8s的Daemonset部署

```
$ kubectl create -f nvidia-device-plugin.yml
```

可以修改容器命令的参数限制VGPU的数量、显存及core。

参数说明：

* --device-split-count： 每张GPU虚拟的VGPU个数

* --device-memory-scaling：所有VGPU使用的显存总和与实际物理显存的比例，每个容器实际使用的显存上限为 `物理显存 * device-memory-scaling / device-split-count`，超出后会oom，目前仅在 compute capabilities 下有效。可以大于1（超用），超出部分会使用机器的内存，对性能有一定的影响。

* --device-cores-scaling：所有VGPU使用的core总和与实际物理core的比例，每个容器实际使用的core上限为`物理core * device-cores-scaling / device-split-count`，目前仅在 compute capabilities 下有效。 可以大于1，但实际使用总量不会大于1。

### 运行GPU任务

通过指定任务的请求资源类型 `nvidia.com/gpu` 来使用VGPU。

```
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  containers:
    - name: cuda-container
      image: nvcr.io/nvidia/cuda:9.0-devel
      resources:
        limits:
          nvidia.com/gpu: 2 # requesting 2 GPUs
    - name: digits-container
      image: nvcr.io/nvidia/digits:20.12-tensorflow-py3
      resources:
        limits:
          nvidia.com/gpu: 2 # requesting 2 GPUs
```

