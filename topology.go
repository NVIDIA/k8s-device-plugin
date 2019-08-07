package main

import (
	"github.com/gpucloud/gohwloc/topology"
	"k8s.io/klog"
)

type pciDevice struct {
	pciType      string
	maxDevices   int
	avialDevices int
	score        float64
	nvidiaUUID   string
	children     []*pciDevice
	dev          *topology.HwlocObject
}

// Destroy destroy the device plugin's topology object
func (dp *NvidiaDevicePlugin) Destroy() {
	if dp.topo != nil {
		dp.topo.Destroy()
	}
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
		dp.root.children[i] = &pciDevice{pciType: nno.Type.String(), dev: nno}
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
			pciType: dev.Type.String(),
			dev:     dev,
		}
	}
	node.children = make([]*pciDevice, len(dev.Children))
	for i := 0; i < len(dev.Children); i++ {
		node.children[i] = &pciDevice{
			pciType: dev.Children[i].Type.String(),
			dev:     dev.Children[i],
		}
		buildTree(node.children[i], dev.Children[i])
	}
}

func updateTree(node *pciDevice) (maxDevices, availDevices int, sum float64) {
	if node == nil {
		return 0, 0, 0.0
	}
	if node.dev != nil && node.dev.Attributes.OSDevType == topology.HwlocObjOSDevGPU {
		maxDevices = 1
		availDevices = node.availDevices
		if availDevices == 1 {
			sum += 100
		}
		node.nvidiaUUID, _ = node.dev.GetInfo("NVIDIAUUID")
	}
	for i := 0; i < len(node.children); i++ {
		tmpMax, tmpAvail, tmpSum := updateTree(node.children[i])
		maxDevices += tmpMax
		availDevices += tmpAvail
		sum += tmpSum
	}
	var factor = 1.0
	if len(node.children) > 1 {
		switch node.pciType {
		case "Bridge":
			factor = 0.9
		case "Package":
			factor = 0.7
		}
	}
	return maxDevices, availDevices, sum * factor
}

func printDeviceTree(node *pciDevice) {
	if node == nil {
		return
	}
	if node.dev != nil {
		backend, _ := node.dev.GetInfo("Backend")
		gpuid, _ := node.dev.GetInfo("NVIDIAUUID")
		klog.Infof("%v, %v, %v, %v, %#v\n", node.pciType, node.dev.Name, backend, gpuid, node.dev.Attributes.OSDevType)
	}
	for i := 0; i < len(node.children); i++ {
		printDeviceTree(node.children[i])
	}
}

func (dp *NvidiaDevicePlugin) findBestDevice(t string, n int) []string {
	devs := []string{}
	switch t {
	case resourceName:
		// XXX: we divide the user's request into two parts:
		// a. request 1 GPU card, select the best 1 GPU card, make sure the left GPU cards will be most valuable
		// b. request more than 1 GPU card, based on the score of the least enough leaves branch
		if n == 1 {
			// request 1 GPU card, select the best 1 GPU card,
			// make sure the left GPU cards will be most valuable
			devs = append(devs, dp.find1GPUDevice())
		} else {
			// find the least enough leaves node
			// find the higher score when the two nodes have same number leaves
			// add the leaves into the result
			devs = append(devs, dp.findNGPUDevice()...)
		}
		return devs
	}

	return devs
}

func (dp *NvidiaDevicePlugin) find1GPUDevice() string {
	// if the current node has maximum GPU devices, select the first one
	// else find the one to make sure left GPU devices have highest score
	// FIXME: consider GPU connect type
	var max float64
	var maxUUID string
	for _, gpu := range dp.devs {
		// find the GPU uuid in the PCI device tree
		var pdev *pciDevice
		pdev = dp.findDeviceFromGPU(gpu)
		if pdev == nil {
			continue
		}
		// mark it as used
		pdev.availDevices = 0
		// count the score of machine with left GPU cards
		_, _, tmp := updateTree(dp.root)
		if tmp > max {
			max = tmp
			maxUUID = gpu
		}
	}
	return maxUUID
}

func (dp *NvidiaDevicePlugin) findNGPUDevice(n int) []string {
	return []string{}
}

func (dp *NvidiaDevicePlugin) findDeviceFromGPU(id string) *pciDevice {
	if dp.root == nil {
		return nil
	}
	var queue = []*pciDevice{dp.root}
	for len(queue) > 0 {
		l = len(queue)
		for i := 0; i < l; i++ {
			if queue[i].availDevices == 0 {
				continue
			}
			if queue[i].nvidiaUUID == id {
				return queue[i]
			}
			for _, c := range queue[i].children {
				if c.availDevices == 0 {
					continue
				}
				queue = append(queue, c)
			}
		}
		queue = queue[l:]
	}
	return nil
}
