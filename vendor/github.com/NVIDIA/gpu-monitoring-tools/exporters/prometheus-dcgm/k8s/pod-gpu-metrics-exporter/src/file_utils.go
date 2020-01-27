// Copyright (c) 2018, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"fmt"
	"os"
)

func writeDestFile(tmpF string, destFile string) error {
	err := os.Rename(tmpF, destFile)
	if err != nil {
		return fmt.Errorf("error replacing temp file with %s: %v", destFile, err)
	}

	// Set read permissions
	mode := os.FileMode(0644)
	err = os.Chmod(destFile, mode)
	if err != nil {
		return fmt.Errorf("error setting %s file permissions: %v", destFile, err)
	}
	return nil
}

func createMetricsDir(dir string) error {
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("err creating directory %s: %v", dir, err)
	}
	return nil
}
