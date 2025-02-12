# NVIDIA GPU feature discovery

> Migrated from https://gitlab.com/nvidia/kubernetes/gpu-feature-discovery

## Table of Contents

- [Overview](#overview)
- [Beta Version](#beta-version)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
  * [Node Feature Discovery (NFD)](#node-feature-discovery-nfd)
  * [Preparing your GPU Nodes](#preparing-your-gpu-nodes)
  * [Deploy NVIDIA GPU Feature Discovery (GFD)](#deploy-nvidia-gpu-feature-discovery-gfd)
    + [Daemonset](#daemonset)
    + [Job](#job)
  * [Verifying Everything Works](#verifying-everything-works)
- [The GFD Command line interface](#the-gfd-command-line-interface)
- [Generated Labels](#generated-labels)
  * [MIG 'single' strategy](#mig-single-strategy)
  * [MIG 'mixed' strategy](#mig-mixed-strategy)
- [Deployment via `helm`](#deployment-via-helm)
    + [Deploying via `helm install` with a direct URL to the `helm` package](#deploying-via-helm-install-with-a-direct-url-to-the-helm-package)
- [Building and running locally on your native machine](#building-and-running-locally-on-your-native-machine)

## Overview

NVIDIA GPU Feature Discovery for Kubernetes is a software component that allows
you to automatically generate labels for the set of GPUs available on a node.
It leverages the [Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)
to perform this labeling.

## Beta Version

This tool should be considered beta until it reaches `v1.0.0`. As such, we may
break the API before reaching `v1.0.0`, but we will setup a deprecation policy
to ease the transition.

## Prerequisites

The list of prerequisites for running the NVIDIA GPU Feature Discovery is
described below:

- nvidia-docker version > 2.0 (see how to [install](https://github.com/NVIDIA/nvidia-docker)
and its [prerequisites](https://github.com/nvidia/nvidia-docker/wiki/Installation-\(version-2.0\)#prerequisites))
- docker configured with nvidia as the [default runtime](https://github.com/NVIDIA/nvidia-docker/wiki/Advanced-topics#default-runtime).
- Kubernetes version >= 1.10
- NVIDIA device plugin for Kubernetes (see how to [setup](https://github.com/NVIDIA/k8s-device-plugin))
- NFD deployed on each node you want to label with the local source configured
  - When deploying GPU feature discovery with helm (as described below) we provide a way to automatically deploy NFD for you
  - To deploy NFD yourself, please see https://github.com/kubernetes-sigs/node-feature-discovery

## Quick Start

The following assumes you have at least one node in your cluster with GPUs and
the standard NVIDIA [drivers](https://www.nvidia.com/Download/index.aspx) have
already been installed on it.

### Node Feature Discovery (NFD)

The first step is to make sure that [Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)
is running on every node you want to label. NVIDIA GPU Feature Discovery use
the `local` source so be sure to mount volumes. See
https://github.com/kubernetes-sigs/node-feature-discovery for more details.

You also need to configure the Node Feature Discovery to only expose vendor
IDs in the PCI source. To do so, please refer to the Node Feature Discovery
documentation.

The following command will deploy NFD with the minimum required set of
parameters to run `gpu-feature-discovery`.

```shell
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.16.2/deployments/static/nfd.yaml
```

**Note:** This is a simple static daemonset meant to demonstrate the basic
features required of `node-feature-discovery` in order to successfully run
`gpu-feature-discovery`. Please see the instructions below for [Deployment via
`helm`](#deployment-via-helm) when deploying in a production setting.

### Preparing your GPU Nodes

The following steps need to be executed on all your GPU nodes.
This README assumes that the NVIDIA drivers and the `nvidia-container-toolkit` have been pre-installed.
It also assumes that you have configured the `nvidia-container-runtime` as the default low-level runtime to use.

Please see: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html

### Deploy NVIDIA GPU Feature Discovery (GFD)

The next step is to run NVIDIA GPU Feature Discovery on each node as a Daemonset
or as a Job.

#### Daemonset

```shell
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.16.2/deployments/static/gpu-feature-discovery-daemonset.yaml
```

**Note:** This is a simple static daemonset meant to demonstrate the basic
features required of `gpu-feature-discovery`. Please see the instructions below
for [Deployment via `helm`](#deployment-via-helm) when deploying in a
production setting.

#### Job

You must change the `NODE_NAME` value in the template to match the name of the
node you want to label:

```shell
export NODE_NAME=<your-node-name>
curl https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.16.2/deployments/static/gpu-feature-discovery-job.yaml.template \
    | sed "s/NODE_NAME/${NODE_NAME}/" > gpu-feature-discovery-job.yaml
kubectl apply -f gpu-feature-discovery-job.yaml
```

**Note:** This method should only be used for testing and not deployed in a
production setting.

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

## The GFD Command line interface

Available options:

```shell
gpu-feature-discovery:
Usage:
  gpu-feature-discovery [--fail-on-init-error=<bool>] [--mig-strategy=<strategy>] [--oneshot | --sleep-interval=<seconds>] [--no-timestamp] [--output-file=<file> | -o <file>]
  gpu-feature-discovery -h | --help
  gpu-feature-discovery --version

Options:
  -h --help                       Show this help message and exit
  --version                       Display version and exit
  --oneshot                       Label once and exit
  --no-timestamp                  Do not add timestamp to the labels
  --fail-on-init-error=<bool>     Fail if there is an error during initialization of any label sources [Default: true]
  --sleep-interval=<seconds>      Time to sleep between labeling [Default: 60s]
  --mig-strategy=<strategy>       Strategy to use for MIG-related labels [Default: none]
  -o <file> --output-file=<file>  Path to output file
                                  [Default: /etc/kubernetes/node-feature-discovery/features.d/gfd]

Arguments:
  <strategy>: none | single | mixed
```

You can also use environment variables:

| Env Variable           | Option               | Example |
| ---------------------- | -------------------- | ------- |
| GFD_FAIL_ON_INIT_ERROR | --fail-on-init-error | true    |
| GFD_MIG_STRATEGY       | --mig-strategy       | none    |
| GFD_ONESHOT            | --oneshot            | TRUE    |
| GFD_NO_TIMESTAMP       | --no-timestamp       | TRUE    |
| GFD_OUTPUT_FILE        | --output-file        | output  |
| GFD_SLEEP_INTERVAL     | --sleep-interval     | 10s     |

Environment variables override the command line options if they conflict.

## Generated Labels

Below is the list of the labels generated by NVIDIA GPU Feature Discovery and their meaning.
For a similar list of labels generated or used by the device plugin, see [here](/README.md#catalog-of-labels).

> [!NOTE]
> Label values in Kubernetes are always of type string. The table's value type describes the type within string formatting.

| Label Name                     | Value Type | Meaning                                                                                                                                                                                | Example        |
| -------------------------------| ---------- |----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------| -------------- |
| nvidia.com/cuda.driver.major   | Integer    | (Deprecated) Major of the version of NVIDIA driver                                                                                                                                     | 550            |
| nvidia.com/cuda.driver.minor   | Integer    | (Deprecated)  Minor of the version of NVIDIA driver                                                                                                                                    | 107            |
| nvidia.com/cuda.driver.rev     | Integer    | (Deprecated) Revision of the version of NVIDIA driver                                                                                                                                  | 02             |
| nvidia.com/cuda.driver-version.major   | Integer    | Major of the version of NVIDIA driver                                                                                                                                          | 550            |
| nvidia.com/cuda.driver-version.minor   | Integer    | Minor of the version of NVIDIA driver                                                                                                                                          | 107            |
| nvidia.com/cuda.driver-version.revision   | Integer    | Revision of the version of NVIDIA driver                                                                                                                                    | 02             |
| nvidia.com/cuda.driver-version.full   | Integer    | Full version number of NVIDIA driver                                                                                                                                            | 550.107.02     |
| nvidia.com/cuda.runtime.major  | Integer    | (Deprecated) Major of the version of CUDA                                                                                                                                              | 12             |
| nvidia.com/cuda.runtime.minor  | Integer    | (Deprecated) Minor of the version of CUDA                                                                                                                                              | 5              |
| nvidia.com/cuda.runtime-version.major   | Integer    | Major of the version of CUDA                                                                                                                                                  | 12             |
| nvidia.com/cuda.runtime-version.minor   | Integer    | Minor of the version of CUDA                                                                                                                                                  | 5              |
| nvidia.com/cuda.runtime-version.full   | Integer    | Full version number of CUDA                                                                                                                                                    | 12.5           |
| nvidia.com/gfd.timestamp       | Integer    | Timestamp of the generated labels (optional)                                                                                                                                           | 1724632719     |
| nvidia.com/gpu.compute.major   | Integer    | Major of the compute capabilities                                                                                                                                                      | 7              |
| nvidia.com/gpu.compute.minor   | Integer    | Minor of the compute capabilities                                                                                                                                                      | 5              |
| nvidia.com/gpu.count           | Integer    | Number of GPUs                                                                                                                                                                         | 2              |
| nvidia.com/gpu.family          | String     | Architecture family of the GPU                                                                                                                                                         | turing         |
| nvidia.com/gpu.machine         | String     | Machine type. If in a public cloud provider, value may be set to the instance type.                                                                                                    | DGX-1          |
| nvidia.com/gpu.memory          | Integer    | Memory of the GPU in megabytes (MB)                                                                                                                                                    | 15360          |
| nvidia.com/gpu.product         | String     | Model of the GPU. May be modified by the device plugin if a sharing strategy is employed depending on the config.                                                                      | Tesla-T4       |
| nvidia.com/gpu.replicas        | String     | Number of GPU replicas available. Will be equal to the number of physical GPUs unless some sharing strategy is employed in which case the GPU count will be multiplied by replicas.    | 4              |
| nvidia.com/gpu.mode            | String     | Mode of the GPU. Can be either "compute" or "display". Details of the GPU modes can be found [here](https://docs.nvidia.com/grid/13.0/grid-gpumodeswitch-user-guide/index.html#compute-and-graphics-mode) | compute        |
| nvidia.com/gpu.clique          | String     | GPUFabric ClusterUUID + CliqueID                                                                                                                               | 7b968a6d-c8aa-45e1-9e07-e1e51be99c31.1 |

Depending on the MIG strategy used, the following set of labels may also be
available (or override the default values for some of the labels listed above):

### MIG 'single' strategy

With this strategy, the single `nvidia.com/gpu` label is overloaded to provide
information about MIG devices on the node, rather than full GPUs. This assumes
all GPUs on the node have been divided into identical partitions of the same
size. The example below shows info for a system with 8 full GPUs, each of which
is partitioned into 7 equal sized MIG devices (56 total).

| Label Name                          | Value Type | Meaning                                  | Example                   |
| ----------------------------------- | ---------- | ---------------------------------------- | ------------------------- |
| nvidia.com/mig.strategy             | String     | MIG strategy in use                      | single                    |
| nvidia.com/gpu.product (overridden) | String     | Model of the GPU (with MIG info added)   | A100-SXM4-40GB-MIG-1g.5gb |
| nvidia.com/gpu.count   (overridden) | Integer    | Number of MIG devices                    | 56                        |
| nvidia.com/gpu.memory  (overridden) | Integer    | Memory of each MIG device in megabytes (MB) | 5120                      |
| nvidia.com/gpu.multiprocessors      | Integer    | Number of Multiprocessors for MIG device | 14                        |
| nvidia.com/gpu.slices.gi            | Integer    | Number of GPU Instance slices            | 1                         |
| nvidia.com/gpu.slices.ci            | Integer    | Number of Compute Instance slices        | 1                         |
| nvidia.com/gpu.engines.copy         | Integer    | Number of DMA engines for MIG device     | 1                         |
| nvidia.com/gpu.engines.decoder      | Integer    | Number of decoders for MIG device        | 1                         |
| nvidia.com/gpu.engines.encoder      | Integer    | Number of encoders for MIG device        | 1                         |
| nvidia.com/gpu.engines.jpeg         | Integer    | Number of JPEG engines for MIG device    | 0                         |
| nvidia.com/gpu.engines.ofa          | Integer    | Number of OfA engines for MIG device     | 0                         |

### MIG 'mixed' strategy

With this strategy, a separate set of labels for each MIG device type is
generated. The name of each MIG device type is defines as follows:

```
MIG_TYPE=mig-<slice_count>g.<memory_size>.gb
e.g.  MIG_TYPE=mig-3g.20gb
```

| Label Name                           | Value Type | Meaning                                  | Example        |
| ------------------------------------ | ---------- | ---------------------------------------- | -------------- |
| nvidia.com/mig.strategy              | String     | MIG strategy in use                      | mixed          |
| nvidia.com/MIG\_TYPE.count           | Integer    | Number of MIG devices of this type       | 2              |
| nvidia.com/MIG\_TYPE.memory          | Integer    | Memory of MIG device type in megabytes (MB) | 10240          |
| nvidia.com/MIG\_TYPE.multiprocessors | Integer    | Number of Multiprocessors for MIG device | 14             |
| nvidia.com/MIG\_TYPE.slices.ci       | Integer    | Number of GPU Instance slices            | 1              |
| nvidia.com/MIG\_TYPE.slices.gi       | Integer    | Number of Compute Instance slices        | 1              |
| nvidia.com/MIG\_TYPE.engines.copy    | Integer    | Number of DMA engines for MIG device     | 1              |
| nvidia.com/MIG\_TYPE.engines.decoder | Integer    | Number of decoders for MIG device        | 1              |
| nvidia.com/MIG\_TYPE.engines.encoder | Integer    | Number of encoders for MIG device        | 1              |
| nvidia.com/MIG\_TYPE.engines.jpeg    | Integer    | Number of JPEG engines for MIG device    | 0              |
| nvidia.com/MIG\_TYPE.engines.ofa     | Integer    | Number of OfA engines for MIG device     | 0              |

## Deployment via `helm`

The preferred method to deploy GFD is as a daemonset using `helm`.
Instructions for installing `helm` can be found
[here](https://helm.sh/docs/intro/install/).

As of `v0.15.0`, the device plugin's helm chart has integrated support to deploy GFD.

To deploy GFD standalone, begin by setting up the plugin's `helm` repository and updating it as follows:

```shell
helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
helm repo update
```

Then verify that the latest release of the plugin is available
(Note that this includes GFD ):

```shell
$ helm search repo nvdp --devel
NAME                     	  CHART VERSION  APP VERSION	DESCRIPTION
nvdp/nvidia-device-plugin	  0.16.2	 0.16.2		A Helm chart for ...
```

Once this repo is updated, you can begin installing packages from it to deploy GFD in standalone mode.

The most basic installation command without any options is then:

```shell
helm upgrade -i nvdp nvdp/nvidia-device-plugin \
  --version 0.16.2 \
  --namespace gpu-feature-discovery \
  --create-namespace \
  --set devicePlugin.enabled=false
```

Disabling auto-deployment of NFD and running with a MIG strategy of 'mixed' in
the default namespace:

```shell
helm upgrade -i nvdp nvdp/nvidia-device-plugin \
  --version=0.16.2 \
  --set allowDefaultNamespace=true \
  --set nfd.enabled=false \
  --set migStrategy=mixed \
  --set devicePlugin.enabled=false
```

**Note:** You only need the to pass the `--devel` flag to `helm search repo`
and the `--version` flag to `helm upgrade -i` if this is a pre-release
version (e.g. `<version>-rc.1`). Full releases will be listed without this.

### Deploying via `helm install` with a direct URL to the `helm` package

If you prefer not to install from the `nvidia-device-plugin` helm repo, you can
run `helm install` directly against the tarball of the plugin's helm package.
The example below installs the same chart as the method above, except that
it uses a direct URL to the helm chart instead of via the helm repo.

Using the default values for the flags:

```shell
helm upgrade -i nvdp \
  --namespace gpu-feature-discovery \
  --set devicePlugin.enabled=false \
  --create-namespace \
  https://nvidia.github.io/k8s-device-plugin/stable/nvidia-device-plugin-0.16.2.tgz
```

## Building and running locally on your native machine

Download the source code:

```shell
git clone https://github.com/NVIDIA/k8s-device-plugin
```

Get dependencies:

```shell
make vendor
```

Build it:

```shell
make build
```

Run it:

```shell
./gpu-feature-discovery --output=$(pwd)/gfd
```
