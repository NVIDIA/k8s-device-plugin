## Quick Start

### Prerequisites

The list of prerequisites for running the NVIDIA GPU Feature Discovery and the Device Plugin is described below:
* NVIDIA drivers >= 384.81
* nvidia-docker >= 2.0 || nvidia-container-toolkit >= 1.7.0 (>= 1.11.0 to use integrated GPUs on Tegra-based systems)
* nvidia-container-runtime configured as the default low-level runtime
* Kubernetes version >= 1.10

### Preparing your GPU Nodes

The following steps need to be executed on all your GPU nodes.
This README assumes that the NVIDIA drivers and the `nvidia-container-toolkit` have been pre-installed.
It also assumes that you have configured the `nvidia-container-runtime` as the default low-level runtime to use.

Please see: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html

#### Example for debian-based systems with `docker` and `containerd`

##### Install the `nvidia-container-toolkit`

```shell
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sudo tee /etc/apt/sources.list.d/libnvidia-container.list

sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
```

##### Configure `docker`
When running `kubernetes` with `docker`, edit the config file which is usually
present at `/etc/docker/daemon.json` to set up `nvidia-container-runtime` as
the default low-level runtime:

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

And then restart `docker`:

```shell
$ sudo systemctl restart docker
```

##### Configure `containerd`
When running `kubernetes` with `containerd`, edit the config file which is
usually present at `/etc/containerd/config.toml` to set up
`nvidia-container-runtime` as the default low-level runtime:

```
version = 2
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "nvidia"

      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
          privileged_without_host_devices = false
          runtime_engine = ""
          runtime_root = ""
          runtime_type = "io.containerd.runc.v2"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
            BinaryName = "/usr/bin/nvidia-container-runtime"
```

And then restart `containerd`:

```shell
$ sudo systemctl restart containerd
```

### Node Feature Discovery (NFD)

The first step is to make sure that [Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)
is running on every node you want to label. NVIDIA GPU Feature Discovery use
the `local` source so be sure to mount volumes. See
https://github.com/kubernetes-sigs/node-feature-discovery for more details.

You also need to configure the `Node Feature Discovery` to only expose vendor
IDs in the PCI source. To do so, please refer to the Node Feature Discovery
documentation.

The following command will deploy NFD with the minimum required set of
parameters to run `gpu-feature-discovery`.

```shell
$ kubectl apply -k https://github.com/kubernetes-sigs/node-feature-discovery/deployment/overlays/default?ref=v0.13.2
```

**Note:** This is a simple static daemonset meant to demonstrate the basic
features required of `node-feature-discovery` in order to successfully run
`gpu-feature-discovery`. Please see the instructions below for [Deployment via
`helm`](#deployment-via-helm) when deploying in a production setting.


### Enabling GPU Support in Kubernetes

Once you have configured the options above on all the GPU nodes in your
cluster, you can enable GPU support by deploying the following Daemonsets:

```shell
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/deployments/static/gpu-feature-discovery-daemonset.yaml
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/deployments/static/nvidia-device-plugin.yml
```

**Note:** This is a simple static daemonset meant to demonstrate the basic
features of the `gpu-feature-discovery` and`nvidia-device-plugin`. Please see 
the instructions below for [Deployment via `helm`](deployment_via_helm.md) 
when deploying the plugin in a production setting.

### Verifying Everything Works

With both NFD and GFD deployed and running, you should now be able to see GPU
related labels appearing on any nodes that have GPUs installed on them.

```shell
$ kubectl get nodes -o yaml
apiVersion: v1
items:
- apiVersion: v1
  kind: Node
  metadata:
    ...

    labels:
      nvidia.com/cuda.driver.major: "455"
      nvidia.com/cuda.driver.minor: "06"
      nvidia.com/cuda.driver.rev: ""
      nvidia.com/cuda.runtime.major: "11"
      nvidia.com/cuda.runtime.minor: "1"
      nvidia.com/gpu.compute.major: "8"
      nvidia.com/gpu.compute.minor: "0"
      nvidia.com/gfd.timestamp: "1594644571"
      nvidia.com/gpu.count: "1"
      nvidia.com/gpu.family: ampere
      nvidia.com/gpu.machine: NVIDIA DGX-2H
      nvidia.com/gpu.memory: "39538"
      nvidia.com/gpu.product: A100-SXM4-40GB
      ...
...

```

### Running GPU Jobs

With the daemonset deployed, NVIDIA GPUs can now be requested by a container
using the `nvidia.com/gpu` resource type:

```yaml
$ cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  restartPolicy: Never
  containers:
    - name: cuda-container
      image: nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda10.2
      resources:
        limits:
          nvidia.com/gpu: 1 # requesting 1 GPU
  tolerations:
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
EOF
```

```shell
$ kubectl logs gpu-pod
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

> **WARNING:** *if you don't request GPUs when using the device plugin with NVIDIA images all
> the GPUs on the machine will be exposed inside your container.*