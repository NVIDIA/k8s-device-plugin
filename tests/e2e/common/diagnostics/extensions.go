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
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type nodeFeatures struct {
	*Config
}

type nodeFeatureRules struct {
	*Config
}

func (c nodeFeatures) Collect(ctx context.Context) error {
	nfs, err := c.NfdClient.NfdV1alpha1().NodeFeatures(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error collecting %T: %w", c, err)
	}

	if err := c.outputTo("nodefeatures.yaml", nfs); err != nil {
		return err
	}

	return nil
}

func (c nodeFeatureRules) Collect(ctx context.Context) error {
	nfrs, err := c.NfdClient.NfdV1alpha1().NodeFeatureRules().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error collecting %T: %w", c, err)
	}

	if err := c.outputTo("nodefeaturerules.yaml", nfrs); err != nil {
		return err
	}

	return nil
}
