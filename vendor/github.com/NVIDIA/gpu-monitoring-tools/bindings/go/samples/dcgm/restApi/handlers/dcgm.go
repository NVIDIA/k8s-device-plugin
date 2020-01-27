package handlers

import (
	"log"
	"math"
	"net/http"
	"time"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
	"github.com/gorilla/mux"
)

func getDcgmStatus(resp http.ResponseWriter, req *http.Request) (status *dcgm.DcgmStatus) {
	st, err := dcgm.Introspect()
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		log.Printf("error: %v%v: %v", req.Host, req.URL, err.Error())
		return
	}
	return &st

}

func getDeviceInfo(resp http.ResponseWriter, req *http.Request) (device *dcgm.Device) {
	var id uint
	params := mux.Vars(req)
	for k, v := range params {
		switch k {
		case "id":
			id = getId(resp, req, v)
		case "uuid":
			id = getIdByUuid(resp, req, v)
		}
	}

	if id == math.MaxUint32 {
		return
	}

	if !isValidId(id, resp, req) {
		return
	}
	d, err := dcgm.GetDeviceInfo(id)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		log.Printf("error: %v%v: %v", req.Host, req.URL, err.Error())
		return
	}
	return &d
}

func getDeviceStatus(resp http.ResponseWriter, req *http.Request) (status *dcgm.DeviceStatus) {
	var id uint
	params := mux.Vars(req)
	for k, v := range params {
		switch k {
		case "id":
			id = getId(resp, req, v)
		case "uuid":
			id = getIdByUuid(resp, req, v)
		}
	}

	if id == math.MaxUint32 {
		return
	}

	if !isValidId(id, resp, req) {
		return
	}

	if !isDcgmSupported(id, resp, req) {
		return
	}

	st, err := dcgm.GetDeviceStatus(id)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		log.Printf("error: %v%v: %v", req.Host, req.URL, err.Error())
		return
	}
	return &st
}

func getHealth(resp http.ResponseWriter, req *http.Request) (health *dcgm.DeviceHealth) {
	var id uint
	params := mux.Vars(req)
	for k, v := range params {
		switch k {
		case "id":
			id = getId(resp, req, v)
		case "uuid":
			id = getIdByUuid(resp, req, v)
		}
	}

	if id == math.MaxUint32 {
		return
	}

	if !isValidId(id, resp, req) {
		return
	}

	h, err := dcgm.HealthCheckByGpuId(id)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		log.Printf("error: %v%v: %v", req.Host, req.URL, err.Error())
		return
	}
	return &h
}

func getProcessInfo(resp http.ResponseWriter, req *http.Request) (pInfo []dcgm.ProcessInfo) {
	params := mux.Vars(req)
	pid := getId(resp, req, params["pid"])
	if pid == math.MaxUint32 {
		return
	}
	group, err := dcgm.WatchPidFields()
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		log.Printf("error: %v%v: %v", req.Host, req.URL, err.Error())
		return
	}

	// wait for watches to be enabled
	log.Printf("Enabling DCGM watches to start collecting process stats. This may take a few seconds....")
	time.Sleep(3000 * time.Millisecond)
	pInfo, err = dcgm.GetProcessInfo(group, pid)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		log.Printf("error: %v%v: %v", req.Host, req.URL, err.Error())
	}
	return
}
