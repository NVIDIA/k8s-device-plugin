package nvml

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
	Init()
	defer Shutdown()

	count, err := GetDeviceCount()
	check(err, t)

	query := "count"
	c := nvsmi.DeviceCount(query)

	if c != count {
		t.Errorf("Device Count from nvml is wrong, got %d, want: %d", count, c)
	}
}

func BenchmarkDeviceCount1(b *testing.B) {
	Init()

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		GetDeviceCount()
	}
	b.StopTimer()

	Shutdown()
}

func TestDriverVersion(t *testing.T) {
	Init()
	defer Shutdown()

	driverVersion, err := GetDriverVersion()
	check(err, t)

	// assuming device count check to be passed before this test
	id := "0"
	query := "driver_version"
	res := nvsmi.Query(id, query)

	if strings.Compare(res, driverVersion) != 0 {
		t.Errorf("Driver version from nvml is wrong, got: %v, want: %v", driverVersion, res)
	}
}

func TestDeviceInfo(t *testing.T) {
	Init()
	defer Shutdown()

	fields := []string{
		"uuid",
		"name",
		"pci.bus_id",
		"power.limit",
		"clocks.max.sm",
		"clocks.max.memory",
	}

	count, err := GetDeviceCount()
	check(err, t)

	for i := uint(0); i < count; i++ {
		device, err := NewDevice(i)
		check(err, t)

		id := strconv.FormatUint(uint64(i), 10)

		for _, val := range fields {
			var msg, output string
			res := nvsmi.Query(id, val)

			switch val {
			case "uuid":
				msg = "Device UUID"
				output = device.UUID
			case "name":
				msg = "Device model"
				output = *device.Model
			case "pci.bus_id":
				msg = "Device bus id"
				output = device.PCI.BusID
			case "power.limit":
				msg = "Device power limit"
				output = strconv.FormatUint(uint64(*device.Power), 10)
				power, err := strconv.ParseFloat(res, 64)
				check(err, t)
				res = strconv.FormatUint(uint64(math.Round(power)), 10)
			case "clocks.max.sm":
				msg = "Device max sm clocks"
				output = strconv.FormatUint(uint64(*device.Clocks.Cores), 10)
			case "clocks.max.memory":
				msg = "Device max mem clocks"
				output = strconv.FormatUint(uint64(*device.Clocks.Memory), 10)
			}
			if strings.Compare(res, output) != 0 {
				t.Errorf("%v from nvml is wrong, got: %v, want: %v", msg, output, res)
			}
		}
	}
}

func BenchmarkDeviceInfo1(b *testing.B) {
	Init()

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		// assuming there will be atleast 1 GPU attached
		NewDevice(uint(0))
	}
	b.StopTimer()

	Shutdown()
}

func TestDeviceStatus(t *testing.T) {
	Init()
	defer Shutdown()

	fields := []string{
		"power.draw",
		"temperature.gpu",
		"utilization.gpu",
		"utilization.memory",
		"encoder.stats.averageFps",
		"clocks.current.sm",
		"clocks.current.memory",
		"pstate",
		"ecc.errors.uncorrected.volatile.device_memory",
		"ecc.errors.uncorrected.volatile.l1_cache",
		"ecc.errors.uncorrected.volatile.l2_cache",
	}

	count, err := GetDeviceCount()
	check(err, t)

	for i := uint(0); i < count; i++ {
		device, err := NewDevice(i)
		check(err, t)

		status, err := device.Status()
		check(err, t)

		id := strconv.FormatUint(uint64(i), 10)

		for _, val := range fields {
			var msg, output string = "", "[Not Supported]"
			res := nvsmi.Query(id, val)

			switch val {
			case "power.draw":
				msg = "Device power utilization"
				output = strconv.FormatUint(uint64(*status.Power), 10)
				power, err := strconv.ParseFloat(res, 64)
				check(err, t)
				res = strconv.FormatUint(uint64(power), 10)
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
			case "pstate":
				msg = "Device performance state"
				output = status.Performance.String()
			case "ecc.errors.uncorrected.volatile.device_memory":
				msg = "ecc error in device memory"
				ecc := status.Memory.ECCErrors.Device
				if ecc != nil {
					output = strconv.FormatUint(*ecc, 10)
				}
			case "ecc.errors.uncorrected.volatile.l1_cache":
				msg = "ecc error in l1 cache"
				ecc := status.Memory.ECCErrors.L1Cache
				if ecc != nil {
					output = strconv.FormatUint(*ecc, 10)
				}
			case "ecc.errors.uncorrected.volatile.l2_cache":
				msg = "ecc error in l2 cache"
				ecc := status.Memory.ECCErrors.L2Cache
				if ecc != nil {
					output = strconv.FormatUint(*ecc, 10)
				}
			}
			if strings.Compare(res, output) != 0 {
				t.Errorf("%v from nvml is wrong, got: %v, want: %v", msg, output, res)
			}
		}
	}

}
