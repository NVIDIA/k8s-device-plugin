package main

import (
	"github.com/gpucloud/gohwloc/topology"
	"k8s.io/klog"
)

type pciDevice struct {
	pciType      string
	avialDevices int
	score        float64
	children     []*pciDevice
	dev          *topology.HwlocObject
}

func (dp *NvidiaDevicePlugin) buildPciDeviceTree() error {
	dp.root = &pciDevice{
		pciType: "root",
	}
	t, err := topology.NewTopology()
	if err != nil {
		return err
	}
	t.Load()
	defer t.Destroy()
	n, err := t.GetNbobjsByType(topology.HwlocObjPackage)
	if err != nil {
		return err
	}
	dp.root.children = make([]*pciDevice, n)
	for i := 0; i < n; i++ {
		nno, err := t.GetObjByType(topology.HwlocObjPackage, uint(i))
		if err != nil {
			klog.Warningf("topology get object by type error: %v", err)
			continue
		}
		buildTree(dp.root.children[i], nno)
	}

	return nil
}

func buildTree(node *pciDevice, dev *topology.HwlocObject) {
	if dev == nil {
		return
	}
	if node == nil {
		node = &pciDevice{
			dev: dev,
		}
	}
	node.children = make([]*pciDevice, len(dev.Children))
	for i := 0; i < len(dev.Children); i++ {
		buildTree(node.children[i], dev.Children[i])
	}
}

func (dp *NvidiaDevicePlugin) findBestDevice(t string, n int) []string {
	devs := []string{}
	switch t {
	case resourceName:
		return devs
	}

	return devs
}
