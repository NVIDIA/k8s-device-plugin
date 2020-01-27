## DCGM REST API

A sample REST API is provided, demonstrating various endpoints for getting GPU metrics via DCGM.


```
# Start the http server
# By default the http server is started at localhost:8070

$ go build && ./restApi

# Query GPU 0 info
$ GPUID=0
$ curl localhost:8070/dcgm/device/info/id/$GPUID

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

$ curl localhost:8070/dcgm/device/info/id/$GPUID/json

# Query GPU info using its UUID

$ UUID=$(curl -s localhost:8070/dcgm/device/info/id/$GPUID | grep -i uuid | cut -d ":" -f2 )
$ curl localhost:8070/dcgm/device/info/uuid/$UUID
$ curl localhost:8070/dcgm/device/info/uuid/$UUID/json

# sample output

{"GPU":0,"DCGMSupported":"Yes","UUID":"GPU-34e8d7ba-0e4d-ac00-6852-695d5d404f51","Power":180,"PCI":{"BusID":"00000000:01:00.0","BAR1":256,"FBTotal":4036,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Identifiers":{"Brand":"GeForce","Model":"GeForce GTX 980","Serial":"0324414056639","Vbios":"84.04.1F.00.02","InforomImageVersion":"G001.0000.01.03","DriverVersion":"384.130"},"Topology":null,"CPUAffinity":"0-11"}

# Query GPU status

$ curl localhost:8070/dcgm/device/status/id/$GPUID
$ curl localhost:8070/dcgm/device/status/id/$GPUID/json

# sample output

Power (W)               : 20.985
Temperature (°C)        : 47
Sm Utilization (%)      : 2
Memory Utilization (%)  : 8
Encoder Utilization (%) : 0
Decoder Utilization (%) : 0
Memory Clock (MHz       : 324
SM Clock (MHz)          : 135

$ curl localhost:8070/dcgm/device/status/uuid/$UUID

# sample output

{"Power":20.793,"Temperature":43,"Utilization":{"GPU":0,"Memory":8,"Encoder":0,"Decoder":0},"Memory":{"GlobalUsed":null,"ECCErrors":{"SingleBit":9223372036854775794,"DoubleBit":9223372036854775794}},"Clocks":{"Cores":135,"Memory":324},"PCI":{"BAR1Used":9,"Throughput":{"Rx":129,"Tx":47,"Replays":0},"FBUsed":423},"Performance":8,"FanSpeed":29}

$ curl localhost:8070/dcgm/device/status/uuid/$UUID/json

# Query GPU process info

# Run CUDA nbody sample and get its PID
$ PID=$(pgrep nbody)

$ curl localhost:8070/dcgm/process/info/pid/$PID
$ curl localhost:8070/dcgm/process/info/pid/$PID/json

# sample output

{"GPU":0,"PID":19132,"Name":"nbody","ProcessUtilization":{"StartTime":1529980640,"EndTime":0,"EnergyConsumed":1346,"SmUtil":0,"MemUtil":0},"PCI":{"BAR1Used":null,"Throughput":{"Rx":null,"Tx":null,"Replays":0},"FBUsed":null},"Memory":{"GlobalUsed":84279296,"ECCErrors":{"SingleBit":0,"DoubleBit":0}},"GpuUtilization":{"GPU":null,"Memory":null,"Encoder":null,"Decoder":null},"Clocks":{"Cores":null,"Memory":null},"Violations":{"Power":0,"Thermal":0,"Reliability":0,"BoardLimit":0,"LowUtilization":0,"SyncBoost":0},"XIDErrors":{"NumErrors":0,"TimeStamp":[]}}

# Query GPU health

$ curl localhost:8070/dcgm/health/id/$GPUID
$ curl localhost:8070/dcgm/health/id/$GPUID/json
$ curl localhost:8070/dcgm/health/uuid/$UUID
$ curl localhost:8070/dcgm/health/uuid/$UUID/json

# sample output

{"GPU":0,"Status":"Healthy","Watches":[]}

# Query DCGM hostengine memory and CPU usage

$ curl localhost:8070/dcgm/status
$ curl localhost:8070/dcgm/status/json

# sample output

{"Memory":18380,"CPU":0.16482222745467387}

```