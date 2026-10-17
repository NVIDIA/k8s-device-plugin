/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/NVIDIA/k8s-device-plugin/internal/plugin"
)

// uniqueSocketFilenamesEnabled decides whether each plugin start listens on
// its own socket. auto reads the node's kubelet version and enables the
// suffix only for 1.37 or newer. Any lookup failure keeps the stable name,
// because a unique name on an older kubelet drops the resource when the
// previous socket disappears.
func uniqueSocketFilenamesEnabled(ctx context.Context, mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "true", "1":
		klog.Info("Unique device plugin socket filenames enabled by flag")
		return true
	case "false", "0":
		klog.Info("Unique device plugin socket filenames disabled by flag")
		return false
	case "", "auto":
		return kubeletVersionAllowsUniqueSockets(ctx)
	default:
		klog.Warningf("Invalid --unique-socket-filenames %q; using stable socket names", mode)
		return false
	}
}

func kubeletVersionAllowsUniqueSockets(ctx context.Context) bool {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Info("NODE_NAME is unset; using stable device plugin socket names")
		return false
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Infof("No in-cluster Kubernetes client (%v); using stable device plugin socket names", err)
		return false
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Infof("Failed to create Kubernetes client (%v); using stable device plugin socket names", err)
		return false
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	node, err := client.CoreV1().Nodes().Get(lookupCtx, nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Infof("Failed to read kubelet version for node %s (%v); using stable device plugin socket names", nodeName, err)
		return false
	}

	kubeletVersion := node.Status.NodeInfo.KubeletVersion
	enabled := plugin.KubeletSupportsUniqueSockets(kubeletVersion)
	klog.Infof("Kubelet version %q; unique device plugin socket filenames: %t", kubeletVersion, enabled)
	return enabled
}
