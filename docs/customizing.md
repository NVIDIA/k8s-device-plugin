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

```yaml
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

```yaml
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

```yaml
version: v1
sharing:
  timeSlicing:
    resources:
    - name: nvidia.com/gpu
      replicas: 10
```

If this configuration were applied to a node with 8 GPUs on it, the plugin
would now advertise 80 `nvidia.com/gpu` resources to Kubernetes instead of 8.

```shell
$ kubectl describe node
...
Capacity:
  nvidia.com/gpu: 80
...
```

Likewise, if the following configuration were applied to a node, then 80
`nvidia.com/gpu.shared` resources would be advertised to Kubernetes instead of 8
`nvidia.com/gpu` resources.

```yaml
version: v1
sharing:
  timeSlicing:
    renameByDefault: true
    resources:
    - name: nvidia.com/gpu
      replicas: 10
    ...
```

```shell
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

```shell
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

```shell
nvidia.com/gpu
```

And the full set of time-sliceable resources on an A100 40GB card would be:

```shell
nvidia.com/gpu
nvidia.com/mig-1g.5gb
nvidia.com/mig-2g.10gb
nvidia.com/mig-3g.20gb
nvidia.com/mig-7g.40gb
```

Likewise, on an A100 80GB card, they would be:

```shell
nvidia.com/gpu
nvidia.com/mig-1g.10gb
nvidia.com/mig-2g.20gb
nvidia.com/mig-3g.40gb
nvidia.com/mig-7g.80gb
```