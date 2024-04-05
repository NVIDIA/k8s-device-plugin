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

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
)

func MustGather(artifactDir, component, namespace string) error {
	// Get the kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")

	// Create the ARTIFACT_DIR
	if err := os.MkdirAll(artifactDir, os.ModePerm); err != nil {
		return err
	}

	// Redirect stdout and stderr to logs
	logFile, err := os.Create(filepath.Join(artifactDir, "must-gather.log"))
	if err != nil {
		return err
	}
	defer logFile.Close()
	errLogFile, err := os.Create(filepath.Join(artifactDir, "must-gather.stderr.log"))
	if err != nil {
		return err
	}
	defer errLogFile.Close()

	// Create the Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error creating Kubernetes rest config: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}
	err = setKubernetesDefaults(config)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error Setting up Kubernetes rest config: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	clientset, err := createKubernetesClient(kubeconfig)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error creating Kubernetes client: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	// Get all Nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error getting nodes: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	nodesFile, err := os.Create(filepath.Join(artifactDir, "nodes.yaml"))
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error creating nodes.yaml: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}
	defer nodesFile.Close()

	data, err := yaml.Marshal(nodes)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error marshalling nodes: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	_, err = nodesFile.Write(data)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error writing nodes.yaml: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	// Get Namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error getting namespaces: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	namespacesFile, err := os.Create(filepath.Join(artifactDir, "namespaces.yaml"))
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error creating namespaces.yaml: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}
	defer namespacesFile.Close()

	data, err = yaml.Marshal(namespaces)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error marshalling namespaces: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	_, err = namespacesFile.Write(data)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error writing namespaces.yaml: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	// Get DaemonSets
	daemonSets, err := clientset.AppsV1().DaemonSets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error getting daemonSets: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	daemonSetsFile, err := os.Create(filepath.Join(artifactDir, "daemonsets.yaml"))
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error creating daemonsets.yaml: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}
	defer daemonSetsFile.Close()

	data, err = yaml.Marshal(daemonSets)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error marshalling daemonSets: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	_, err = daemonSetsFile.Write(data)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error writing daemonsets.yaml: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	// Get all pods in the target namespace
	podList, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error getting pods: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	pods, err := os.Create(filepath.Join(artifactDir, fmt.Sprintf("%s_pods.yaml", component)))
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error creating %s_pods.yaml: %v\n", component, err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}
	defer pods.Close()

	data, err = yaml.Marshal(podList)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error marshalling podList: %v\n", err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
		}
		return err
	}

	_, err = pods.Write(data)
	if err != nil {
		if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error writing %s_pods.yaml: %v\n", component, err)); lerr != nil {
			err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
			return err
		}
		return err
	}

	// Get logs per pod
	for _, pod := range podList.Items {
		componentLogs, err := os.Create(filepath.Join(artifactDir, fmt.Sprintf("%s_logs.log", pod.Name)))
		if err != nil {
			if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error creating %s_logs.log: %v\n", pod.Name, err)); lerr != nil {
				err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
			}
			return err
		}
		defer componentLogs.Close()

		podLogOpts := v1.PodLogOptions{}
		req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &podLogOpts)
		podLogs, err := req.Stream(context.Background())
		if err != nil {
			if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error getting pod logs: %v\n", err)); lerr != nil {
				err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
			}
			return err
		}
		defer podLogs.Close()

		buf := make([]byte, 4096)
		for {
			n, err := podLogs.Read(buf)
			if err != nil {
				break
			}
			_, err = componentLogs.Write(buf[:n])
			if err != nil {
				if _, lerr := errLogFile.WriteString(fmt.Sprintf("Error writing pod logs: %v\n", err)); lerr != nil {
					err = fmt.Errorf("%v+ error writing to stderr log file: %v", err, lerr)
				}
				return err
			}
		}
	}

	return nil
}

func createKubernetesClient(kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// setKubernetesDefaults sets default values on the provided client config for accessing the
// Kubernetes API or returns an error if any of the defaults are impossible or invalid.
func setKubernetesDefaults(config *rest.Config) error {
	// TODO remove this hack.  This is allowing the GetOptions to be serialized.
	config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}

	if config.APIPath == "" {
		config.APIPath = "/api"
	}
	if config.NegotiatedSerializer == nil {
		// This codec factory ensures the resources are not converted. Therefore, resources
		// will not be round-tripped through internal versions. Defaulting does not happen
		// on the client.
		config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	}
	return rest.SetKubernetesDefaults(config)
}
