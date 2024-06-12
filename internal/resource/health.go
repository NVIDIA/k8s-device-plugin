package resource

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	DevicePluginConfigPath  = "/etc/nvidia-device-plugin/"
	IsolatedDevicesFilePath = "/etc/nvidia-device-plugin/unhealthyDevices.json"
	HealthyServerPort       = "7123"
)

type UnhealthyDevices struct {
	GPUIndex []string `json:"index"`
	GPUUuid  []string `json:"uuid"`
}

type HealthServer struct {
	httpServer *http.Server
	mux        *http.ServeMux
}

func NewHealthServer(portString string) (*HealthServer, error) {
	port, err := strconv.Atoi(portString)
	if err != nil {
		log.Println("Port set for health server is invalid.")
		return nil, err
	}
	if port > 65535 || port < 1 {
		return nil, fmt.Errorf("port set for health server is invalid, it should be in [1, 65535]")
	}

	healthServer := &HealthServer{
		httpServer: &http.Server{
			Addr: fmt.Sprintf(":%v", port),
		},
		mux: http.NewServeMux(),
	}
	healthServer.init()

	return healthServer, nil
}

func (h *HealthServer) init() {
	h.mux.HandleFunc("/health", h.serveHealthyHandler)
	h.httpServer.Handler = h.mux
}

func (h *HealthServer) Serve() error {
	return h.httpServer.ListenAndServe()
}

func (h *HealthServer) serveHealthyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func FindUnhealthyDevices() (*UnhealthyDevices, error) {
	_, err := os.Stat(IsolatedDevicesFilePath)
	if os.IsNotExist(err) {
		return nil, nil
	}

	unhealthyDevices := UnhealthyDevices{}
	// To wait for write file
	time.Sleep(3 * time.Second)

	jsonData, err := ioutil.ReadFile(IsolatedDevicesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s", IsolatedDevicesFilePath)
	}

	err = json.Unmarshal(jsonData, &unhealthyDevices)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal json file %s", IsolatedDevicesFilePath)
	}

	return &unhealthyDevices, nil
}
