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

package diagnostics

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	nfdclient "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
)

const (
	// the core group
	Pods       = "pods"
	Nodes      = "nodes"
	Namespaces = "namespaces"

	// the apps group
	Deployments = "deployments"
	DaemonSets  = "daemonsets"

	// the batch group
	Jobs = "jobs"

	// Supported extensions
	NodeFeature     = "nodeFeature"
	NodeFeatureRule = "nodeFeatureRule"
)

type Diagnostic struct {
	*Config
	collectors []Collector
}

type Option func(*Diagnostic)

func WithNamespace(namespace string) func(*Diagnostic) {
	return func(d *Diagnostic) {
		d.Config.namespace = namespace
	}
}

func WithArtifactDir(artifactDir string) func(*Diagnostic) {
	return func(d *Diagnostic) {
		d.Config.artifactDir = artifactDir
	}
}

func WithKubernetesClient(clientset kubernetes.Interface) func(*Diagnostic) {
	return func(d *Diagnostic) {
		d.Clientset = clientset
	}
}

func WithNFDClient(nfdClient *nfdclient.Clientset) func(*Diagnostic) {
	return func(d *Diagnostic) {
		d.NfdClient = nfdClient
	}
}

func WithObjects(objects ...string) func(*Diagnostic) {
	return func(d *Diagnostic) {
		seen := make(map[string]bool)
		for _, obj := range objects {
			if seen[obj] {
				continue
			}
			seen[obj] = true
			switch obj {
			case Nodes:
				d.collectors = append(d.collectors, nodes{Config: d.Config})
			case Namespaces:
				d.collectors = append(d.collectors, namespaces{Config: d.Config})
			case Pods:
				d.collectors = append(d.collectors, pods{Config: d.Config})
			case Deployments:
				d.collectors = append(d.collectors, deployments{Config: d.Config})
			case DaemonSets:
				d.collectors = append(d.collectors, daemonsets{Config: d.Config})
			case Jobs:
				d.collectors = append(d.collectors, jobs{Config: d.Config})
			case NodeFeature:
				d.collectors = append(d.collectors, nodeFeatures{Config: d.Config})
			case NodeFeatureRule:
				d.collectors = append(d.collectors, nodeFeatureRules{Config: d.Config})
			default:
				klog.Warningf("Unsupported object %s", obj)
				continue
			}
		}
	}
}

func New(opts ...Option) (*Diagnostic, error) {
	c := &Config{}
	dc := &Diagnostic{
		Config: c,
	}

	// use the variadic function to set the options
	for _, opt := range opts {
		opt(dc)
	}

	return dc, nil
}
