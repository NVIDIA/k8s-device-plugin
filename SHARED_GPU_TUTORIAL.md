# Tutorial

We will deploy two types of daemonsets via the helm chart.  One to support sharing of GPUs and one to support exclusive (regular non-shared GPUs).

Label GPU nodes as either shared or exclusive.

```shell
kubectl label node nodename1 gpu-node-type=exclusive
...
kubectl label node nodename2 gpu-node-type=shared
...
```

Deploy the driver twice.  Once for the exclusive GPU nodes and once for the shared GPU nodes based on the above node labels.

```shell
helm install nvidia-exclusive deployments/helm/nvidia-device-plugin --set nodeSelector='gpu-node-type=exclusive'
helm install nvidia-exclusive deployments/helm/nvidia-device-plugin --set nodeSelector='gpu-node-type=shared' --set resourceConfig='gpu:sharedgpu:4'
```

You can use [kubectl-view-allocations](https://github.com/davidB/kubectl-view-allocations) to see that you now have two extended resource types.  The tool will also tell you the quantity in use and available.

- nvidia.com/gpu
- nvidia.com/sharedgpu

Create two pods with shared GPUs that do some work on the GPUs.

```bash
kubectl create -f pods/pod-shared-pytorch.yml
kubectl create -f pods/pod-shared-pytorch.yml
```

Inspect the logs of the pods to see that both are using pytorch with CUDA support to train a DNN model on MNIST.  
You can also run `nvidia-smi -L` to see that the GPU is the same (given that they are actually sharing a GPU and didn't pick up different GPUs if you have more than one GPU on your system).

Note that in the pods, `nvidia-smi` only lists that pod's processes (yet the percentage is actually for all processes) but from the host everything for a given GPU.

You can also create pods with exclusive ownership by replacing "nvidia.com/sharedgpu" with "nvidia.com/gpu" as shown in this [example pod](pods/pod1.yml).
