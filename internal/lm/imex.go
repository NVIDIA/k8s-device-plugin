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
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/resource"
)

func newImexLabeler(config *spec.Config, devices []resource.Device) (Labeler, error) {
	clusterUUID, cliqueID, err := getFabricIDs(devices)
	if err != nil {
		return nil, err
	}
	if clusterUUID == "" || cliqueID == "" {
		return empty{}, nil
	}

	labels := Labels{
		"nvidia.com/gpu.clique": strings.Join([]string{clusterUUID, cliqueID}, "."),
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
