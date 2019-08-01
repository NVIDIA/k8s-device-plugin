package topology

// #cgo LDFLAGS: -lhwloc
// #include <hwloc.h>
import "C"

// HwlocCPUSet A CPU set is a bitmap whose bits are set according to CPU physical OS indexes.
/*
 * It may be consulted and modified with the bitmap API as any
 * ::hwloc_bitmap_t (see hwloc/bitmap.h).
 *
 * Each bit may be converted into a PU object using
 * hwloc_get_pu_obj_by_os_index().
 */
type HwlocCPUSet struct {
	BitMap
}

// NewCPUSet create a HwlocCPUSet instance based on hwloc_cpuset_t
func NewCPUSet(cpuset C.hwloc_cpuset_t) *HwlocCPUSet {
	var bm BitMap = NewBitmap(cpuset)
	return &HwlocCPUSet{
		BitMap: bm,
	}
}

func (set HwlocCPUSet) hwloc_cpuset_t() C.hwloc_cpuset_t {
	return set.BitMap.bm
}

// Destroy free the HwlocCPUSet object
func (set HwlocCPUSet) Destroy() {
	set.BitMap.Destroy()
}
