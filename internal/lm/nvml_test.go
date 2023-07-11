package lm

import (
	"testing"

	"github.com/NVIDIA/gpu-feature-discovery/internal/resource"
	rt "github.com/NVIDIA/gpu-feature-discovery/internal/resource/testing"
	"github.com/stretchr/testify/require"
)

func TestMigCapabilityLabeler(t *testing.T) {
	testCases := []struct {
		description    string
		devices        []resource.Device
		expectedError  bool
		expectedLabels map[string]string
	}{
		{
			description: "no devices returns empty labels",
		},
		{
			description: "single non-mig capable device returns mig.capable as false",
			devices: []resource.Device{
				rt.NewFullGPU(),
			},
			expectedLabels: map[string]string{
				"nvidia.com/mig.capable": "false",
			},
		},
		{
			description: "multiple non-mig capable devices returns mig.capable as false",
			devices: []resource.Device{
				rt.NewFullGPU(),
				rt.NewFullGPU(),
			},
			expectedLabels: map[string]string{
				"nvidia.com/mig.capable": "false",
			},
		},
		{
			description: "single mig capable device returns mig.capable as true",
			devices: []resource.Device{
				rt.NewMigEnabledDevice(),
			},
			expectedLabels: map[string]string{
				"nvidia.com/mig.capable": "true",
			},
		},
		{
			description: "one mig capable device among multiple returns mig.capable as true",
			devices: []resource.Device{
				rt.NewFullGPU(),
				rt.NewMigEnabledDevice(),
			},
			expectedLabels: map[string]string{
				"nvidia.com/mig.capable": "true",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			nvmlMock := rt.NewManagerMockWithDevices(tc.devices...)

			migCapabilityLabeler, _ := newMigCapabilityLabeler(nvmlMock)

			labels, err := migCapabilityLabeler.Labels()
			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.EqualValues(t, tc.expectedLabels, labels)
		})
	}
}
