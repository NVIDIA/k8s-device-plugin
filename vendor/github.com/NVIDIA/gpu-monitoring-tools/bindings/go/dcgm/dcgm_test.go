package dcgm

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml/nvsmi"
)

func check(err error, t *testing.T) {
	if err != nil {
		t.Errorf("%v\n", err)
	}
}

func TestDeviceCount(t *testing.T) {
	check(Init(Embedded), t)
	defer func() { check(Shutdown(), t) }()

	count, err := GetAllDeviceCount()
	check(err, t)

	query := "count"
	c := nvsmi.DeviceCount(query)

	if c != count {
		t.Errorf("Device Count from dcgm is wrong, got %d, want: %d", count, c)
	}
}

func BenchmarkDeviceCount1(b *testing.B) {
	Init(Embedded)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		GetAllDeviceCount()
	}
	b.StopTimer()

	Shutdown()
}

func TestDeviceInfo(t *testing.T) {
	check(Init(Embedded), t)
	defer func() { check(Shutdown(), t) }()

	fields := []string{
		"driver_version",
		"name",
		"serial",
		"uuid",
		"pci.bus_id",
		"vbios_version",
		"inforom.img",
		"clocks.max.sm",
		"clocks.max.memory",
		"power.limit",
	}

	gpus, err := GetSupportedDevices()
	check(err, t)

	for _, gpu := range gpus {
		info, err := GetDeviceInfo(gpu)
		check(err, t)

		id := strconv.FormatUint(uint64(gpu), 10)

		for _, val := range fields {
			var msg, output string
			res := nvsmi.Query(id, val)

			switch val {
			case "driver_version":
				msg = "Driver version"
				output = info.Identifiers.DriverVersion
			case "name":
				msg = "Device name"
				output = info.Identifiers.Model
			case "serial":
				msg = "Device Serial number"
				output = info.Identifiers.Serial
			case "uuid":
				msg = "Device UUID"
				output = info.UUID
			case "pci.bus_id":
				msg = "Device PCI busId"
				output = info.PCI.BusID
			case "vbios_version":
				msg = "Device vbios version"
				output = info.Identifiers.Vbios
			case "inforom.img":
				msg = "Device inforom image"
				output = info.Identifiers.InforomImageVersion
			case "clocks.max.sm":
				msg = "Device sm clock"
				output = strconv.FormatUint(uint64(*info.Clocks.Cores), 10)
			case "clocks.max.memory":
				msg = "Device mem clock"
				output = strconv.FormatUint(uint64(*info.Clocks.Memory), 10)
			case "power.limit":
				msg = "Device power limit"
				output = strconv.FormatUint(uint64(*info.Power), 10)
				power, err := strconv.ParseFloat(res, 64)
				check(err, t)
				res = strconv.FormatUint(uint64(math.Round(power)), 10)
			}

			if strings.Compare(res, output) != 0 {
				t.Errorf("%v from dcgm is wrong, got: %v, want: %v", msg, output, res)
			}
		}
	}
}

func BenchmarkDeviceInfo1(b *testing.B) {
	Init(Embedded)

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		// assuming there will be atleast 1 GPU attached
		GetDeviceInfo(uint(0))
	}
	b.StopTimer()

	Shutdown()
}

func TestDeviceStatus(t *testing.T) {
	check(Init(Embedded), t)
	defer func() { check(Shutdown(), t) }()

	gpus, err := GetSupportedDevices()
	check(err, t)

	fields := []string{
		"power.draw",
		"temperature.gpu",
		"utilization.gpu",
		"utilization.memory",
		"encoder.stats.averageFps",
		"clocks.current.sm",
		"clocks.current.memory",
	}

	for _, gpu := range gpus {
		status, err := GetDeviceStatus(gpu)
		check(err, t)

		id := strconv.FormatUint(uint64(gpu), 10)

		for _, val := range fields {
			var msg, output string
			res := nvsmi.Query(id, val)

			switch val {
			case "power.draw":
				msg = "Device power utilization"
				output = strconv.FormatUint(uint64(math.Round(*status.Power)), 10)
				power, err := strconv.ParseFloat(res, 64)
				check(err, t)
				res = strconv.FormatUint(uint64(math.Round(power)), 10)
			case "temperature.gpu":
				msg = "Device temperature"
				output = strconv.FormatUint(uint64(*status.Temperature), 10)
			case "utilization.gpu":
				msg = "Device gpu utilization"
				output = strconv.FormatUint(uint64(*status.Utilization.GPU), 10)
			case "utilization.memory":
				msg = "Device memory utilization"
				output = strconv.FormatUint(uint64(*status.Utilization.Memory), 10)
			case "encoder.stats.averageFps":
				msg = "Device encoder utilization"
				output = strconv.FormatUint(uint64(*status.Utilization.Encoder), 10)
			case "clocks.current.sm":
				msg = "Device sm clock"
				output = strconv.FormatUint(uint64(*status.Clocks.Cores), 10)
			case "clocks.current.memory":
				msg = "Device mem clock"
				output = strconv.FormatUint(uint64(*status.Clocks.Memory), 10)
			}

			if strings.Compare(res, output) != 0 {
				t.Errorf("%v from dcgm is wrong, got: %v, want: %v", msg, output, res)
			}
		}
	}
}
