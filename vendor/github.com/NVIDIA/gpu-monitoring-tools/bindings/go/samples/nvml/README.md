## NVML Samples

Modelled on the [NVIDIA System Management Interface (nvidia-smi)](https://developer.nvidia.com/nvidia-system-management-interface), a commnad line utility using NVML, three samples have been provided to show how to use NVML go bindings.

#### deviceInfo

Provides basic information about each GPU on the system.

```
$ go build && ./deviceInfo

# sample output

Driver Version : 384.111
GPU            : 0
UUID           : GPU-34e8d7ba-0e4d-ac00-6852-695d5d404f51
Model          : GeForce GTX 980
Path           : /dev/nvidia0
Power          : 180 W
CPU Affinity   : NUMA node0
Bus ID         : 00000000:01:00.0
BAR1           : 256 MiB
Bandwidth      : 15760 MB/s
Cores          : 1392 MHz
Memory         : 3505 MHz
P2P Available  : None
---------------------------------------------------------------------
GPU            : 1
UUID           : GPU-8d3b966d-2248-c3f4-1784-49851a1d02b3
Model          : GeForce GTX TITAN
Path           : /dev/nvidia1
Power          : 250 W
CPU Affinity   : NUMA node0
Bus ID         : 00000000:06:00.0
BAR1           : 128 MiB
Bandwidth      : 8000 MB/s
Cores          : 1202 MHz
Memory         : 3004 MHz
P2P Available  : None
---------------------------------------------------------------------
```

#### dmon

Monitors each device status including its power, memory and GPU utilization.

```
$ go build && ./dmon

# sample output

# gpu   pwr  temp    sm   mem   enc   dec  mclk  pclk
# Idx     W     C     %     %     %     %   MHz   MHz
    0    20    43     0     8     0     0   324   135
    1    10    32     0     0     0     0   324   324

```

#### processInfo

Informs about GPU processes running on all devices.

```
$ go build && ./processInfo

# sample output

# gpu     pid   type   mem   command
# Idx       #    C/G     %   name
    0   25712    C+G     0   nbody
    1       -      -     -   -
```
