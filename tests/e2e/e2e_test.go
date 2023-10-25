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
	"math/rand"
	"os"
	"testing"
	"time"

	"k8s.io/kubernetes/test/e2e"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
)

var (
	NVIDIA_DRIVER_ENABLED = flag.Bool("driver-enabled", false, "NVIDIA driver is installed on test infra")
	HelmChart             = flag.String("helm-chart", "", "Helm chart to use")
	ImageRepo             = flag.String("image.repo", "", "Image repository to fetch image from")
	ImageTag              = flag.String("image.tag", "", "Image tag to use")
	ImagePullPolicy       = flag.String("image.pull-policy", "IfNotPresent", "Image pull policy")
)

// handleFlags sets up all flags and parses the command line.
func handleFlags() {
	config.CopyFlags(config.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	flag.Parse()
}

func TestMain(m *testing.M) {
	// Register test flags, then parse flags.
	handleFlags()

	framework.AfterReadingAllFlags(&framework.TestContext)

	// check if flags are set and if not cancel the test run
	if *ImageRepo == "" || *ImageTag == "" || *HelmChart == "" {
		framework.Failf("Required flags not set. Please set -gfd.repo, -gfd.tag and -helm-chart")
	}

	// TODO: Deprecating repo-root over time... instead just use gobindata_util.go , see #23987.
	// Right now it is still needed, for example by
	// test/e2e/framework/ingress/ingress_utils.go
	// for providing the optional secret.yaml file and by
	// test/e2e/framework/util.go for cluster/log-dump.
	if framework.TestContext.RepoRoot != "" {
		testfiles.AddFileSource(testfiles.RootFileSource{Root: framework.TestContext.RepoRoot})
	}

	rand.New(rand.NewSource(time.Now().UnixNano()))
	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	e2e.RunE2ETests(t)
}
