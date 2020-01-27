package main

import (
	"context"
	"log"
	"net/http"
	"time"

	h "github.com/NVIDIA/gpu-monitoring-tools/bindings/go/samples/dcgm/restApi/handlers"
	"github.com/gorilla/mux"
)

const timeout = 5 * time.Second

type httpServer struct {
	router *mux.Router
	server *http.Server
}

func newHttpServer(addr string) *httpServer {
	r := mux.NewRouter()

	s := &httpServer{
		router: r,
		server: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  timeout,
			WriteTimeout: timeout,
		},
	}

	// make a global map of device uuids and ids
	h.DevicesUuids()

	s.handler()
	return s
}

func (s *httpServer) handler() {
	deviceInfo := "/dcgm/device/info"
	subrouter := s.router.PathPrefix(deviceInfo).Subrouter()
	subrouter.HandleFunc("/id/{id}", h.DeviceInfo).Methods("GET")
	subrouter.HandleFunc("/id/{id}/json", h.DeviceInfo).Methods("GET")
	subrouter.HandleFunc("/uuid/{uuid}", h.DeviceInfoByUuid).Methods("GET")
	subrouter.HandleFunc("/uuid/{uuid}/json", h.DeviceInfoByUuid).Methods("GET")

	deviceStatus := "/dcgm/device/status"
	subrouter = s.router.PathPrefix(deviceStatus).Subrouter()
	subrouter.HandleFunc("/id/{id}", h.DeviceStatus).Methods("GET")
	subrouter.HandleFunc("/id/{id}/json", h.DeviceStatus).Methods("GET")
	subrouter.HandleFunc("/uuid/{uuid}", h.DeviceStatusByUuid).Methods("GET")
	subrouter.HandleFunc("/uuid/{uuid}/json", h.DeviceStatusByUuid).Methods("GET")

	processInfo := "/dcgm/process/info/pid/{pid}"
	subrouter = s.router.PathPrefix(processInfo).Subrouter()
	subrouter.HandleFunc("", h.ProcessInfo).Methods("GET")
	subrouter.HandleFunc("/json", h.ProcessInfo).Methods("GET")

	health := "/dcgm/health"
	subrouter = s.router.PathPrefix(health).Subrouter()
	subrouter.HandleFunc("/id/{id}", h.Health).Methods("GET")
	subrouter.HandleFunc("/id/{id}/json", h.Health).Methods("GET")
	subrouter.HandleFunc("/uuid/{uuid}", h.HealthByUuid).Methods("GET")
	subrouter.HandleFunc("/uuid/{uuid}/json", h.HealthByUuid).Methods("GET")

	dcgmStatus := "/dcgm/status"
	subrouter = s.router.PathPrefix(dcgmStatus).Subrouter()
	subrouter.HandleFunc("", h.DcgmStatus).Methods("GET")
	subrouter.HandleFunc("/json", h.DcgmStatus).Methods("GET")
}

func (s *httpServer) serve() {
	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("Error: %v", err)
	}
}

func (s *httpServer) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("Error: %v", err)
	} else {
		log.Println("http server stopped")
	}
}
