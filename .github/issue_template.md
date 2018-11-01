_The template below is mostly useful for bug reports and support questions. Feel free to remove anything which doesn't apply to you and add more information where it makes sense._
---

### 1. Issue or feature description

### 2. Steps to reproduce the issue

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
 - [ ] NVIDIA container library logs (see [troubleshooting](https://github.com/NVIDIA/nvidia-docker/wiki/Troubleshooting))
