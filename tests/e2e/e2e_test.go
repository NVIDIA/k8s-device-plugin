/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

package e2e

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/NVIDIA/k8s-device-plugin/tests/e2e/framework"
)

var (
	NVIDIA_DRIVER_ENABLED = flag.Bool("driver-enabled", false, "NVIDIA driver is installed on test infra")
	HelmChart             = flag.String("helm-chart", "", "Helm chart to use")
	LogArtifactDir        = flag.String("log-artifacts", "", "Directory to store logs")
	ImageRepo             = flag.String("image.repo", "", "Image repository to fetch image from")
	ImageTag              = flag.String("image.tag", "", "Image tag to use")
	ImagePullPolicy       = flag.String("image.pull-policy", "IfNotPresent", "Image pull policy")
	CollectLogsFrom       = flag.String("collect-logs-from", "default", "Comma separated list of objects to collect logs from after test")
)

func TestMain(m *testing.M) {
	// Register test flags, then parse flags.
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()

	// check if flags are set and if not cancel the test run
	if *ImageRepo == "" || *ImageTag == "" || *HelmChart == "" {
		log.Fatal("Required flags not set. Please set -image.repo, -image.tag and -helm-chart")
	}

	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	// Run tests through the Ginkgo runner with output to console + JUnit for Jenkins
	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
	// Randomize specs as well as suites
	suiteConfig.RandomizeAllSpecs = true

	ginkgo.RunSpecs(t, "nvidia k8s-device-plugin e2e suite", suiteConfig, reporterConfig)
}
