# NVIDIA device plugin for Kubernetes

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
  * [Shared Access to GPUs with CUDA Time-Slicing](#shared-access-to-gpus-with-cuda-time-slicing)
- [Deployment via `helm`](#deployment-via-helm)
  * [Configuring the device plugin's `helm` chart](#configuring-the-device-plugins-helm-chart)
    + [Passing configuration to the plugin via a `ConfigMap`.](#passing-configuration-to-the-plugin-via-a-configmap)
      - [Single Config File Example](#single-config-file-example)
      - [Multiple Config File Example](#multiple-config-file-example)
      - [Updating Per-Node Configuration With a Node Label](#updating-per-node-configuration-with-a-node-label)
    + [Setting other helm chart values](#setting-other-helm-chart-values)
    + [Deploying with gpu-feature-discovery for automatic node labels](#deploying-with-gpu-feature-discovery-for-automatic-node-labels)
  * [Deploying via `helm install` with a direct URL to the `helm` package](#deploying-via-helm-install-with-a-direct-url-to-the-helm-package)
- [Building and Running Locally](#building-and-running-locally)
- [Changelog](#changelog)
- [Issues and Contributing](#issues-and-contributing)

## About

The NVIDIA device plugin for Kubernetes is a Daemonset that allows you to automatically:
- Expose the number of GPUs on each nodes of your cluster
- Keep track of the health of your GPUs
- Run GPU enabled containers in your Kubernetes cluster.

This repository contains NVIDIA's official implementation of the [Kubernetes device plugin](https://github.com/kubernetes/enhancements/blob/master/keps/sig-node/3573-device-plugin/README.md).

Please note that:
- The NVIDIA device plugin API is beta as of Kubernetes v1.10.
- The NVIDIA device plugin is currently lacking
    - Comprehensive GPU health checking features
    - GPU cleanup features
    - ...
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

##### Install the `nvidia-container-toolkit`
```bash
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
```
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
```
$ sudo systemctl restart containerd
```

### Enabling GPU Support in Kubernetes

Once you have configured the options above on all the GPU nodes in your
cluster, you can enable GPU support by deploying the following Daemonset:

```shell
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
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

  `[envvar | volume-mounts] (default 'envvar')`

  The `DEVICE_LIST_STRATEGY` flag allows one to choose which strategy the plugin
  will use to advertise the list of GPUs allocated to a container. This is
  traditionally done by setting the `NVIDIA_VISIBLE_DEVICES` environment variable
  as described
  [here](https://github.com/NVIDIA/nvidia-container-runtime#nvidia_visible_devices).
  This strategy can be selected via the (default) `envvar` option. Support has
  been added to the `nvidia-container-toolkit` to also allow passing the list
  of devices as a set of volume mounts instead of as an environment variable.
  This strategy can be selected via the `volume-mounts` option. Details for the
  rationale behind this strategy can be found
  [here](https://docs.google.com/document/d/1uXVF-NWZQXgP1MLb87_kMkQvidpnkNWicdpO2l9g-fw/edit#heading=h.b3ti65rojfy5).

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

### Shared Access to GPUs with CUDA Time-Slicing

The NVIDIA device plugin allows oversubscription of GPUs through a set of
extended options in its configuration file. Under the hood, CUDA time-slicing
is used to allow workloads that land on oversubscribed GPUs to interleave with
one another. However, nothing special is done to isolate workloads that are
granted replicas from the same underlying GPU, and each workload has access to
the GPU memory and runs in the same fault-domain as of all the others (meaning
if one workload crashes, they all do).


These extended options can be seen below:
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

## Deployment via `helm`

The preferred method to deploy the device plugin is as a daemonset using `helm`.
Instructions for installing `helm` can be found
[here](https://helm.sh/docs/intro/install/).

Begin by setting up the plugin's `helm` repository and updating it at follows:
```shell
$ helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
$ helm repo update
```

Then verify that the latest release (`v0.14.0`) of the plugin is available:
```
$ helm search repo nvdp --devel
NAME                     	  CHART VERSION  APP VERSION	DESCRIPTION
nvdp/nvidia-device-plugin	  0.14.0	 0.14.0		A Helm chart for ...
```

Once this repo is updated, you can begin installing packages from it to deploy
the `nvidia-device-plugin` helm chart.

The most basic installation command without any options is then:
```
helm upgrade -i nvdp nvdp/nvidia-device-plugin \
  --namespace nvidia-device-plugin \
  --create-namespace \
  --version 0.14.0
```

**Note:** You only need the to pass the `--devel` flag to `helm search repo`
and the `--version` flag to `helm upgrade -i` if this is a pre-release
version (e.g. `<version>-rc.1`). Full releases will be listed without this.

### Configuring the device plugin's `helm` chart

The `helm` chart for the latest release of the plugin (`v0.14.0`) includes
a number of customizable values.

Prior to `v0.12.0` the most commonly used values were those that had direct
mappings to the command line options of the plugin binary. As of `v0.12.0`, the
preferred method to set these options is via a `ConfigMap`. The primary use
case of the original values is then to override an option from the `ConfigMap`
if desired. Both methods are discussed in more detail below.

The full set of values that can be set are found here:
[here](https://github.com/NVIDIA/k8s-device-plugin/blob/v0.14.0/deployments/helm/nvidia-device-plugin/values.yaml).

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
    --version=0.14.0 \
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
    --version=0.14.0 \
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
    --version=0.14.0 \
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
    --version=0.14.0 \
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
      [envvar | volume-mounts] (default "envvar")
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
  legacyDaemonsetAPI:
      use the legacy daemonset API version 'extensions/v1beta1'
      (default 'false')
  runtimeClassName:
      the runtimeClassName to use, for use with clusters that have multiple runtimes. (typical value is 'nvidia')
```

Please take a look in the
[`values.yaml`](https://github.com/NVIDIA/k8s-device-plugin/blob/v0.14.0/deployments/helm/nvidia-device-plugin/values.yaml)
file to see the full set of overridable parameters for the device plugin.

Examples of setting these options include:

Enabling compatibility with the `CPUManager` and running with a request for
100ms of CPU time and a limit of 512MB of memory.
```shell
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.14.0 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set compatWithCPUManager=true \
    --set resources.requests.cpu=100m \
    --set resources.limits.memory=512Mi
```

Using the legacy Daemonset API (only available on Kubernetes < `v1.16`):
```shell
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.14.0 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set legacyDaemonsetAPI=true
```

Enabling compatibility with the `CPUManager` and the `mixed` `migStrategy`
```shell
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.14.0 \
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
    --version=0.14.0 \
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
    https://nvidia.github.io/k8s-device-plugin/stable/nvidia-device-plugin-0.14.0.tgz
```

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

### Version v0.14.0

- Promote v0.14.0-rc.3 to v0.14.0
- Bumped `nvidia-container-toolkit` dependency to latest version for newer CDI spec generation code

### Version v0.14.0-rc.3

- Removed `--cdi-enabled` config option and instead trigger CDI injection based on `cdi-annotation` strategy.
- Bumped `go-nvlib` dependency to latest version for support of new MIG profiles.
- Added `cdi-annotation-prefix` config option to control how CDI annotations are generated.
- Renamed `driver-root-ctr-path` config option added in `v0.14.0-rc.1` to `container-driver-root`.
- Updated GFD subchart to version 0.8.0-rc.2

### Version v0.14.0-rc.2

- Fix bug from v0.14.0-rc.1 when using cdi-enabled=false

### Version v0.14.0-rc.1

- Added --cdi-enabled flag to GPU Device Plugin. With this enabled, the device plugin will generate CDI specifications for available NVIDIA devices. Allocation will add CDI anntiations (`cdi.k8s.io/*`) to the response. These are read by a CDI-enabled runtime to make the required modifications to a container being created.
- Updated GFD subchart to version 0.8.0-rc.1
- Bumped Golang version to 1.20.1
- Bumped CUDA base images version to 12.1.0
- Switched to klog for logging
- Added a static deployment file for Microshift

### Version v0.13.0

- Promote v0.13.0-rc.3 to v0.13.0
- Fail on startup if no valid resources are detected
- Ensure that display adapters are skipped when enumerating devices
- Bump GFD subchart to version 0.7.0

### Version v0.13.0-rc.3

- Use `nodeAffinity` instead of `nodeSelector` by default in daemonsets
- Mount `/sys` instead of `/sys/class/dmi/id/product_name` in GPU Feature Discovery daemonset
- Bump GFD subchart to version 0.7.0-rc.3

### Version v0.13.0-rc.2

- Bump cuda base image to 11.8.0
- Use consistent indentation in YAML manifests
- Fix bug from v0.13.0-rc.1 when using mig-strategy="mixed"
- Add logged error message if setting up health checks fails
- Support MIG devices with 1g.10gb+me profile
- Distribute replicas evenly across GPUs during allocation
- Bump GFD subchart to version 0.7.0-rc.2

### Version v0.13.0-rc.1

- Improve health checks to detect errors when waiting on device events
- Log ECC error events detected during health check
- Add the GIT sha to version information for the CLI and container images
- Use NVML interfaces from go-nvlib to query devices
- Refactor plugin creation from resources
- Add a CUDA-based resource manager that can be used to expose integrated devices on Tegra-based systems
- Bump GFD subchart to version 0.7.0-rc.1

### Version v0.12.3

- Bump cuda base image to 11.7.1
- Remove CUDA compat libs from the device-plugin image in favor of libs installed by the driver
- Fix securityContext.capabilities indentation
- Add namespace override for multi-namespace deployments

### Version v0.12.2

- Add an 'empty' config fallback (but don't apply it by default)
- Make config fallbacks for config-manager a configurable, ordered list
- Allow an empty config file and default to "version: v1"
- Bump GFD subchart to version 0.6.1
- Move NFD servicAccount info under 'master' in helm chart
- Make priorityClassName configurable through helm
- Fix assertions for panicking on uniformity with migStrategy=single
- Fix example configmap settings in values.yaml file

### Version v0.12.1

- Exit the plugin and GFD sidecar containers on error instead of logging and continuing
- Only force restart of daemonsets when using config file and allow overrides
- Fix bug in calculation for GFD security context in helm chart

### Version v0.12.0

- Promote v0.12.0-rc.6 to v0.12.0
- Update README.md with all of the v0.12.0 features

### Version v0.12.0-rc.6

- Send SIGHUP from GFD sidecar to GFD main container on config change
- Reuse main container's securityContext in sidecar containers
- Update GFD subchart to v0.6.0-rc.1
- Bump CUDA base image version to 11.7.0
- Add a flag called FailRequestsGreaterThanOne for TimeSlicing resources

### Version v0.12.0-rc.5

- Allow either an external ConfigMap name or a set of configs in helm
- Handle cases where no default config is specified to config-manager
- Update API used to pass config files to helm to use map instead of list
- Fix bug that wasn't properly stopping plugins across a soft restart

### Version v0.12.0-rc.4

- Make GFD and NFD (optional) subcharts of the device plugin's helm chart
- Add new config-manager binary to run as sidecar and update the plugin's configuration via a node label
- Add support to helm to provide multiple config files for the config map
- Refactor main to allow configs to be reloaded across a (soft) restart
- Add field for `TimeSlicing.RenameByDefault` to rename all replicated resources to `<resource-name>.shared`
- Disable support for resource-renaming in the config (will no longer be part of this release)

### Version v0.12.0-rc.3

- Add ability to parse Duration fields from config file
- Omit either the Plugin or GFD flags from the config when not present
- Fix bug when falling back to none strategy from single strategy

### Version v0.12.0-rc.2

- Move MigStrategy from Sharing.Mig.Strategy back to Flags.MigStrategy
- Remove timeSlicing.strategy and any allocation policies built around it
- Add support for specifying a config file to the helm chart

### Version v0.12.0-rc.1

- Add API for specifying time-slicing parameters to support GPU sharing
- Add API for specifying explicit resource naming in the config file
- Update config file to be used across plugin and GFD
- Stop publishing images to dockerhub (now only published to nvcr.io)
- Add NVIDIA_MIG_MONITOR_DEVICES=all to daemonset envvars when mig mode is enabled
- Print the plugin configuration at startup
- Add the ability to load the plugin configuration from a file
- Remove deprecated tolerations for critical-pod
- Drop critical-pod annotation(removed from 1.16+) in favor of priorityClassName
- Pass all parameters as env in helm chart and example daemonset.yamls files for consistency

### Version v0.11.0

- Update CUDA base image version to 11.6.0
- Add support for multi-arch images

### Version v0.10.0

- Update CUDA base images to 11.4.2
- Ignore Xid=13 (Graphics Engine Exception) critical errors in device health-check
- Ignore Xid=68 (Video processor exception) critical errors in device health-check
- Build multi-arch container images for linux/amd64 and linux/arm64
- Use Ubuntu 20.04 for Ubuntu-based container images
- Remove Centos7 images

### Version v0.9.0

- Fix bug when using CPUManager and the device plugin MIG mode not set to "none"
- Allow passing list of GPUs by device index instead of uuid
- Move to urfave/cli to build the CLI
- Support setting command line flags via environment variables

### Version v0.8.2

- Update all dockerhub references to nvcr.io

### Version v0.8.1

- Fix permission error when using NewDevice instead of NewDeviceLite when constructing MIG device map

### Version v0.8.0

- Raise an error if a device has migEnabled=true but has no MIG devices
- Allow mig.strategy=single on nodes with non-MIG gpus

### Version v0.7.3

- Update vendoring to include bug fix for `nvmlEventSetWait_v2`

### Version v0.7.2

- Fix bug in dockfiles for ubi8 and centos using CMD not ENTRYPOINT

### Version v0.7.1

- Update all Dockerfiles to point to latest cuda-base on nvcr.io

### Version v0.7.0

- Promote v0.7.0-rc.8 to v0.7.0

### Version v0.7.0-rc.8

- Permit configuration of alternative container registry through environment variables.
- Add an alternate set of gitlab-ci directives under .nvidia-ci.yml
- Update all k8s dependencies to v1.19.1
- Update vendoring for NVML Go bindings
- Move restart loop to force recreate of plugins on SIGHUP

### Version v0.7.0-rc.7

- Fix bug which only allowed running the plugin on machines with CUDA 10.2+ installed

### Version v0.7.0-rc.6

- Add logic to skip / error out when unsupported MIG device encountered
- Fix bug treating memory as multiple of 1000 instead of 1024
- Switch to using CUDA base images
- Add a set of standard tests to the .gitlab-ci.yml file

### Version v0.7.0-rc.5

- Add deviceListStrategyFlag to allow device list passing as volume mounts

### Version v0.7.0-rc.4

- Allow one to override selector.matchLabels in the helm chart
- Allow one to override the udateStrategy in the helm chart

### Version v0.7.0-rc.3

- Fail the plugin if NVML cannot be loaded
- Update logging to print to stderr on error
- Add best effort removal of socket file before serving
- Add logic to implement GetPreferredAllocation() call from kubelet

### Version v0.7.0-rc.2

- Add the ability to set 'resources' as part of a helm install
- Add overrides for name and fullname in helm chart
- Add ability to override image related parameters helm chart
- Add conditional support for overriding secutiryContext in helm chart

### Version v0.7.0-rc.1

- Added `migStrategy` as a parameter to select the MIG strategy to the helm chart
- Add support for MIG with different strategies {none, single, mixed}
- Update vendored NVML bindings to latest (to include MIG APIs)
- Add license in UBI image
- Update UBI image with certification requirements

### Version v0.6.0

- Update CI, build system, and vendoring mechanism
- Change versioning scheme to v0.x.x instead of v1.0.0-betax
- Introduced helm charts as a mechanism to deploy the plugin

### Version v0.5.0

- Add a new plugin.yml variant that is compatible with the CPUManager
- Change CMD in Dockerfile to ENTRYPOINT
- Add flag to optionally return list of device nodes in Allocate() call
- Refactor device plugin to eventually handle multiple resource types
- Move plugin error retry to event loop so we can exit with a signal
- Update all vendored dependencies to their latest versions
- Fix bug that was inadvertently *always* disabling health checks
- Update minimal driver version to 384.81

### Version v0.4.0

- Fixes a bug with a nil pointer dereference around `getDevices:CPUAffinity`

### Version v0.3.0

- Manifest is updated for Kubernetes 1.16+ (apps/v1)
- Adds more logging information

### Version v0.2.0

- Adds the Topology field for Kubernetes 1.16+

### Version v0.1.0

- If gRPC throws an error, the device plugin no longer ends up in a non responsive state.

### Version v0.0.0

- Reversioned to SEMVER as device plugins aren't tied to a specific version of kubernetes anymore.

### Version v1.11

- No change.

### Version v1.10

- The device Plugin API is now v1beta1

### Version v1.9

- The device Plugin API changed and is no longer compatible with 1.8
- Error messages were added

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
