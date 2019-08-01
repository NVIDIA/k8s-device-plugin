package topology

// #cgo LDFLAGS: -lhwloc
// #include <hwloc.h>
// #include <hwloc/bitmap.h>
import "C"
import "unsafe"

// BitMap is a struct containing a slice of bytes,
// being used as a bitmap.
type BitMap struct {
	bm C.hwloc_bitmap_t
}

// NewBitmap returns a BitMap. It requires a size. A bitmap with a size of
// eight or less will be one byte in size, and so on.
func NewBitmap(set C.hwloc_bitmap_t) BitMap {
	b := BitMap{
		bm: set,
	}
	if set == nil {
		b.bm = C.hwloc_bitmap_alloc()
	}
	return b
}

// Destroy free the BitMap
func (b BitMap) Destroy() {
	if b.bm != nil {
		C.hwloc_bitmap_free(b.bm)
	}
}

// Set sets a position in
// the bitmap to 1.
func (b BitMap) Set(i uint64) error {
	C.hwloc_bitmap_set(b.bm, C.uint(i))
	return nil
}

// Unset sets a position in
// the bitmap to 0.
func (b BitMap) Unset(i uint64) error {
	C.hwloc_bitmap_clr(b.bm, C.uint(i))
	return nil
}

// Values returns a slice of ints
// represented by the values in the bitmap.
func (b BitMap) Values() ([]uint64, error) {
	list := make([]uint64, 0)
	return list, nil
}

// IsSet returns a boolean indicating whether the bit is set for the position in question.
func (b BitMap) IsSet(i uint64) (bool, error) {
	if C.hwloc_bitmap_isset(b.bm, C.uint(i)) == 1 {
		return true, nil
	}
	return false, nil
}

// IsZero Test whether bitmap is empty
// return 1 if bitmap is empty, 0 otherwise.
func (b BitMap) IsZero() (bool, error) {
	if C.hwloc_bitmap_iszero(b.bm) == 1 {
		return true, nil
	}
	return false, nil
}

func (b BitMap) String() string {
	var bitmap = C.CString("")
	defer C.free(unsafe.Pointer(bitmap))
	C.hwloc_bitmap_asprintf(&bitmap, b.bm)
	var res = C.GoString(bitmap)
	return res
}

func FromString(input string) (BitMap, error) {
	return NewBitmap(nil), nil
}
