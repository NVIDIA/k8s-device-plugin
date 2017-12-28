# Deploy GPU Enabled Applications in Kubernetes

## Table of Contents

- [About](#about)
- [Prerequisites](#prerequisites)
- [GPU Enabled TensorFlow Jupyter Notebook](#gpu-enabled-tensorflow-jupyter-notebook)

# About

This directory lists a few examples that showcases how to use GPU enabled
applications in kubernetes.
We currently have the following examples:
- [TensorFlow 1.4.1 Jupyter notebook](https://hub.docker.com/r/tensorflow/tensorflow)

## Prerequisites

The list of prerequisites for running GPU enabled Applications:
* NVIDIA drivers ~= 361.93
* nvidia-docker version > 2.0 (see how to [install](https://github.com/NVIDIA/nvidia-docker) and it's [prerequisites](https://github.com/nvidia/nvidia-docker/wiki/Installation-\(version-2.0\)#prerequisites))
* docker configured with nvidia as the [default runtime](https://github.com/NVIDIA/nvidia-docker/wiki/Advanced-topics#default-runtime).
* Kubernetes version = 1.9
* The `DevicePlugins` feature gate enabled

[Quick start guide is available at the root of the repository.](https://github.com/NVIDIA/k8s-device-plugin)

# GPU Enabled TensorFlow Jupyter Notebook

## Deploy

You can deploy a TensorFlow Jupyter notebook by running the following command:
```shell
$ kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v1.9/examples/tensorflow-notebook.yml
```

This creates a tensorflow deployment with one replicas. You can see if the pods are running by typing the following command:
```shell
$ kubectl get pods
NAME                           READY     STATUS    RESTARTS   AGE
tf-notebook-747db6987b-vz4wj   1/1       Running   0          1h
```

Copy the name of the pod and run the following command to see where the pod landed:
```shell
$ kubectl describe pods tf-notebook-747db6987b-vz4wj | grep 'Node:'
```

Finally check the logs of the TensorFlow Jupyter notebook to get the login token for the first time
```shell
$ kubectl logs tf-notebook-747db6987b-vz4wj
[I 18:09:16.111 NotebookApp] Writing notebook server cookie secret to /root/.local/share/jupyter/runtime/notebook_cookie_secret
[W 18:09:16.120 NotebookApp] WARNING: The notebook server is listening on all IP addresses and not using encryption. This is not recommended.
[I 18:09:16.125 NotebookApp] Serving notebooks from local directory: /notebooks
[I 18:09:16.126 NotebookApp] 0 active kernels
[I 18:09:16.126 NotebookApp] The Jupyter Notebook is running at:
[I 18:09:16.126 NotebookApp] http://[all ip addresses on your system]:8888/?token=d48fd1d869305f2ab231a203a2f005b97b7e371def3c434f
[I 18:09:16.126 NotebookApp] Use Control-C to stop this server and shut down all kernels (twice to skip confirmation).
[C 18:09:16.126 NotebookApp]

    Copy/paste this URL into your browser when you connect for the first time,
    to login with a token:
      http://localhost:8888/?token=d48fd1d869305f2ab231a203a2f005b97b7e371def3c434f
```

You should now be able to access your notebook at http://YOUR_NODE_IP:8888/?token=YOUR_NOTEBOOK_TOKEN

## Usage

You should now be able to check that GPUs are available in the notebook by running the following code:
```python
from tensorflow.python.client import device_lib

def get_available_gpus():
    local_device_protos = device_lib.list_local_devices()
    return [x.name for x in local_device_protos if x.device_type == 'GPU']

print(get_available_gpus())
```
