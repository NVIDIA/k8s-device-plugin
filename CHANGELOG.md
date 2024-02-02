## Changelog

### Version v0.14.4

- Update to refactored go-gpuallocator code. This permanently fixes the NVML_NVLINK_MAX_LINKS value addressed in a
  hotfix in v0.14.3. This also addresses a bug due to uninitialized NVML when calling go-gpuallocator.

### Version v0.14.3

- Patch vendored code for new NVML_NVLINK_MAX_LINKS value
- Bumped CUDA base images version to 12.3.0

### Version v0.14.2

- Update GFD subchart to v0.8.2
- Bumped CUDA base images version to 12.2.2

### Version v0.14.1

- Fix parsing of `deviceListStrategy` in config file to correctly support strings as well as slices.
- Update GFD subchart to v0.8.1
- Bumped CUDA base images version to 12.2.0


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