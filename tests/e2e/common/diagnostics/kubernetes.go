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
	"bufio"
	"context"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type nodes struct {
	*Config
}

type namespaces struct {
	*Config
}

type pods struct {
	*Config
}

type deployments struct {
	*Config
}

type daemonsets struct {
	*Config
}

type jobs struct {
	*Config
}

func (c nodes) Collect(ctx context.Context) error {
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error collecting %T: %w", c, err)
	}

	if err := c.outputTo("nodes.yaml", nodes); err != nil {
		return err
	}

	return nil
}

func (c namespaces) Collect(ctx context.Context) error {
	namespaces, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error collecting %T: %w", c, err)
	}

	if err := c.outputTo("namespaces.yaml", namespaces); err != nil {
		return err
	}

	return nil
}

func (c daemonsets) Collect(ctx context.Context) error {
	daemonsets, err := c.Clientset.AppsV1().DaemonSets(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error collecting %T: %w", c, err)
	}

	if err := c.outputTo("daemonsets.yaml", daemonsets); err != nil {
		return err
	}

	return nil
}

func (c deployments) Collect(ctx context.Context) error {
	deployments, err := c.Clientset.AppsV1().Deployments(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error collecting %T: %w", c, err)
	}

	if err := c.outputTo("deployments.yaml", deployments); err != nil {
		return err
	}

	return nil
}

func (c pods) Collect(ctx context.Context) error {
	pods, err := c.Config.Clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error collecting %T: %w", c, err)
	}

	if err := c.outputTo("pods.yaml", pods); err != nil {
		return err
	}

	var errs error
	for _, pod := range pods.Items {
		errs = errors.Join(err, podLogCollector{c.Config, pod.Name}.Collect(ctx))
	}

	return errs
}

func (c jobs) Collect(ctx context.Context) error {
	jobs, err := c.Clientset.BatchV1().Jobs(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error collecting %T: %w", c, err)
	}

	if err := c.outputTo("jobs.yaml", jobs); err != nil {
		return err
	}

	return nil
}

type podLogCollector struct {
	*Config
	name string
}

func (c podLogCollector) Collect(ctx context.Context) error {
	podLogFile, err := c.createFile(fmt.Sprintf("%s.log", c.name))
	if err != nil {
		return fmt.Errorf("error creating podLogFile: %w", err)
	}
	defer podLogFile.Close()

	req := c.Clientset.CoreV1().Pods(c.namespace).GetLogs(c.name, &v1.PodLogOptions{})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("error getting pod logs: %w", err)
	}

	buf := bufio.NewScanner(podLogs)
	for buf.Scan() {
		if _, err := podLogFile.Write(buf.Bytes()); err != nil {
			return fmt.Errorf("error writing pod logs: %w", err)
		}
		if _, err := podLogFile.Write([]byte("\n")); err != nil {
			return fmt.Errorf("error writing pod logs: %w", err)
		}
	}
	if err := buf.Err(); err != nil {
		return fmt.Errorf("error reading pod log: %w", err)
	}

	return nil
}
