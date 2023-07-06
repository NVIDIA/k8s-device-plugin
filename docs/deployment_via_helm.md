## Deployment via `helm`

The preferred method to deploy the `GPU Feature Discovery` and `Device Plugin` 
is as a daemonset using `helm`. Instructions for installing `helm` can be 
found [here](https://helm.sh/docs/intro/install/).

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
the `gpu-feature-discovery` and `nvidia-device-plugin` helm chart.

The most basic installation command without any options is then:

```shell
helm upgrade -i nvdp nvdp/nvidia-device-plugin \
  --namespace nvidia-device-plugin \
  --create-namespace \
  --version 0.14.0
```

**Note:** As os `v0.14.0`, by default helm will install `NFD` , 
`gpu-feature-discovery` and `nvidia-device-plugin` in the 
`nvidia-device-plugin` namespace. If you want to install them in a different 
namespace, you can use the `--namespace` flag. You can turn off the 
installation of `NFD` and `gpu-feature-discovery` by setting 
`nfd.enabled=false`, `gpuFeatureDiscovery.enabled=false` or 
`devicePlugin.enabled=false`  respectively.

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

**Note:** The following document provides more information on the available MIG
strategies and how they should be used [Supporting Multi-Instance GPUs (MIG) in
Kubernetes](https://docs.google.com/document/d/1mdgMQ8g7WmaI_XVVRrCvHPFPOMCm5LQD5JefgAh6N8g).

Please take a look in the following `values.yaml` files to see the full set of
overridable parameters for both the top-level `gpu-feature-discovery` chart and
the `node-feature-discovery` subchart.

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

```shell
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

```shell
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

```shell
$ kubectl create ns nvidia-device-plugin
```

```shell
$ kubectl create cm -n nvidia-device-plugin nvidia-plugin-configs \
    --from-file=config=/tmp/dp-example-config0.yaml
```

```shell
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.14.0 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set config.name=nvidia-plugin-configs
```

#####  Multiple Config File Example

For multiple config files, the procedure is similar.

Create a second `config` file with the following contents:

```shell
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

```shell
$ helm upgrade -i nvdp nvdp/nvidia-device-plugin \
    --version=0.14.0 \
    --namespace nvidia-device-plugin \
    --create-namespace \
    --set config.default=config0 \
    --set-file config.map.config0=/tmp/dp-example-config0.yaml \
    --set-file config.map.config1=/tmp/dp-example-config1.yaml
```

As before, this can also be done with a pre-created `ConfigMap` if desired:

```shell
$ kubectl create ns nvidia-device-plugin
```

```shell
$ kubectl create cm -n nvidia-device-plugin nvidia-plugin-configs \
    --from-file=config0=/tmp/dp-example-config0.yaml \
    --from-file=config1=/tmp/dp-example-config1.yaml
```

```shell
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

```shell
kubectl label nodes <node-name> –-overwrite \
    nvidia.com/device-plugin.config=<config-name>
```

For example, applying a custom config for all nodes that have T4 GPUs installed
on them might be:

```shell
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

```shell
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

```shell
nvidia.com/<resource-name>.replicas = <num-replicas>
```

Additionally, the `nvidia.com/<resource-name>.product` will be modified as follows if
`renameByDefault=false`.

```shell
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

```shell
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
