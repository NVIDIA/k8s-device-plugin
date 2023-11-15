// Copyright (c) 2019, NVIDIA CORPORATION. All rights reserved.

package gpuallocator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/NVIDIA/go-gpuallocator/internal/links"
	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvlib/pkg/nvml"
)

// Device represents a GPU device as reported by NVML, including all of its
// Point-to-Point link information.
type Device struct {
	nvlibDevice
	Index int
	Links map[int][]P2PLink
}

type nvlibDevice struct {
	device.Device
	// The previous binding implementation used to cache specific device properties.
	// These should be considered deprecated and the functions associated with device.Device
	// should be used instead.
	UUID string
	PCI  struct {
		BusID string
	}
	CPUAffinity *uint
}

// newDevice constructs a Device for the specified index and nvml Device.
func newDevice(i int, d device.Device) (*Device, error) {
	uuid, ret := d.GetUUID()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device uuid: %v", ret)
	}
	pciInfo, ret := d.GetPciInfo()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device pci info: %v", ret)
	}

	device := Device{
		nvlibDevice: nvlibDevice{
			Device:      d,
			UUID:        uuid,
			PCI:         struct{ BusID string }{BusID: links.PciInfo(pciInfo).BusID()},
			CPUAffinity: links.PciInfo(pciInfo).CPUAffinity(),
		},
		Index: i,
		Links: make(map[int][]P2PLink),
	}

	return &device, nil
}

// P2PLink represents a Point-to-Point link between two GPU devices. The link
// is between the Device struct this struct is embedded in and the GPU Device
// contained in the P2PLink struct itself.
type P2PLink struct {
	GPU  *Device
	Type links.P2PLinkType
}

// DeviceList stores an ordered list of devices.
type DeviceList []*Device

// DeviceSet is used to hold and manipulate a set of unique GPU devices.
type DeviceSet map[string]*Device

// NewDevices creates a list of Devices from all available nvml.Devices using the specified options.
func NewDevices(opts ...Option) (DeviceList, error) {
	o := &deviceListBuilder{}
	for _, opt := range opts {
		opt(o)
	}
	if o.nvmllib == nil {
		o.nvmllib = nvml.New()
	}
	if o.devicelib == nil {
		o.devicelib = device.New(
			device.WithNvml(o.nvmllib),
		)
	}

	return o.build()
}

// build uses the configured options to build a DeviceList.
func (o *deviceListBuilder) build() (DeviceList, error) {
	if err := o.nvmllib.Init(); err != nvml.SUCCESS {
		return nil, fmt.Errorf("error calling nvml.Init: %v", err)
	}
	defer func() {
		_ = o.nvmllib.Shutdown()
	}()

	nvmlDevices, err := o.devicelib.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %v", err)
	}

	var devices DeviceList
	for i, d := range nvmlDevices {
		device, err := newDevice(i, d)
		if err != nil {
			return nil, fmt.Errorf("failed to construct linked device: %v", err)
		}
		devices = append(devices, device)
	}

	for i, d1 := range nvmlDevices {
		for j, d2 := range nvmlDevices {
			if i != j {
				p2plink, err := links.GetP2PLink(d1, d2)
				if err != nil {
					return nil, fmt.Errorf("error getting P2PLink for devices (%v, %v): %v", i, j, err)
				}
				if p2plink != links.P2PLinkUnknown {
					devices[i].Links[j] = append(devices[i].Links[j], P2PLink{devices[j], p2plink})
				}

				nvlink, err := links.GetNVLink(d1, d2)
				if err != nil {
					return nil, fmt.Errorf("error getting NVLink for devices (%v, %v): %v", i, j, err)
				}
				if nvlink != links.P2PLinkUnknown {
					devices[i].Links[j] = append(devices[i].Links[j], P2PLink{devices[j], nvlink})
				}
			}
		}
	}

	return devices, nil
}

// NewDevicesFrom creates a list of Devices from the specific set of GPU uuids passed in.
func NewDevicesFrom(uuids []string) (DeviceList, error) {
	devices, err := NewDevices()
	if err != nil {
		return nil, err
	}
	return devices.Filter(uuids)
}

// Filter filters out the selected devices from the list.
// If the supplied list of uuids is nil, no filtering is performed.
// Note that the specified uuids must exist in the list of devices.
func (d DeviceList) Filter(uuids []string) (DeviceList, error) {
	if uuids == nil {
		return d, nil
	}

	filtered := []*Device{}
	for _, uuid := range uuids {
		for _, device := range d {
			if device.UUID == uuid {
				filtered = append(filtered, device)
				break
			}
		}
		if len(filtered) == 0 || filtered[len(filtered)-1].UUID != uuid {
			return nil, fmt.Errorf("no device with uuid: %v", uuid)
		}
	}

	return filtered, nil
}

// String returns a compact representation of a Device as string of its index.
func (d *Device) String() string {
	return fmt.Sprintf("%v", d.Index)
}

// Details returns all details of a Device as a multi-line string.
func (d *Device) Details() string {
	s := ""
	s += fmt.Sprintf("Device %v:\n", d.Index)
	s += fmt.Sprintf("  UUID: %v\n", d.UUID)
	s += fmt.Sprintf("  PCI BusID: %v\n", d.PCI.BusID)
	s += fmt.Sprintf("  SocketAffinity: %v\n", *d.CPUAffinity)
	s += fmt.Sprintf("  Topology: \n")
	for gpu, links := range d.Links {
		s += fmt.Sprintf("    GPU %v Links:\n", gpu)
		for _, link := range links {
			s += fmt.Sprintf("      %v\n", link.Type)
		}
	}

	return strings.TrimSuffix(s, "\n")
}

// NewDeviceSet creates a new DeviceSet.
func NewDeviceSet(devices ...*Device) DeviceSet {
	set := make(DeviceSet)
	set.Insert(devices...)
	return set
}

// Insert inserts a list of devices into a DeviceSet.
func (ds DeviceSet) Insert(devices ...*Device) {
	for _, device := range devices {
		ds[device.UUID] = device
	}
}

// Delete deletes a list of devices from a DeviceSet.
func (ds DeviceSet) Delete(devices ...*Device) {
	for _, device := range devices {
		delete(ds, device.UUID)
	}
}

// Contains checks if a device is present in a DeviceSet.
func (ds DeviceSet) Contains(device *Device) bool {
	if device == nil {
		return false
	}

	_, ok := ds[device.UUID]
	return ok
}

// ContainsAll checks if a list of devices is present in a DeviceSet.
func (ds DeviceSet) ContainsAll(devices []*Device) bool {
	if len(devices) > len(ds) {
		return false
	}

	for _, d := range devices {
		if !ds.Contains(d) {
			return false
		}
	}

	return true
}

// SortedSlice etunrs returns a slice of devices,
// sorted by device index from a DeviceSet.
func (ds DeviceSet) SortedSlice() []*Device {
	devices := make([]*Device, 0, len(ds))

	for _, device := range ds {
		devices = append(devices, device)
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Index < devices[j].Index
	})

	return devices
}
