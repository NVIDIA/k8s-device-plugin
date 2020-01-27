package dcgm

/*
#include <stdlib.h>
#include "dcgm_structs.h"
*/
import "C"

import (
	"fmt"
	"math"
	"unsafe"
)

const (
	dcgmInt32Blank = 0x7ffffff0         // 2147483632
	dcgmInt64Blank = 0x7ffffffffffffff0 // 9223372036854775792
)

func uintPtr(c C.uint) *uint {
	i := uint(c)
	return &i
}

func uintPtrInt(c C.int) *uint {
	i := uint(c)
	return &i
}

func uintPtrUnsafe(p unsafe.Pointer) *uint {
	if p == nil {
		return nil
	}
	uintP := (*uint)(unsafe.Pointer(p))
	val := *uintP
	return &val
}

func uint64Ptr(c C.longlong) *uint64 {
	i := uint64(c)
	return &i
}

func uint64PtrUint(c C.uint) *uint64 {
	i := uint64(c)
	return &i
}

func uint64PtrUnsafe(p unsafe.Pointer) *uint64 {
	if p == nil {
		return nil
	}
	uintP := (*uint64)(unsafe.Pointer(p))
	val := *uintP
	return &val
}

func toInt64(c C.longlong) int64 {
	i := int64(c)
	return i
}

func dblToUint(val C.double) *uint {
	i := uint(val)
	return &i
}

func dblToFloat(val C.double) *float64 {
	i := float64(val)
	return &i
}

func dblToFloatUnsafe(val unsafe.Pointer) *float64 {
	if val == nil {
		return nil
	}
	dblP := (*C.double)(unsafe.Pointer(val))
	floatP := float64(*dblP)
	return &floatP
}

func stringPtr(c *C.char) *string {
	s := C.GoString(c)
	return &s
}

func errorString(result C.dcgmReturn_t) error {
	if result == C.DCGM_ST_OK {
		return nil
	}
	err := C.GoString(C.errorString(result))
	return fmt.Errorf("%v", err)
}

func freeCString(cStr *C.char) {
	C.free(unsafe.Pointer(cStr))
}

func isInt32Blank(value int) bool {
	if value >= dcgmInt32Blank {
		return true
	}
	return false
}

func isInt64Blank(value int64) bool {
	if value >= dcgmInt64Blank {
		return true
	}
	return false
}

func blank64(val *uint64) *uint64 {
	if val != nil && isInt64Blank(int64(*val)) {
		return nil
	}
	return val
}

func blank32(val *uint) *uint {
	if val != nil && isInt32Blank(int(*val)) {
		return nil
	}
	return val
}

func makeVersion1(struct_type uintptr) C.uint {
	version := C.uint(struct_type | 1<<24)
	return version
}

func makeVersion2(struct_type uintptr) C.uint {
	version := C.uint(struct_type | 2<<24)
	return version
}

func roundFloat(f *float64) *float64 {
	var val float64
	if f != nil {
		val = math.Round(*f)
	}
	return &val
}
