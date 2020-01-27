package nvsmi

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

const (
	bin       = "nvidia-smi"
	gpuArg    = "--id="
	queryArg  = "--query-gpu="
	formatArg = "--format=csv,noheader,nounits"
)

func Query(id string, query string) string {
	var out bytes.Buffer

	cmd := exec.Command(bin, gpuArg+id, queryArg+query, formatArg)
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		fmt.Errorf("nvsmi exec error: %v\n", err)
	}
	return strings.TrimSpace(out.String())
}

func DeviceCount(query string) uint {
	var out bytes.Buffer

	cmd := exec.Command(bin, queryArg+query, formatArg)
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		fmt.Errorf("nvsmi exec error: %v\n", err)
	}

	nvSmi := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	return uint(len(nvSmi))
}
