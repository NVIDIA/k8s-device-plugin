package topology

// #cgo LDFLAGS: -lhwloc
// #include <hwloc.h>
/*
uint64_t get_obj_local_memory(hwloc_obj_t obj) {
	return obj->attr->numanode.local_memory;
}

uint32_t get_obj_page_types_len(hwloc_obj_t obj) {
	return obj->attr->numanode.page_types_len;
}

struct hwloc_cache_attr_s get_obj_cache_attr(hwloc_obj_t obj) {
	return obj->attr->cache;
}

struct hwloc_group_attr_s get_obj_group_attr(hwloc_obj_t obj) {
	return obj->attr->group;
}
struct hwloc_pcidev_attr_s get_obj_pcidev_attr(hwloc_obj_t obj) {
	return obj->attr->pcidev;
}
struct hwloc_bridge_attr_s get_obj_bridge_attr(hwloc_obj_t obj) {
	return obj->attr->bridge;
}
struct hwloc_osdev_attr_s get_obj_osdev_attr(hwloc_obj_t obj) {
	return obj->attr->osdev;
}
*/
import "C"
import (
	"unsafe"
)

// NewHwlocObject create a HwlocObject based on C.hwloc_obj_t
func NewHwlocObject(obj C.hwloc_obj_t) (*HwlocObject, error) {
	if obj == nil {
		return nil, nil
	}
	hobj := &HwlocObject{
		Type:            HwlocObjType(obj._type),
		SubType:         C.GoString(obj.subtype),
		OSIndex:         uint(obj.os_index),
		Name:            C.GoString(obj.name),
		TotalMemory:     uint64(obj.total_memory),
		Depth:           int(obj.depth),
		LogicalIndex:    uint(obj.logical_index),
		CPUSet:          NewCPUSet(obj.cpuset),
		NextCousin:      &HwlocObject{},
		PrevCousin:      &HwlocObject{},
		Parent:          &HwlocObject{},
		SiblingRank:     uint(obj.sibling_rank),
		NextSibling:     &HwlocObject{},
		PrevSibling:     &HwlocObject{},
		CompleteCPUSet:  &HwlocCPUSet{},
		NodeSet:         &HwlocNodeSet{},
		CompleteNodeSet: &HwlocNodeSet{},
		private:         obj,
	}
	if obj.attr != nil {
		hobj.Attributes = &HwlocObjAttr{}
		hobj.Attributes.NumaNode = &HwlocNumaNodeAttr{
			LocalMemory:     uint64(C.get_obj_local_memory(obj)),
			PageTypesLength: uint(C.get_obj_page_types_len(obj)),
		}
		cache := C.get_obj_cache_attr(obj)
		hobj.Attributes.Cache = &HwlocCacheAttr{
			Size:          uint64(cache.size),
			Depth:         uint(cache.depth),
			LineSize:      uint(cache.linesize),
			Associativity: int(cache.associativity),
			Type:          HwlocObjCacheType(cache._type),
		}
		group := C.get_obj_group_attr(obj)
		hobj.Attributes.Group = &HwlocGroupAttr{
			Depth:   uint(group.depth),
			Kind:    uint(group.kind),
			SubKind: uint(group.subkind),
		}
		pcidev := C.get_obj_pcidev_attr(obj)
		hobj.Attributes.PCIDev = &HwlocPCIDevAttr{
			Domain:      uint16(pcidev.domain),
			Bus:         uint8(pcidev.bus),
			Dev:         uint8(pcidev.dev),
			Func:        uint8(pcidev._func),
			ClassID:     uint16(pcidev.class_id),
			VendorID:    uint16(pcidev.vendor_id),
			DeviceID:    uint16(pcidev.device_id),
			SubVendorID: uint16(pcidev.subvendor_id),
			SubDeviceID: uint16(pcidev.subdevice_id),
			Revision:    uint8(pcidev.revision),
			LinkSpeed:   float32(pcidev.linkspeed),
		}
		bridge := C.get_obj_bridge_attr(obj)
		hobj.Attributes.Bridge = &HwlocBridgeAttr{
			UpstreamType:   HwlocObjBridgeType(bridge.upstream_type),
			DownStreamType: HwlocObjBridgeType(bridge.downstream_type),
			Depth:          uint(bridge.depth),
			// TODO: UpstreamPCI, DownStreamPCIDomain, DownStreamPCISecondaryBus, DownStreamPCISubordinateBus
		}
		osdev := C.get_obj_osdev_attr(obj)
		hobj.Attributes.OSDevType = HwlocObjOSDevType(osdev._type)
	}
	if po := obj.parent; po != nil {
		hobj.Parent, _ = NewHwlocObject(po)
	}
	return hobj, nil
}

// GetInfo Search the given key name in object infos and return the corresponding value.
/*
 * If multiple keys match the given name, only the first one is returned.
 *
 * \return \c NULL if no such key exists.
 */
func (o *HwlocObject) GetInfo(name string) (string, error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	res := C.hwloc_obj_get_info_by_name(o.hwloc_obj_t(), cname)
	return C.GoString(res), nil
}

// hwloc_obj_t Return C.hwloc_obj_t for HwlocObject
func (o *HwlocObject) hwloc_obj_t() C.hwloc_obj_t {
	return o.private
}

// AddInfo Add the given info name and value pair to the given object.
/*
 * The info is appended to the existing info array even if another key
 * with the same name already exists.
 *
 * The input strings are copied before being added in the object infos.
 *
 * \return \c 0 on success, \c -1 on error.
 *
 * \note This function may be used to enforce object colors in the lstopo
 * graphical output by using "lstopoStyle" as a name and "Background=#rrggbb"
 * as a value. See CUSTOM COLORS in the lstopo(1) manpage for details.
 *
 * \note If \p value contains some non-printable characters, they will
 * be dropped when exporting to XML, see hwloc_topology_export_xml() in hwloc/export.h.
 */
func (o *HwlocObject) AddInfo(name, value string) error {
	cname := C.CString(name)
	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cname))
	defer C.free(unsafe.Pointer(cvalue))
	ret := C.hwloc_obj_add_info(o.hwloc_obj_t(), cname, cvalue)
	_ = ret
	return nil
}

// String Return a constant stringified object type.
// This function is the basic way to convert a generic type into a string.
// The output string may be parsed back by hwloc_type_sscanf().
func (t HwlocObjType) String() string {
	switch t {
	case HwlocObjMachine:
		return "Machine"
	case HwlocObjPackage:
		return "Package"
	case HwlocObjCore:
		return "Core"
	case HwlocObjPU:
		return "PU"
	case HwlocObjL1Cache:
		return "L1Cache"
	case HwlocObjL2Cache:
		return "L2Cache"
	case HwlocObjL3Cache:
		return "L3Cache"
	case HwlocObjL4Cache:
		return "L4Cache"
	case HwlocObjL5Cache:
		return "L5Cache"
	case HwlocObjL1ICache:
		return "L1iCache"
	case HwlocObjL2ICache:
		return "L2iCache"
	case HwlocObjL3ICache:
		return "L3iCache"
	case HwlocObjGroup:
		return "Group"
	case HwlocObjNumaNode:
		return "NUMANode"
	case HwlocObjBridge:
		return "Bridge"
	case HwlocObjPCIDevice:
		return "PCIDev"
	case HwlocObjOSDevice:
		return "OSDev"
	case HwlocObjMisc:
		return "Misc"
	default:
		return "Unknown"
	}
}
