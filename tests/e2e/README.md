<!--
SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
SPDX-License-Identifier: Apache-2.0

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# README – End‑to‑End (Ginkgo/Gomega) Test Suite for the NVIDIA K8s Device Plugin

---

## 1  Purpose
This repository contains a self‑contained Ginkgo v2 / Gomega end‑to‑end (E2E) test suite that

1. Creates an **isolated namespace** per run.
2. Deploys the **NVIDIA k8s‑device‑plugin Helm chart** under a random release name.
3. Executes a **CUDA “*n‑body*” benchmark job** to validate GPU scheduling.

On test failure the suite gathers logs and **ensures full cleanup** (namespace deletion, finalizer removal).
The suite targets CI pipelines and developers validating chart or driver changes before promotion.

---

## 2  Prerequisites

| Requirement          | Notes                                                                        |
|----------------------|-------------------------------------------------------------------------------|
| **Go ≥ 1.22**        | Needed for building helper binaries.                                          |
| **Kubernetes cluster** | Must be reachable via `kubectl`; worker nodes require NVIDIA GPUs.            |
| **Helm v3 CLI**      | Only required for manual debugging; the suite uses a programmatic client.     |
| **Linux/macOS host** | The Makefile assumes a POSIX‑compatible shell.                                |

---

## 3  Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `KUBECONFIG` | ✔ | — | Path to the target‑cluster kubeconfig. |
| `HELM_CHART` | ✔ | — | Helm chart reference (e.g. `oci://ghcr.io/nvidia/k8s-device-plugin`). |
| `E2E_IMAGE_REPO` | ✔ | — | Repository hosting the image under test. |
| `E2E_IMAGE_TAG` | ✔ | — | Image tag to test. |
| `E2E_IMAGE_PULL_POLICY` | ✔ | — | Image pull policy (`Always`, `IfNotPresent`, …). |
| `E2E_TIMEOUT_SECONDS` | ✖ | `1800` | Global timeout (s). |
| `LOG_ARTIFACTS_DIR` | ✖ | `./artifacts` | Directory for Helm & test logs. |
| `COLLECT_LOGS_FROM` | ✖ | (unset) | Comma‑separated node list or `all` for log collection. |
| `NVIDIA_DRIVER_ENABLED` | ✖ | `false` | Skip GPU job when driver is unavailable. |

> *Unset variables fall back to defaults via `getIntEnvVar` / `getBoolEnvVar`.*

---

## 4  Build helper binaries

```bash
make ginkgo
# → ./bin/ginkgo (latest v2 CLI)
```

---

## 5  Run the suite

### 5.1  Default invocation
```bash
make test-e2e
```
Generates the CLI (if missing), executes all specs under `./tests/e2e`, and writes a JSON report to `ginkgo.json`.

### 5.2  Focused run / extra flags
```bash
GINKGO_ARGS='--focus="[GPU Job]" --keep-going' make test-e2e
```
Any flag accepted by `ginkgo run` can be forwarded through `GINKGO_ARGS`.

---

## 6  Execution flow

| Phase | Key functions / objects | Description |
|-------|-------------------------|-------------|
| **Init** | `TestMain`, `getTestEnv` | Validates env vars, sets global timeout. |
| **Client setup** | `getK8sClients`, `getHelmClient` | Creates REST clients (core, CRD, NFD) and a Helm client that shares the same `rest.Config`. |
| **Namespace** | `CreateTestingNS` | Generates a unique namespace labelled `e2e-run=<uid>`. |
| **Chart deploy** | `helmClient.InstallRelease` | Installs the chart in the test namespace with a random release name. |
| **Workload** | `newGPUJob` | Launches `nvcr.io/nvidia/k8s/cuda-sample:nbody` requesting `nvidia.com/gpu=1`. |
| **Assertions** | Gomega matchers | Waits for `JobSucceeded == 1` and validates pod logs. |
| **Cleanup** | `cleanupNamespaceResources`, `AfterSuite` | Removes finalizers, deletes namespace, closes Helm log file. |

---

## 7  Artifacts & logs

```
${LOG_ARTIFACTS_DIR}/
└── helm/
    ├── helm_logs        # Release operations, one per test namespace
    └── ...

ginkgo.json              # Structured test outcome for CI parsing
```
If `COLLECT_LOGS_FROM` is set, additional node‑level or container logs are archived in the same directory.

---

## 8  Extending the suite

### 8.1  Creating additional spec files

1. Add a new `_test.go` file under `tests/e2e`.
2. Import the Ginkgo/Gomega DSL:
   ```go
   import (
       . "github.com/onsi/ginkgo/v2"
       . "github.com/onsi/gomega"
   )
   ```
3. Wrap your tests with `Describe`, `Context`, `When`, `It`, etc.
4. Scope all resources to `testNamespace` and always guard API calls with `Expect(err).NotTo(HaveOccurred())`.
5. Use helpers such as `wait.PollUntilContextTimeout` for custom waits and back‑off loops.

### 8.2  Adding additional *When* blocks to `device-plugin_test.go`
The suite already contains a high‑level file, `tests/e2e/device-plugin_test.go`, which drives most GPU‑focused checks.  To extend it:

1. **Open** `tests/e2e/device-plugin_test.go`.
2. **Locate** the outer `Describe("GPU Device Plugin", Ordered, func() { … })` wrapper.
3. **Add a sibling `When` container** under this `Describe` for each new behaviour you want to validate:
   ```go
   When("....", func() {
       It("should ......", func(ctx context.Context) {
            // 
            // 
            // ...
       })
   })
   ```
4. **Use `Ordered`** on the `When` block *only* if its order relative to other tests is significant (e.g. upgrade/downgrade flows). Otherwise omit it for independent execution.
5. **Share helpers**: you can reference `helmClient`, `clientSet`, `randomSuffix()`, `eventuallyNonControlPlaneNodes`, etc., directly because they are package‑level variables/functions exposed by `e2e`.
6. **Diagnostics on failure** are automatic – `AfterEach` will collect logs whenever `CurrentSpecReport().Failed()` is `true`.

> Keep each `When` block focused on one behaviour. If it spawns multiple `It` tests, make sure they are idempotent and leave no residual resources so that later blocks start from a clean state.

---

## 9  Troubleshooting  Troubleshooting

| Symptom | Possible fix |
|---------|--------------|
| **`ErrImagePull` for CUDA job** | Validate `E2E_IMAGE_REPO` / `E2E_IMAGE_TAG` and registry access. |
| Job stuck in **`Pending`** | Ensure nodes advertise `nvidia.com/gpu` and tolerations match taints. |
| Helm install failure | Render manifests locally via `helm template $HELM_CHART` to inspect errors. |

---

## 10  License
This test code is released under the same license as the NVIDIA k8s‑device‑plugin project (Apache‑2.0).

---

## 11  References
* [Ginkgo v2](https://github.com/onsi/ginkgo)
* [mittwald/go‑helm‑client](https://github.com/mittwald/go-helm-client)
* [Kubernetes‑sigs/Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)
* [Kubernetes blog – *End‑to‑End Testing for Everyone*](https://kubernetes.io/blog/2020/07/27/kubernetes-e2e-testing-for-everyone/)
