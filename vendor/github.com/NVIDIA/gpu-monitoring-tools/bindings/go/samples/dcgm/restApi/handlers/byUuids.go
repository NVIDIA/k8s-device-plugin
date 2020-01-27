package handlers

import (
	"log"
	"net/http"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

// map of uuids and device id
var uuids map[string]uint

func DevicesUuids() {
	uuids = make(map[string]uint)
	count, err := dcgm.GetAllDeviceCount()
	if err != nil {
		log.Printf("(DCGM) Error getting devices: %s", err)
		return
	}

	for i := uint(0); i < count; i++ {
		deviceInfo, err := dcgm.GetDeviceInfo(i)
		if err != nil {
			log.Printf("(DCGM) Error getting device information: %s", err)
			return
		}
		uuids[deviceInfo.UUID] = i
	}
}

func DeviceInfoByUuid(resp http.ResponseWriter, req *http.Request) {
	device := getDeviceInfo(resp, req)
	if device == nil {
		return
	}
	if isJson(req) {
		encode(resp, req, device)
		return
	}
	print(resp, req, device, deviceInfo)
}

func DeviceStatusByUuid(resp http.ResponseWriter, req *http.Request) {
	st := getDeviceStatus(resp, req)
	if st == nil {
		return
	}
	if isJson(req) {
		encode(resp, req, st)
		return
	}
	print(resp, req, st, deviceStatus)
}

func HealthByUuid(resp http.ResponseWriter, req *http.Request) {
	h := getHealth(resp, req)
	if h == nil {
		return
	}
	if isJson(req) {
		encode(resp, req, h)
		return
	}
	print(resp, req, h, healthStatus)
}
