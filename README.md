# NVIDIA device plugin for Kubernetes

[![End-to-end Tests](https://github.com/NVIDIA/k8s-device-plugin/actions/workflows/e2e.yaml/badge.svg)](https://github.com/NVIDIA/k8s-device-plugin/actions/workflows/e2e.yaml) [![Go Report Card](https://goreportcard.com/badge/github.com/NVIDIA/k8s-device-plugin)](https://goreportcard.com/report/github.com/NVIDIA/k8s-device-plugin) [![Latest Release](https://img.shields.io/github/v/release/NVIDIA/k8s-device-plugin)](https://github.com/NVIDIA/k8s-device-plugin/releases/latest)

## Table of Contents

- [About](#about)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
  * [Preparing your GPU Nodes](#preparing-your-gpu-nodes)
  * [Enabling GPU Support in Kubernetes](#enabling-gpu-support-in-kubernetes)
  * [Running GPU Jobs](#running-gpu-jobs)
- [Configuring the NVIDIA device plugin binary](#configuring-the-nvidia-device-plugin-binary)
  * [As command line flags or envvars](#as-command-line-flags-or-envvars)
  * [As a configuration file](#as-a-configuration-file)
  * [Configuration Option Details](#configuration-option-details)
  * [Shared Access to GPUs](#shared-access-to-gpus)
    * [With CUDA Time-Slicing](#with-cuda-time-slicing)
    * [With CUDA MPS](#with-cuda-mps)
- [Deployment via `helm`](#deployment-via-helm)
  * [Configuring the device plugin's `helm` chart](#configuring-the-device-plugins-helm-chart)
    + [Passing configuration to the plugin via a `ConfigMap`.](#passing-configuration-to-the-plugin-via-a-configmap)
      - [Single Config File Example](#single-config-file-example)
      - [Multiple Config File Example](#multiple-config-file-example)
      - [Updating Per-Node Configuration With a Node Label](#updating-per-node-configuration-with-a-node-label)
    + [Setting other helm chart values](#setting-other-helm-chart-values)
    + [Deploying with gpu-feature-discovery for automatic node labels](#deploying-with-gpu-feature-discovery-for-automatic-node-labels)
    + [Deploying gpu-feature-discovery in standalone mode](#deploying-gpu-feature-discovery-in-standalone-mode)
  * [Deploying via `helm install` with a direct URL to the `helm` package](#deploying-via-helm-install-with-a-direct-url-to-the-helm-package)
- [Building and Running Locally](#building-and-running-locally)
- [Changelog](#changelog)
- [Issues and Contributing](#issues-and-contributing)

## About

The NVIDIA device plugin for Kubernetes is a Daemonset that allows you to automatically:
- Expose the number of GPUs on each nodes of your cluster
- Keep track of the health of your GPUs
- Run GPU enabled containers in your Kubernetes cluster.

This repository contains NVIDIA's official implementation of the [Kubernetes device plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/).
As of v0.16.1 this repository also holds the implementation for GPU Feature Discovery labels,
for further information on GPU Feature Discovery see [here](docs/gpu-feature-discovery/README.md).

Please note that:
- The NVIDIA device plugin API is beta as of Kubernetes v1.10.
- The NVIDIA device plugin is currently lacking
    - Comprehensive GPU health checking features
    - GPU cleanup features
- Support will only be provided for the official NVIDIA device plugin (and not
  for forks or other variants of this plugin).

## Prerequisites

The list of prerequisites for running the NVIDIA device plugin is described below:
* NVIDIA drivers ~= 384.81
* nvidia-docker >= 2.0 || nvidia-container-toolkit >= 1.7.0 (>= 1.11.0 to use integrated GPUs on Tegra-based systems)
* nvidia-container-runtime configured as the default low-level runtime
* Kubernetes version >= 1.10

## Quick Start

### Preparing your GPU Nodes

The following steps need to be executed on all your GPU nodes.
This README assumes that the NVIDIA drivers and the `nvidia-container-toolkit` have been pre-installed.
It also assumes that you have configured the `nvidia-container-runtime` as the default low-level runtime to use.

Please see: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html

#### Example for debian-based systems with `docker` and `containerd`

##### Install the NVIDIA Container Toolkit

For instructions on installing and getting started with the NVIDIA Container Toolkit, refer to the [installation guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html#installation-guide).


Also note the configuration instructions for:
* [`containerd`](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#configuring-containerd-for-kubernetes)
* [`CRI-O`](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#configuring-cri-o)
* [`docker` (Deprecated)](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#configuring-docker)

Remembering to restart each runtime after applying the configuration changes.

If the `nvidia` runtime should be set as the default runtime (required for `docker`), the `--set-as-default` argument
must also be included in the commands above. If this is not done, a RuntimeClass needs to be defined.

##### Notes on `CRI-O` configuration
When running `kubernetes` with `CRI-O`, add the config file to set the
`nvidia-container-runtime` as the default low-level OCI runtime under
`/etc/crio/crio.conf.d/99-nvidia.conf`. This will take priority over the default
`crun` config file at `/etc/crio/crio.conf.d/10-crun.conf`:
```
[crio]

  [crio.runtime]
    default_runtime = "nvidia"

    [crio.runtime.runtimes]

      [crio.runtime.runtimes.nvidia]
        runtime_path = "/usr/bin/nvidia-container-runtime"
        runtime_type = "oci"
```
As stated in the linked documentation, this file can automatically be generated with the nvidia-ctk command:
```
$ sudo nvidia-ctk runtime configure --runtime=crio --set-as-default --config=/etc/crio/crio.conf.d/99-nvidia.conf
```
`CRI-O` uses `crun` as default low-level OCI runtime so `crun` needs to be added
to the runtimes of the `nvidia-container-runtime` in the config file at `/etc/nvidia-container-runtime/config.toml`:
```
[nvidia-container-runtime]
runtimes = ["crun", "docker-runc", "runc"]
```
And then restart `CRI-O`:
```
$ sudo systemctl restart crio
```

### Enabling GPU Support in Kubernetes

Once you have configured the options above on all the GPU nodes in your
cluster, you can enable GPU support by deploying the following Daemonset:

```shell
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.16.1/deployments/static/nvidia-device-plugin.yml
```

**Note:** This is a simple static daemonset meant to demonstrate the basic
features of the `nvidia-device-plugin`. Please see the instructions below for
[Deployment via `helm`](#deployment-via-helm) when deploying the plugin in a
production setting.

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

```
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

## Configuring the NVIDIA device plugin binary

The NVIDIA device plugin has a number of options that can be configured for it.
These options can be configured as command line flags, environment variables,
or via a config file when launching the device plugin. Here we explain what
each of these options are and how to configure them directly against the plugin
binary. The following section explains how to set these configurations when
deploying the plugin via `helm`.

### As command line flags or envvars

| Flag                     | Envvar                  | Default Value   |
|--------------------------|-------------------------|-----------------|
| `--mig-strategy`         | `$MIG_STRATEGY`         | `"none"`        |
| `--fail-on-init-error`   | `$FAIL_ON_INIT_ERROR`   | `true`          |
| `--nvidia-driver-root`   | `$NVIDIA_DRIVER_ROOT`   | `"/"`           |
| `--pass-device-specs`    | `$PASS_DEVICE_SPECS`    | `false`         |
| `--device-list-strategy` | `$DEVICE_LIST_STRATEGY` | `"envvar"`      |
| `--device-id-strategy`   | `$DEVICE_ID_STRATEGY`   | `"uuid"`        |
| `--config-file`          | `$CONFIG_FILE`          | `""`            |

### As a configuration file
```
version: v1
flags:
  migStrategy: "none"
  failOnInitError: true
  nvidiaDriverRoot: "/"
  plugin:
    passDeviceSpecs: false
    deviceListStrategy: "envvar"
    deviceIDStrategy: "uuid"
```

**Note:** The configuration file has an explicit `plugin` section because it
is a shared configuration between the plugin and
[`gpu-feature-discovery`](https://github.com/NVIDIA/gpu-feature-discovery).
All options inside the `plugin` section are specific to the plugin. All
options outside of this section are shared.

### Configuration Option Details
**`MIG_STRATEGY`**:
  the desired strategy for exposing MIG devices on GPUs that support it

  `[none | single | mixed] (default 'none')`

  The `MIG_STRATEGY` option configures the daemonset to be able to expose
  Multi-Instance GPUs (MIG) on GPUs that support them. More information on what
  these strategies are and how they should be used can be found in [Supporting
  Multi-Instance GPUs (MIG) in
  Kubernetes](https://docs.google.com/document/d/1mdgMQ8g7WmaI_XVVRrCvHPFPOMCm5LQD5JefgAh6N8g).

  **Note:** With a `MIG_STRATEGY` of mixed, you will have additional resources
  available to you of the form `nvidia.com/mig-<slice_count>g.<memory_size>gb`
  that you can set in your pod spec to get access to a specific MIG device.

**`FAIL_ON_INIT_ERROR`**:
  fail the plugin if an error is encountered during initialization, otherwise block indefinitely

  `(default 'true')`

  When set to true, the `FAIL_ON_INIT_ERROR` option fails the plugin if an error is
  encountered during initialization. When set to false, it prints an error
  message and blocks the plugin indefinitely instead of failing. Blocking
  indefinitely follows legacy semantics that allow the plugin to deploy
  successfully on nodes that don't have GPUs on them (and aren't supposed to have
  GPUs on them) without throwing an error. In this way, you can blindly deploy a
  daemonset with the plugin on all nodes in your cluster, whether they have GPUs
  on them or not, without encountering an error.  However, doing so means that
  there is no way to detect an actual error on nodes that are supposed to have
  GPUs on them. Failing if an initialization error is encountered is now the
  default and should be adopted by all new deployments.

**`NVIDIA_DRIVER_ROOT`**:
  the root path for the NVIDIA driver installation

  `(default '/')`

  When the NVIDIA drivers are installed directly on the host, this should be
  set to `'/'`. When installed elsewhere (e.g. via a driver container), this
  should be set to the root filesystem where the drivers are installed (e.g.
  `'/run/nvidia/driver'`).

  **Note:** This option is only necessary when used in conjunction with the
  `$PASS_DEVICE_SPECS` option described below. It tells the plugin what prefix
  to add to any device file paths passed back as part of the device specs.

**`PASS_DEVICE_SPECS`**:
  pass the paths and desired device node permissions for any NVIDIA devices
  being allocated to the container

  `(default 'false')`

  This option exists for the sole purpose of allowing the device plugin to
  interoperate with the `CPUManager` in Kubernetes. Setting this flag also
  requires one to deploy the daemonset with elevated privileges, so only do so if
  you know you need to interoperate with the `CPUManager`.

**`DEVICE_LIST_STRATEGY`**:
  the desired strategy for passing the device list to the underlying runtime

  `[envvar | volume-mounts | cdi-annotations | cdi-cri ] (default 'envvar')`

  **Note**: Multiple device list strategies can be specified (as a comma-separated list).

  The `DEVICE_LIST_STRATEGY` flag allows one to choose which strategy the plugin
  will use to advertise the list of GPUs allocated to a container. Possible values are:

  * `envvar` (default): the `NVIDIA_VISIBLE_DEVICES` environment variable
  as described
  [here](https://github.com/NVIDIA/nvidia-container-runtime#nvidia_visible_devices)
  is used to select the devices that are to be injected by the NVIDIA Container Runtime.
  * `volume-mounts`: the list of devices is passed as a set of volume mounts instead of as an environment variable
  to instruct the NVIDIA Container Runtime to inject the devices.
  Details for the
  rationale behind this strategy can be found
  [here](https://docs.google.com/document/d/1uXVF-NWZQXgP1MLb87_kMkQvidpnkNWicdpO2l9g-fw/edit#heading=h.b3ti65rojfy5).
  * `cdi-annotations`: CDI annotations are used to select the devices that are to be injected.
  Note that this does not require the NVIDIA Container Runtime, but does required a CDI-enabled container engine.
  * `cdi-cri`: the `CDIDevices` CRI field is used to select the CDI devices that are to be injected.
  This requries support in Kubernetes to forward these requests in the CRI to a CDI-enabled container engine.

**`DEVICE_ID_STRATEGY`**:
  the desired strategy for passing device IDs to the underlying runtime

  `[uuid | index] (default 'uuid')`

  The `DEVICE_ID_STRATEGY` flag allows one to choose which strategy the plugin will
  use to pass the device ID of the GPUs allocated to a container. The device ID
  has traditionally been passed as the UUID of the GPU. This flag lets a user
  decide if they would like to use the UUID or the index of the GPU (as seen in
  the output of `nvidia-smi`) as the identifier passed to the underlying runtime.
  Passing the index may be desirable in situations where pods that have been
  allocated GPUs by the plugin get restarted with different physical GPUs
  attached to them.

**`CONFIG_FILE`**:
  point the plugin at a configuration file instead of relying on command line
  flags or environment variables

  `(default '')`

  The order of precedence for setting each option is (1) command line flag, (2)
  environment variable, (3) configuration file. In this way, one could use a
  pre-defined configuration file, but then override the values set in it at
  launch time. As described below, a `ConfigMap` can be used to point the
  plugin at a desired configuration file when deploying via `helm`.

### Shared Access to GPUs

The NVIDIA device plugin allows oversubscription of GPUs through a set of
extended options in its configuration file. There are two flavors of sharing
available: Time-Slicing and MPS.

**Note:** The use of time-slicing and MPS are mutually exclusive.

In the case of time-slicing, CUDA time-slicing is used to allow workloads sharing a GPU to
interleave with each other. However, nothing special is done to isolate workloads that are
granted replicas from the same underlying GPU, and each workload has access to
the GPU memory and runs in the same fault-domain as of all the others (meaning
if one workload crashes, they all do).

In the case of MPS, a control daemon is used to manage access to the shared GPU.
In contrast to time-slicing, MPS does space partitioning and allows memory and
compute resources to be explicitly partitioned and enforces these limits per
workload.

#### With CUDA Time-Slicing

The extended options for sharing using time-slicing can be seen below:
```
version: v1
sharing:
  timeSlicing:
    renameByDefault: <bool>
    failRequestsGreaterThanOne: <bool>
    resources:
    - name: <resource-name>
      replicas: <num-replicas>
    ...
```

That is, for each named resource under `sharing.timeSlicing.resources`, a number
of replicas can now be specified for that resource type. These replicas
represent the number of shared accesses that will be granted for a GPU
represented by that resource type.

If `renameByDefault=true`, then each resource will be advertised under the name
`<resource-name>.shared` instead of simply `<resource-name>`.

If `failRequestsGreaterThanOne=true`, then the plugin will fail to allocate any
shared resources to a container if they request more than one. The container’s
pod will fail with an `UnexpectedAdmissionError` and need to be manually deleted,
updated, and redeployed.

For example:
```
version: v1
sharing:
  timeSlicing:
    resources:
    - name: nvidia.com/gpu
      replicas: 10
```

If this configuration were applied to a node with 8 GPUs on it, the plugin
would now advertise 80 `nvidia.com/gpu` resources to Kubernetes instead of 8.

```
$ kubectl describe node
...
Capacity:
  nvidia.com/gpu: 80
...
```

Likewise, if the following configuration were applied to a node, then 80
`nvidia.com/gpu.shared` resources would be advertised to Kubernetes instead of 8
`nvidia.com/gpu` resources.

```
version: v1
sharing:
  timeSlicing:
    renameByDefault: true
    resources:
    - name: nvidia.com/gpu
      replicas: 10
    ...
```

```
$ kubectl describe node
...
Capacity:
  nvidia.com/gpu.shared: 80
...
```

In both cases, the plugin simply creates 10 references to each GPU and
indiscriminately hands them out to anyone that asks for them.

If `failRequestsGreaterThanOne=true` were set in either of these
configurations and a user requested more than one `nvidia.com/gpu` or
`nvidia.com/gpu.shared` resource in their pod spec, then the container would
fail with the resulting error:

```
$ kubectl describe pod gpu-pod
...
Events:
  Type     Reason                    Age   From               Message
  ----     ------                    ----  ----               -------
  Warning  UnexpectedAdmissionError  13s   kubelet            Allocate failed due to rpc error: code = Unknown desc = request for 'nvidia.com/gpu: 2' too large: maximum request size for shared resources is 1, which is unexpected
...
```

**Note:** Unlike with "normal" GPU requests, requesting more than one shared
GPU does not imply that you will get guaranteed access to a proportional amount
of compute power. It only implies that you will get access to a GPU that is
shared by other clients (each of which has the freedom to run as many processes
on the underlying GPU as they want). Under the hood CUDA will simply give an
equal share of time to all of the GPU processes across all of the clients. The
`failRequestsGreaterThanOne` flag is meant to help users understand this
subtlety, by treating a request of `1` as an access request rather than an
exclusive resource request. Setting `failRequestsGreaterThanOne=true` is
recommended, but it is set to `false` by default to retain backwards
compatibility.

As of now, the only supported resource available for time-slicing are
`nvidia.com/gpu` as well as any of the resource types that emerge from
configuring a node with the mixed MIG strategy.

For example, the full set of time-sliceable resources on a T4 card would be:
```
nvidia.com/gpu
```

And the full set of time-sliceable resources on an A100 40GB card would be:
```
nvidia.com/gpu
nvidia.com/mig-1g.5gb
nvidia.com/mig-2g.10gb
nvidia.com/mig-3g.20gb
nvidia.com/mig-7g.40gb
```

Likewise, on an A100 80GB card, they would be:
```
nvidia.com/gpu
nvidia.com/mig-1g.10gb
nvidia.com/mig-2g.20gb
nvidia.com/mig-3g.40gb
nvidia.com/mig-7g.80gb
```

### With CUDA MPS

**Note**: Sharing with MPS is currently not supported on devices with MIG enabled.

The extended options for sharing using MPS can be seen below:
```
version: v1
sharing:
  mps:
    renameByDefault: <bool>
    resources:
    - name: <resource-name>
      replicas: <num-replicas>
    ...
```

That is, for each named resource under `sharing.mps.resources`, a number
of replicas can be specified for that resource type. As is the case with
time-slicing, these replicas represent the number of shared accesses that will
be granted for a GPU associated with that resource type. In contrast with
time-slicing, the amount of memory allowed per client (i.e. per partition) is
managed by the MPS control daemon and limited to an equal fraction of the total
device memory. In addition to controlling the amount of memory that each client
can consume, the MPS control daemon also limits the amount of compute capacity
that can be consumed by a client.

If `renameByDefault=true`, then each resource will be advertised under the name
`<resource-name>.shared` instead of simply `<resource-name>`.

For example:
```
version: v1
sharing:
  mps:
    resources:
    - name: nvidia.com/gpu
      replicas: 10
```

If this configuration were applied to a node with 8 GPUs on it, the plugin
would now advertise 80 `nvidia.com/gpu` resources to Kubernetes instead of 8.

```
$ kubectl describe node
...
Capacity:
  nvidia.com/gpu: 80
...
```

Likewise, if the following configuration were applied to a node, then 80
`nvidia.com/gpu.shared` resources would be advertised to Kubernetes instead of 8
`nvidia.com/gpu` resources.

```
version: v1
sharing:
  mps:
    renameByDefault: true
    resources:
    - name: nvidia.com/gpu
      replicas: 10
    ...
```

```
$ kubectl describe node
...
Capacity:
  nvidia.com/gpu.shared: 80
...
```

Furthermore, each of these resources -- either `nvidia.com/gpu` or
`nvidia.com/gpu.shared` -- would have access to the same fraction (1/10) of the
total memory and compute resources of the GPU.

**Note**: As of now, the only supported resource available for MPS are `nvidia.com/gpu`
resources and only with full GPUs.

## Deployment via `helm`

The preferred method to deploy the device plugin is as a daemonset using `helm`.
Instructions for installing `helm` can be found
[here](https://helm.sh/docs/intro/install/).

Begin by setting up the plugin's `helm` repository and updating it at follows:
```shell
$ helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
$ helm repo update
```

Then verify that the latest release (`v0.16.1`) of the plugin is available:
```
$ helm search repo nvdp --devel
NAME                     	  CHART VERSION  APP VERSION	DESCRIPTION
nvdp/nvidia-device-plugin	  0.16.1	 0.16.1		A Helm chart for ...
```

Once this repo is updated, you can begin installing packages from it to deploy
the `nvidia-device-plugin` helm chart.

The most basic installation command without any options is then:
```
helm upgrade -i nvdp nvdp/nvidia-device-plugin \
  --namespace nvidia-device-plugin \
  --create-namespace \
  --version 0.16.1
```

**Note:** You only need the to pass the `--devel` flag to `helm search repo`
and the `--version` flag to `helm upgrade -i` if this is a pre-release
version (e.g. `<version>-rc.1`). Full releases will be listed without this.

### Configuring the device plugin's `helm` chart

The `helm` chart for the latest release of the plugin (`v0.16.1`) includes
a number of customizable values.

Prior to `v0.12.0` the most commonly used values were those that had direct
mappings to the command line options of the plugin binary. As of `v0.12.0`, the
preferred method to set these options is via a `ConfigMap`. The primary use
case of the original values is then to override an option from the `ConfigMap`
if desired. Both methods are discussed in more detail below.

The full set of values that can be set are found here:
[here](https://github.com/NVIDIA/k8s-device-plugin/blob/v0.16.1/deployments/helm/nvidia-device-plugin/values.yaml).

#### Passing configuration to the plugin via a `ConfigMap`.

In general, we provide a mechanism to pass _multiple_ configuration files to
to the plugin's `helm` chart, with the ability to choose which configuration
file should be applied to a node via a node label.

In this way, a single chart can be used to deploy each component, but custom
configurations can be applied to different nodes throughout the cluster.

There are two ways to provide a `ConfigMap` for use by the plugin:
  1. Via an external reference to a pre-defined `ConfigMap`
  1. As a set of named config files to build an integrated `ConfigMap` associated with the chart

These can be set via the chart values `config.name` and `config.map` respectively.
In both cases, the value `config.default` can be set to point to one of the
named configs in the `ConfigMap` and provide a default configuration for nodes
that have not been customized via a node label (more on this later).

#####  Single Config File Example
As an example, create a valid config file on your local filesystem, such as the following:
```
cat << EOF > /tmp/dp-example-config0.yaml
version: v1
flags:
  migStrategy: "none"
  failOnInitError: true
  nvidiaDriverRoot: "/"
  plugin:
    passDeviceSpecs: false
    deviceListStrategy: envvar
    deviceIDStrategy: uuid
EOF
```

And deploy the device plugin via helm (pointing it at this config file and giving it a name):
```
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.16.1 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set-file config.map.config=/tmp/dp-example-config0.yaml
```

Under the hood this will deploy a `ConfigMap` associated with the plugin and put
the contents of the `dp-example-config0.yaml` file into it, using the name
`config` as its key. It will then start the plugin such that this config gets
applied when the plugin comes online.

If you don’t want the plugin’s helm chart to create the `ConfigMap` for you, you
can also point it at a pre-created `ConfigMap` as follows:
```
$ kubectl create ns nvidia-device-plugin
```
```
$ kubectl create cm -n nvidia-device-plugin nvidia-plugin-configs \
    --from-file=config=/tmp/dp-example-config0.yaml
```
```
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.16.1 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set config.name=nvidia-plugin-configs
```

#####  Multiple Config File Example

For multiple config files, the procedure is similar.

Create a second `config` file with the following contents:
```
cat << EOF > /tmp/dp-example-config1.yaml
version: v1
flags:
  migStrategy: "mixed" # Only change from config0.yaml
  failOnInitError: true
  nvidiaDriverRoot: "/"
  plugin:
    passDeviceSpecs: false
    deviceListStrategy: envvar
    deviceIDStrategy: uuid
EOF
```

And redeploy the device plugin via helm (pointing it at both configs with a specified default).
```
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.16.1 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set config.default=config0 \
    --set-file config.map.config0=/tmp/dp-example-config0.yaml \
    --set-file config.map.config1=/tmp/dp-example-config1.yaml
```

As before, this can also be done with a pre-created `ConfigMap` if desired:
```
$ kubectl create ns nvidia-device-plugin
```
```
$ kubectl create cm -n nvidia-device-plugin nvidia-plugin-configs \
    --from-file=config0=/tmp/dp-example-config0.yaml \
    --from-file=config1=/tmp/dp-example-config1.yaml
```
```
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.16.1 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set config.default=config0 \
    --set config.name=nvidia-plugin-configs
```

**Note:** If the `config.default` flag is not explicitly set, then a default
value will be inferred from the config if one of the config names is set to
'`default`'. If neither of these are set, then the deployment will fail unless
there is only **_one_** config provided. In the case of just a single config being
provided, it will be chosen as the default because there is no other option.

##### Updating Per-Node Configuration With a Node Label

With this setup, plugins on all nodes will have `config0` configured for them
by default. However, the following label can be set to change which
configuration is applied:
```
kubectl label nodes <node-name> –-overwrite \
    nvidia.com/device-plugin.config=<config-name>
```

For example, applying a custom config for all nodes that have T4 GPUs installed
on them might be:
```
kubectl label node \
    --overwrite \
    --selector=nvidia.com/gpu.product=TESLA-T4 \
    nvidia.com/device-plugin.config=t4-config
```

**Note:** This label can be applied either _before_ or _after_ the plugin is
started to get the desired configuration applied on the node. Anytime it
changes value, the plugin will immediately be updated to start serving the
desired configuration. If it is set to an unknown value, it will skip
reconfiguration. If it is ever unset, it will fallback to the default.

#### Setting other helm chart values

As mentioned previously, the device plugin's helm chart continues to provide
direct values to set the configuration options of the plugin without using a
`ConfigMap`. These should only be used to set globally applicable options
(which should then never be embedded in the set of config files provided by the
`ConfigMap`), or used to override these options as desired.

These values are as follows:
```
  migStrategy:
      the desired strategy for exposing MIG devices on GPUs that support it
      [none | single | mixed] (default "none")
  failOnInitError:
      fail the plugin if an error is encountered during initialization, otherwise block indefinitely
      (default 'true')
  compatWithCPUManager:
      run with escalated privileges to be compatible with the static CPUManager policy
      (default 'false')
  deviceListStrategy:
      the desired strategy for passing the device list to the underlying runtime
      [envvar | volume-mounts | cdi-annotations | cdi-cri] (default "envvar")
  deviceIDStrategy:
      the desired strategy for passing device IDs to the underlying runtime
      [uuid | index] (default "uuid")
  nvidiaDriverRoot:
      the root path for the NVIDIA driver installation (typical values are '/' or '/run/nvidia/driver')
```

**Note:**  There is no value that directly maps to the `PASS_DEVICE_SPECS`
configuration option of the plugin. Instead a value called
`compatWithCPUManager` is provided which acts as a proxy for this option.
It both sets the `PASS_DEVICE_SPECS` option of the plugin to true **AND** makes
sure that the plugin is started with elevated privileges to ensure proper
compatibility with the `CPUManager`.

Besides these custom configuration options for the plugin, other standard helm
chart values that are commonly overridden are:

```
  runtimeClassName:
      the runtimeClassName to use, for use with clusters that have multiple runtimes. (typical value is 'nvidia')
```

Please take a look in the
[`values.yaml`](https://github.com/NVIDIA/k8s-device-plugin/blob/v0.16.1/deployments/helm/nvidia-device-plugin/values.yaml)
file to see the full set of overridable parameters for the device plugin.

Examples of setting these options include:

Enabling compatibility with the `CPUManager` and running with a request for
100ms of CPU time and a limit of 512MB of memory.
```shell
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.16.1 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set compatWithCPUManager=true \
    --set resources.requests.cpu=100m \
    --set resources.limits.memory=512Mi
```

Enabling compatibility with the `CPUManager` and the `mixed` `migStrategy`
```shell
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.16.1 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set compatWithCPUManager=true \
    --set migStrategy=mixed
```

#### Deploying with gpu-feature-discovery for automatic node labels

As of `v0.12.0`, the device plugin's helm chart has integrated support to
deploy
[`gpu-feature-discovery`](https://github.com/NVIDIA/gpu-feature-discovery)
(GFD) as a subchart. One can use GFD to automatically generate labels for the
set of GPUs available on a node. Under the hood, it leverages Node Feature
Discovery to perform this labeling.

To enable it, simply set `gfd.enabled=true` during helm install.
```
helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.16.1 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set gfd.enabled=true
```

Under the hood this will also deploy
[`node-feature-discovery`](https://github.com/kubernetes-sigs/node-feature-discovery)
(NFD) since it is a prerequisite of GFD. If you already have NFD deployed on
your cluster and do not wish for it to be pulled in by this installation, you
can disable it with `nfd.enabled=false`.

In addition to the standard node labels applied by GFD, the following label
will also be included when deploying the plugin with the time-slicing extensions
described [above](#shared-access-to-gpus-with-cuda-time-slicing).

```
nvidia.com/<resource-name>.replicas = <num-replicas>
```

Additionally, the `nvidia.com/<resource-name>.product` will be modified as follows if
`renameByDefault=false`.
```
nvidia.com/<resource-name>.product = <product name>-SHARED
```

Using these labels, users have a way of selecting a shared vs. non-shared GPU
in the same way they would traditionally select one GPU model over another.
That is, the `SHARED` annotation ensures that a `nodeSelector` can be used to
attract pods to nodes that have shared GPUs on them.

Since having `renameByDefault=true` already encodes the fact that the resource is
shared on the resource name , there is no need to annotate the product
name with `SHARED`. Users can already find the shared resources they need by
simply requesting it in their pod spec.

Note: When running with `renameByDefault=false` and `migStrategy=single` both
the MIG profile name and the new `SHARED` annotation will be appended to the
product name, e.g.:
```
nvidia.com/gpu.product = A100-SXM4-40GB-MIG-1g.5gb-SHARED
```

#### Deploying gpu-feature-discovery in standalone mode

As of v0.16.1, the device plugin's helm chart has integrated support to deploy
[`gpu-feature-discovery`](https://gitlab.com/nvidia/kubernetes/gpu-feature-discovery/-/tree/main)

When gpu-feature-discovery in deploying standalone, begin by setting up the
plugin's `helm` repository and updating it at follows:

```shell
$ helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
$ helm repo update
```

Then verify that the latest release (`v0.16.1`) of the plugin is available
(Note that this includes the GFD chart):

```shell
$ helm search repo nvdp --devel
NAME                     	  CHART VERSION  APP VERSION	DESCRIPTION
nvdp/nvidia-device-plugin	  0.16.1	 0.16.1		A Helm chart for ...
```

Once this repo is updated, you can begin installing packages from it to deploy
the `gpu-feature-discovery` component in standalone mode.

The most basic installation command without any options is then:

```
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
  --version 0.16.1 \
  --namespace gpu-feature-discovery \
  --create-namespace \
  --set devicePlugin.enabled=false
```

Disabling auto-deployment of NFD and running with a MIG strategy of 'mixed' in
the default namespace.

```shell
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.16.1 \
    --set allowDefaultNamespace=true \
    --set nfd.enabled=false \
    --set migStrategy=mixed \
    --set devicePlugin.enabled=false
```

**Note:** You only need the to pass the `--devel` flag to `helm search repo`
and the `--version` flag to `helm upgrade -i` if this is a pre-release
version (e.g. `<version>-rc.1`). Full releases will be listed without this.

### Deploying via `helm install` with a direct URL to the `helm` package

If you prefer not to install from the `nvidia-device-plugin` `helm` repo, you can
run `helm install` directly against the tarball of the plugin's `helm` package.
The example below installs the same chart as the method above, except that
it uses a direct URL to the `helm` chart instead of via the `helm` repo.

Using the default values for the flags:
```shell
$ helm upgrade -i nvdp \
    --namespace nvidia-device-plugin \
    --create-namespace \
    https://nvidia.github.io/k8s-device-plugin/stable/nvidia-device-plugin-0.16.1.tgz
```

## Building and Running Locally

The next sections are focused on building the device plugin locally and running it.
It is intended purely for development and testing, and not required by most users.
It assumes you are pinning to the latest release tag (i.e. `v0.16.1`), but can
easily be modified to work with any available tag or branch.

### With Docker

#### Build
Option 1, pull the prebuilt image from [Docker Hub](https://hub.docker.com/r/nvidia/k8s-device-plugin):
```shell
$ docker pull nvcr.io/nvidia/k8s-device-plugin:v0.16.1
$ docker tag nvcr.io/nvidia/k8s-device-plugin:v0.16.1 nvcr.io/nvidia/k8s-device-plugin:devel
```

Option 2, build without cloning the repository:
```shell
$ docker build \
    -t nvcr.io/nvidia/k8s-device-plugin:devel \
    -f deployments/container/Dockerfile.ubuntu \
    https://github.com/NVIDIA/k8s-device-plugin.git#v0.16.1
```

Option 3, if you want to modify the code:
```shell
$ git clone https://github.com/NVIDIA/k8s-device-plugin.git && cd k8s-device-plugin
$ docker build \
    -t nvcr.io/nvidia/k8s-device-plugin:devel \
    -f deployments/container/Dockerfile.ubuntu \
    .
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
$ C_INCLUDE_PATH=/usr/local/cuda/include LIBRARY_PATH=/usr/local/cuda/lib64 go build
```

#### Run
Without compatibility for the `CPUManager` static policy:
```shell
$ ./k8s-device-plugin
```

With compatibility for the `CPUManager` static policy:
```shell
$ ./k8s-device-plugin --pass-device-specs
```

## Changelog

See the [changelog](CHANGELOG.md)

## Issues and Contributing
[Checkout the Contributing document!](CONTRIBUTING.md)

* You can report a bug by [filing a new issue](https://github.com/NVIDIA/k8s-device-plugin/issues/new)
* You can contribute by opening a [pull request](https://help.github.com/articles/using-pull-requests/)

### Versioning

Before v1.10 the versioning scheme of the device plugin had to match exactly the version of Kubernetes.
After the promotion of device plugins to beta this condition was was no longer required.
We quickly noticed that this versioning scheme was very confusing for users as they still expected to see
a version of the device plugin for each version of Kubernetes.

This versioning scheme applies to the tags `v1.8`, `v1.9`, `v1.10`, `v1.11`, `v1.12`.

We have now changed the versioning to follow [SEMVER](https://semver.org/). The
first version following this scheme has been tagged `v0.0.0`.

Going forward, the major version of the device plugin will only change
following a change in the device plugin API itself. For example, version
`v1beta1` of the device plugin API corresponds to version `v0.x.x` of the
device plugin. If a new `v2beta2` version of the device plugin API comes out,
then the device plugin will increase its major version to `1.x.x`.

As of now, the device plugin API for Kubernetes >= v1.10 is `v1beta1`.  If you
have a version of Kubernetes >= 1.10 you can deploy any device plugin version >
`v0.0.0`.

### Upgrading Kubernetes with the Device Plugin

Upgrading Kubernetes when you have a device plugin deployed doesn't require you
to do any, particular changes to your workflow.  The API is versioned and is
pretty stable (though it is not guaranteed to be non breaking). Starting with
Kubernetes version 1.10, you can use `v0.3.0` of the device plugin to perform
upgrades, and Kubernetes won't require you to deploy a different version of the
device plugin. Once a node comes back online after the upgrade, you will see
GPUs re-registering themselves automatically.

Upgrading the device plugin itself is a more complex task. It is recommended to
drain GPU tasks as we cannot guarantee that GPU tasks will survive a rolling
upgrade. However we make best efforts to preserve GPU tasks during an upgrade.
