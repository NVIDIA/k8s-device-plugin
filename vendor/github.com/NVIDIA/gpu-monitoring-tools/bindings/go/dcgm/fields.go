package dcgm

/*
#include "dcgm_agent.h"
#include "dcgm_structs.h"
*/
import "C"
import (
	"fmt"
)

const (
	updateFreq     = 1000000 // usec
	maxKeepAge     = 300     // sec
	maxKeepSamples = 0       // nolimit
)

type fieldHandle struct{ handle C.dcgmFieldGrp_t }

func fieldGroupCreate(fieldsGroupName string, fields []C.ushort, count int) (fieldsId fieldHandle, err error) {
	var fieldsGroup C.dcgmFieldGrp_t

	groupName := C.CString(fieldsGroupName)
	defer freeCString(groupName)

	result := C.dcgmFieldGroupCreate(handle.handle, C.int(count), &fields[0], groupName, &fieldsGroup)
	if err = errorString(result); err != nil {
		return fieldsId, fmt.Errorf("Error creating DCGM fields group: %s", err)
	}
	fieldsId = fieldHandle{fieldsGroup}
	return
}

func fieldGroupDestroy(fieldsGroup fieldHandle) (err error) {
	result := C.dcgmFieldGroupDestroy(handle.handle, fieldsGroup.handle)
	if err = errorString(result); err != nil {
		fmt.Errorf("Error destroying DCGM fields group: %s", err)
	}
	return
}

func watchFields(gpuId uint, fieldsGroup fieldHandle, groupName string) (groupId groupHandle, err error) {
	group, err := createGroup(groupName)
	if err != nil {
		return
	}

	err = addToGroup(group, gpuId)
	if err != nil {
		return
	}

	result := C.dcgmWatchFields(handle.handle, group.handle, fieldsGroup.handle, C.longlong(updateFreq), C.double(maxKeepAge), C.int(maxKeepSamples))
	if err = errorString(result); err != nil {
		return groupId, fmt.Errorf("Error watching fields: %s", err)
	}

	_ = updateAllFields()
	return group, nil
}

func updateAllFields() error {
	waitForUpdate := C.int(1)
	result := C.dcgmUpdateAllFields(handle.handle, waitForUpdate)
	return errorString(result)
}
