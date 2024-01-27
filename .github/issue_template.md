---
name: Device Plugin Bug report
about: Create a report to help us improve
title: ''
labels: ''

---

_The template below is mostly useful for bug reports and support questions. Feel free to remove anything which doesn't apply to you and add more information where it makes sense._

_**Important Note:  NVIDIA AI Enterprise customers can get support from NVIDIA Enterprise support. Please open a case [here](https://enterprise-support.nvidia.com/s/create-case)**._


### 1. Quick Debug Information
* OS/Version(e.g. RHEL8.6, Ubuntu22.04):
* Kernel Version:
* Container Runtime Type/Version(e.g. Containerd, CRI-O, Docker):
* K8s Flavor/Version(e.g. K8s, OCP, Rancher, GKE, EKS):

### 2. Issue or feature description
_Briefly explain the issue in terms of expected behavior and current behavior._

### 3. Information to [attach](https://help.github.com/articles/file-attachments-on-issues-and-pull-requests/) (optional if deemed irrelevant)

Common error checking:
 - [ ] The output of `nvidia-smi -a` on your host
 - [ ] Your docker configuration file (e.g: `/etc/docker/daemon.json`)
 - [ ] The k8s-device-plugin container logs
 - [ ] The kubelet logs on the node (e.g: `sudo journalctl -r -u kubelet`)

Additional information that might help better understand your environment and reproduce the bug:
 - [ ] Docker version from `docker version`
 - [ ] Docker command, image and tag used
 - [ ] Kernel version from `uname -a`
 - [ ] Any relevant kernel output lines from `dmesg`
 - [ ] NVIDIA packages version from `dpkg -l '*nvidia*'` _or_ `rpm -qa '*nvidia*'`
 - [ ] NVIDIA container library version from `nvidia-container-cli -V`

