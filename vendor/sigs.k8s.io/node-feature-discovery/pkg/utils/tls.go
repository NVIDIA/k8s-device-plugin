/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
)

// TlsConfig is a TLS config wrapper/helper for cert rotation
type TlsConfig struct {
	sync.Mutex
	config *tls.Config
}

// GetConfig returns the current TLS configuration. Intended to be used as the
// GetConfigForClient callback in tls.Config.
func (c *TlsConfig) GetConfig(*tls.ClientHelloInfo) (*tls.Config, error) {
	c.Lock()
	defer c.Unlock()

	return c.config, nil
}

// UpdateConfig updates the wrapped TLS config
func (c *TlsConfig) UpdateConfig(certFile, keyFile, caFile string) error {
	c.Lock()
	defer c.Unlock()

	// Load cert for authenticating this server
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load server certificate: %v", err)
	}
	// Load CA cert for client cert verification
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("failed to read root certificate file: %v", err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCert); !ok {
		return fmt.Errorf("failed to add certificate from '%s'", caFile)
	}

	// Create TLS config
	c.config = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientCAs:          caPool,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		GetConfigForClient: c.GetConfig,
		MinVersion:         tls.VersionTLS13,
	}
	return nil
}
