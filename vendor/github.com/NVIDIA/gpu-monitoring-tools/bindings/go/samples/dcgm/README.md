# DCGM Samples

Modelled on [dcgmi (Data Center GPU Manager Interface)](https://developer.nvidia.com/data-center-gpu-manager-dcgm) and [nvidia-smi (NVIDIA System Management Interface)](https://developer.nvidia.com/nvidia-system-management-interface), seven samples and a [REST API](https://github.com/NVIDIA/gpu-monitoring-tools/blob/master/bindings/go/samples/dcgm/restApi/README.md) have been provided to show how to use DCGM go bindings.

## DCGM running modes

DCGM can be run in three different ways.

#### Embedded Mode

In embedded mode, hostengine is started as part of the running process and is loaded as a shared library. In this mode, metrics are also updated and collected automatically. This mode is recommended for users who wants to avoid managing an autonomous hostengine.

#### Standalone Mode

This mode lets you connect to an already running hostengine at a specified TCP/IP or Unix socket address. This mode is recommended for remote connections to the hostengine.  By default, DCGM will assume a TCP connection and attempt to connect to localhost, unless specified.
```
# If hostengine is running at a different address, pass it as

IP - Valid IP address for the remote hostengine to connect to, at port 5555.

IP:PORT - Valid IP address and port

O - Given address is a TCP/IP address

1 - Given address is an Unix socket filename

$ ./sample -connect "IP" -socket "0"

```

#### StartHostengine

This is an add-on mode which opens an Unix socket for starting and connecting with hostengine. The hostengine is started as a child process of the running process and automatically terminated on exit. When operating in this mode, make sure to stop an already running hostengine to avoid any connection address conflicts. This mode is recommended for safely integrating DCGM in an already existing setup.


## Samples


#### deviceInfo

Provides detailed information about each GPU on the system, along with whether the given GPU is DCGM supported or not.

```
$ go build && ./deviceInfo

# sample output

Driver Version         : 384.130
GPU                    : 0
DCGMSupported          : Yes
UUID                   : GPU-34e8d7ba-0e4d-ac00-6852-695d5d404f51
Brand                  : GeForce
Model                  : GeForce GTX 980
Serial Number          : 0324414056639
Vbios                  : 84.04.1F.00.02
InforomImage Version   : G001.0000.01.03
Bus ID                 : 00000000:01:00.0
BAR1 (MB)              : 256
FrameBuffer Memory (MB): 4036
Bandwidth (MB/s)       : 15760
Cores (MHz)            : 1392
Memory (MHz)           : 3505
Power (W)              : 180
CPUAffinity            : 0-11
P2P Available          : None
---------------------------------------------------------------------
```

#### dmon

Monitors each device status including its power, memory and GPU utilization.

```
$ go build && ./dmon

# sample output

Started host engine version 1.4.3 using socket path: /tmp/dcgmrxvqro.socket
# gpu   pwr  temp    sm   mem   enc   dec  mclk  pclk
# Idx     W     C     %     %     %     %   MHz   MHz
    0    43    48     0     1     0     0  3505   936
    0    43    48     0     1     0     0  3505   936
```

#### health

Monitors the health of the given GPU every second, by checking the configured watches for any errors/failures/warnings.

```
$ go build && ./health

# sample output
GPU                : 0
Status             : Healthy
```

#### hostengineStatus

Reports about DCGM hostengine memory and CPU usage.

```
$ go build && ./hostengineStatus

# sample output

Memory  : 11480 KB
CPU     : 0.08 %
```

#### policy

Sets GPU usage and error policies and notifies in case of violations via callback functions.

```
$ go build && ./policy

# sample output

2018/06/25 23:48:34 Policy successfully set.
2018/06/25 23:48:34 Listening for violations...
GPU        : 0
Error      : XID Error
Timestamp  : 2018-06-25 18:55:30 +0000 UTC
Data       : {31}
```

#### processInfo

Provides per GPU detailed stats for this process.

```
$ go build && ./processInfo -pid PID

# sample output

----------------------------------------------------------------------
GPU ID                       : 0
----------Execution Stats---------------------------------------------
PID                          : 15074
Name                         : nbody
Start Time                   : 2018-06-25 16:50:28 -0700 PDT
End Time                     : Still Running
----------Performance Stats-------------------------------------------
Energy Consumed (Joules)     : 181
Max GPU Memory Used (bytes)  : 84279296
Avg SM Clock (MHz)           : N/A
Avg Memory Clock (MHz)       : N/A
Avg SM Utilization (%)       : N/A
Avg Memory Utilization (%)   : N/A
Avg PCIe Rx Bandwidth (MB)   : N/A
Avg PCIe Tx Bandwidth (MB)   : N/A
----------Event Stats-------------------------------------------------
Single Bit ECC Errors        : 0
Double Bit ECC Errors        : 0
Critical XID Errors          : 0
----------Slowdown Stats----------------------------------------------
Due to - Power (%)           : 0
       - Thermal (%)         : 0
       - Reliability (%)     : 0
       - Board Limit (%)     : 0
       - Low Utilization (%) : 0
       - Sync Boost (%)      : 0
----------Process Utilization-----------------------------------------
Avg SM Utilization (%)       : 0
Avg Memory Utilization (%)   : 0
----------------------------------------------------------------------
```

#### topology

Informs about GPU topology and its CPU affinity.

```
$ go build && ./topology

# sample output

Started host engine version 1.4.3 using socket path: /tmp/dcgmvjeqkh.socket
      GPU0CPUAffinity
GPU0    X 0-11

Legend:
 X    = Self
 SYS  = Connection traversing PCIe as well as the SMP interconnect between NUMA nodes (e.g., QPI/UPI)
 NODE = Connection traversing PCIe as well as the interconnect between PCIe Host Bridges within a NUMA node
 PHB  = Connection traversing PCIe as well as a PCIe Host Bridge (typically the CPU)
 PXB  = Connection traversing multiple PCIe switches (without traversing the PCIe Host Bridge)
 PIX  = Connection traversing a single PCIe switch
 PSB  = Connection traversing a single on-board PCIe switch
 NV#  = Connection traversing a bonded set of # NVLinks
 2018/06/25 15:36:38 Successfully terminated nv-hostengine.
```