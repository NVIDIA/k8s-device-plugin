# NVIDIA device plugin for Kubernetes
For details from official Nvidia device plugin repository, please visit https://github.com/NVIDIA/k8s-device-plugin

# How GPU resource be shared by MPS in Linux ?

<ul>
<li>GPU compute mode will be set to EXCLUSIVE_PROCESS to ensure any process request to use GPU will need to talk to MPS control daemon and MPS server</li>
<li>By default, each MPS client process can be access up to 100% memory and 100% available threads of GPUs</li>
<li>MPS resource can be limited from MPS control daemon level, MPS client level to CUDA context level: https://docs.nvidia.com/deploy/mps/#performance</li>
</ul>
<br/>
refer: https://docs.nvidia.com/deploy/mps

# Strategies to provisioning resource in MPS
refer to https://docs.nvidia.com/deploy/mps/#performance:
<ul>
<li>A common provisioning strategy is to uniformly partition the available threads equally to each MPS client processes - <strong>this is how NVDP devs implemented MPS</strong></li>
<li>A more optimal strategy is to uniformly partition the portion by half of the number of expected clients</li>
<li>The near optimal provision strategy is to non-uniformly partition the available threads based on the workloads of each MPS clients (i.e., set active thread percentage to 30% for client 1 and set active thread percentage to 70 % client 2 if the ratio of the client1 workload and the client2 workload is 30%: 70%) - <strong>this is what i want</strong> </li>
<li>The most optimal provision strategy is to precisely limit the number of SMs to use for each MPS clients knowing the execution resource requirements of each client</li>
</ul>

# How did the main branch of nvidia device plugin implemented MPS?
<ul>
<li>NVDP devs just set hard limit at control daemon level, by 100/n for both memory and threads, with n is the number of replicas </li>
<li>I think it will be so inconvenient for us to use MPS</li>
</ul>
# My solution
<ol>
<li>I will remove the hard limit 100/n be set at control daemon level</li>
<li>Instead, i wll set resource limit for each container will use MPS in Kubernetes  by two environment variable: CUDA_MPS_ACTIVE_THREAD_PERCENTAGE and CUDA_MPS_PINNED_DEVICE_MEM_LIMIT </li>
<li>By that way, the resource provisioning of MPS in NVDP will be very flexible, because each container will be provided the number of threads and memory as it need, was that so nice? </li>
<ol>