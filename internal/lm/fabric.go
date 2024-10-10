/**
# Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package lm

import (
	"bufio"
	"fmt"
	"io"
	"math/rand" // nolint:gosec
	"net"
	"os"
	"sort"
	"strings"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/resource"

	"github.com/google/uuid"
	"k8s.io/klog/v2"
)

func newImexLabeler(config *spec.Config, devices []resource.Device) (Labeler, error) {
	if config.Flags.GFD.ImexNodesConfigFile == nil || *config.Flags.GFD.ImexNodesConfigFile == "" {
		// No imex config file, return empty labels
		return empty{}, nil
	}

	imexConfigFile, err := os.Open(*config.Flags.GFD.ImexNodesConfigFile)
	if os.IsNotExist(err) {
		// No imex config file, return empty labels
		return empty{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to open imex config file: %v", err)
	}
	defer imexConfigFile.Close()

	clusterUUID, cliqueID, err := getFabricIDs(devices)
	if err != nil {
		return nil, err
	}
	if clusterUUID == "" || cliqueID == "" {
		return empty{}, nil
	}

	imexDomainID, err := getImexDomainID(imexConfigFile)
	if err != nil {
		return nil, err
	}
	if imexDomainID == "" {
		return empty{}, nil
	}

	labels := Labels{
		"nvidia.com/gpu.clique":      strings.Join([]string{clusterUUID, cliqueID}, "."),
		"nvidia.com/gpu.imex-domain": strings.Join([]string{imexDomainID, cliqueID}, "."),
	}

	return labels, nil
}

func getFabricIDs(devices []resource.Device) (string, string, error) {
	uniqueClusterUUIDs := make(map[string][]int)
	uniqueCliqueIDs := make(map[string][]int)
	for i, device := range devices {
		isFabricAttached, err := device.IsFabricAttached()
		if err != nil {
			return "", "", fmt.Errorf("error checking imex capability: %v", err)
		}
		if !isFabricAttached {
			continue
		}

		clusterUUID, cliqueID, err := device.GetFabricIDs()
		if err != nil {

			return "", "", fmt.Errorf("error getting fabric IDs: %w", err)
		}

		uniqueClusterUUIDs[clusterUUID] = append(uniqueClusterUUIDs[clusterUUID], i)
		uniqueCliqueIDs[cliqueID] = append(uniqueCliqueIDs[cliqueID], i)
	}

	if len(uniqueClusterUUIDs) > 1 {
		klog.Warningf("Cluster UUIDs are non-unique: %v", uniqueClusterUUIDs)
		return "", "", nil
	}

	if len(uniqueCliqueIDs) > 1 {
		klog.Warningf("Clique IDs are non-unique: %v", uniqueCliqueIDs)
		return "", "", nil
	}

	for clusterUUID := range uniqueClusterUUIDs {
		for cliqueID := range uniqueCliqueIDs {
			return clusterUUID, cliqueID, nil
		}
	}
	return "", "", nil
}

// getImexDomainID reads the imex config file and returns a unique identifier
// based on the sorted list of IP addresses in the file.
func getImexDomainID(r io.Reader) (string, error) {
	// Read the file line by line
	var ips []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if net.ParseIP(ip) == nil {
			return "", fmt.Errorf("invalid IP address in imex config file: %s", ip)
		}
		ips = append(ips, ip)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read imex config file: %v", err)
	}

	if len(ips) == 0 {
		// No IPs in the file, return empty labels
		return "", nil
	}

	sort.Strings(ips)

	return generateContentUUID(strings.Join(ips, "\n")), nil

}

func generateContentUUID(seed string) string {
	// nolint:gosec
	rand := rand.New(rand.NewSource(hash(seed)))
	charset := make([]byte, 16)
	rand.Read(charset)
	uuid, _ := uuid.FromBytes(charset)
	return uuid.String()
}

func hash(s string) int64 {
	h := int64(0)
	for _, c := range s {
		h = 31*h + int64(c)
	}
	return h
}
