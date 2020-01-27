package handlers

import (
	"net/http"
)

func DeviceInfo(resp http.ResponseWriter, req *http.Request) {
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

func DeviceStatus(resp http.ResponseWriter, req *http.Request) {
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

func ProcessInfo(resp http.ResponseWriter, req *http.Request) {
	pInfo := getProcessInfo(resp, req)
	if len(pInfo) == 0 {
		return
	}
	if isJson(req) {
		encode(resp, req, pInfo)
		return
	}
	processPrint(resp, req, pInfo)
}

func Health(resp http.ResponseWriter, req *http.Request) {
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

func DcgmStatus(resp http.ResponseWriter, req *http.Request) {
	st := getDcgmStatus(resp, req)
	if st == nil {
		return
	}
	if isJson(req) {
		encode(resp, req, st)
		return
	}
	print(resp, req, st, hostengine)
}
