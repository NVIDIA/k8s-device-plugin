## Using CDI with the NVIDIA Device Plugin

The GPU Device plugin can be configured to use the [Container Device Interface (CDI)](https://tags.cncf.io/container-device-interface)
to specify which devices need to be injected into a container once a device is
allocated.

This may resolve issues around a container losing access to devices under container
updates. These typically manifest as:
```
NVML: Unknown error
```
in the container.

### Prerequisites

1. The host container runtime must be CDI-enabled. This includes `containerd`
   1.7 and newer, and CRI-O 1.24 and newer.
2. The nvidia runtime should _not_ be the default runtime, but it must still be
   installed, and configured as an available runtime. See the instructions for:
    * [containerd](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#configuring-containerd)
    * [CRI-O](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#configuring-cri-o)
3. A [Runtime Class](https://kubernetes.io/docs/concepts/containers/runtime-class/) is created and associated
with the `nvidia` runtime.

### Configuration

Two things need to be considered here, ensuring that the GPU Device Plugin container has access to
NVIDIA GPU drivers and devices, and ensuring that CDI specifications are generated and devices are
requested using CDI. This can be done by including the following arguments in the Helm install command:
* `--set runtimeClassName=nvidia`: ensures that the device plugin is started with the `nvidia` runtime and has access to the NVIDIA GPU driver and devices.
* `--set nvidiaDriverRoot=/` (or `--set nvidiaDriverRoot=/run/nvidia/driver` if the driver container is used): ensures that the driver files are available to generate the correct CDI specifications.
* `--set deviceListStrategy=cdi-annotations`: configures annotations to be used to request CDI devices from the CDI-enabled container engine instead of the `NVIDIA_VISIBLE_DEVICES` environment variable.

Note that other utility pods such as the DCGM exporter must also be configured to use the `nvidia` RuntimeClass instead of relying on the `nvidia` runtime being configured as the default.