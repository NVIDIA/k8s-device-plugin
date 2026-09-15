# AGENTS.md

Guidance for AI coding agents working in this repository. Human contributors should also read
[CONTRIBUTING.md](CONTRIBUTING.md) and the
[documentation site](https://nvidia.github.io/k8s-device-plugin/), both of which take precedence
over this file.

## Project Summary

The NVIDIA device plugin for Kubernetes is a DaemonSet that advertises NVIDIA GPUs to the kubelet
through the
[Kubernetes device plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
framework, so that the kubelet can allocate those GPUs to containers. It also tracks GPU health.
As of v0.15.0 this repository additionally holds the implementation of GPU Feature Discovery (GFD),
which labels nodes with GPU properties.

The repository builds four binaries, each with its own entrypoint under `cmd/`:

- `nvidia-device-plugin` is the device plugin itself.
- `gpu-feature-discovery` is the GFD node labeler, which can be deployed alongside the plugin or on
  its own.
- `mps-control-daemon` is the control daemon for CUDA MPS-based GPU sharing.
- `config-manager` applies the per-node configuration selected by a node label.

The Go module is `github.com/NVIDIA/k8s-device-plugin` and the configuration API types live in
`api/config/v1`.

## Repository layout

- `api/config/v1/` holds the plugin configuration API types, covering sharing, MIG, IMEX, and flags.
- `cmd/` holds one directory per binary, as listed under Project Summary.
- `internal/` holds the supporting packages. The most important are `internal/rm` (resource
  managers for full GPUs and MIG devices), `internal/plugin` (the device plugin server),
  `internal/lm` (label management for GFD), `internal/cdi` (Container Device Interface spec
  generation), `internal/vgpu` (vGPU detection), and `internal/resource` (device discovery through
  NVML, sysfs, and CUDA).
- `deployments/helm/` holds the Helm chart for the plugin and GFD.
- `deployments/container/` holds the Dockerfile for the plugin image.
- `deployments/devel/` holds the pinned development toolchain used by `make`.
- `docs/` holds additional documentation, including `docs/cdi.md` and `docs/gpu-feature-discovery/`.
- `tests/` holds the end-to-end and Helm test suites and is a separate Go module.
- `testdata/` holds fixtures used by the end-to-end tests.
- `hack/` holds the development and release scripts for the changelog, third-party notices, and
  release preparation.
- `demo/` holds scripts that build and run a kind-based demo cluster.

## Build, test, and lint

All standard tasks go through the Makefile. Prefer make targets over invoking tools directly so
that CI and local runs stay consistent.

- `make build` compiles every package in the module, and `make cmds` builds each binary under
  `cmd/`.
- `make test` runs the unit tests over `cmd/...`, `internal/...`, and `api/...` with coverage.
- `make check` runs the full check suite, which at present contains only `make lint`.
- `make lint` runs `golangci-lint run ./...` using the configuration in `.golangci.yml`.
- `make fmt` applies `gofmt -s` to the codebase and writes the files in place.
- `make goimports` runs `goimports -local github.com/NVIDIA/k8s-device-plugin` and writes the files
  in place.
- `make generate` runs `go generate ./...`, which regenerates the `moq` mocks.
- `make check-modules` tidies, verifies, and vendors every module, then fails if `go.mod`, `go.sum`,
  or `vendor/` turn out to be stale. CI runs the same check.
- `make third-party-notices` regenerates `THIRD_PARTY_NOTICES.md` and `make check-third-party-notices`
  fails if that file is out of date. CI runs the same check.
- `make coverage` runs the unit tests and then prints a per-function coverage report that excludes
  mocks.
- `make test-e2e` runs a Ginkgo suite that requires a live Kubernetes cluster with real GPUs. Do
  not report it as passing when it was not run.
- `make test-helm` renders the Helm chart templates and asserts on the result. It needs no cluster.

Always run `make fmt`, `make check`, and `make test` before considering Go changes complete.

After changing a dependency, run `make check-modules` and `make check-third-party-notices`, then
commit the regenerated `vendor/` tree and `THIRD_PARTY_NOTICES.md` alongside the source change.
After changing an interface that carries a `//go:generate moq` directive, run `make generate` and
commit the regenerated mock.

## Coding Conventions

- Write idiomatic Go and follow the existing patterns in `internal/` for logging, error wrapping,
  and NVML access rather than introducing new libraries.
- Comments explain **why**, not **what**. Identifier names should carry the what.
- Keep changes scoped to the task. Avoid drive-by refactors, speculative abstractions, and
  unrelated formatting churn, and keep each pull request to one concern.
- Every `.go` file should start with the Apache-2.0 boilerplate header. Match the header in
  neighboring files exactly rather than inventing a variant.
- The `vendor/` directory is checked in. Run `go mod tidy` and `go mod vendor` after any dependency
  change and never hand-edit vendored code.
- The `*_mock.go` files are generated by `moq` from `//go:generate` directives, so regenerate them
  with `make generate` rather than editing them by hand.

## Testing Conventions

- Unit tests live in co-located `*_test.go` files and use `testify` through `require` and `assert`.
  Most of them group cases as subtests with `t.Run`. Run them with `make test`. Add tests that cover
  new behavior.
- The end-to-end tests under `tests/e2e` use Ginkgo and Gomega and live in a separate Go module.
  They need a real cluster with GPUs, so treat them as unavailable unless that environment is
  present. The Helm tests under `tests/helm` are plain Go tests and need no cluster.
- When fixing a bug, add a regression test that fails without the fix.

## Contribution process (see `CONTRIBUTING.md`)

- Any significant change needs an issue describing the problem or proposal before implementation
  begins. This covers architectural changes, new features, breaking changes to API or behavior, and
  non-trivial bug fixes. Check for a linked issue, or ask for one, rather than assuming a pull
  request on its own is sufficient.
- All commits must be signed off for DCO using `git commit -s`, which appends a
  `Signed-off-by: Name <email>` line. DCO is a required status check. Never fabricate a sign-off
  identity. Use the configured git user's identity.
- Do not open, push to, or comment on GitHub issues or pull requests without explicit user
  confirmation.
- Keep pull request titles short and imperative. Fill in the sections in
  [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md) and explain the motivation
  rather than restating the diff.

## Things to avoid

- Never commit credentials, API keys, tokens, passwords, kubeconfigs, or private keys.
- Never hand-edit a generated file. Regenerate it instead, as described under Coding Conventions.
- Never commit built binaries, `coverage.out`, or anything that [.gitignore](.gitignore) already
  excludes.
- Never modify [GOVERNANCE.md](GOVERNANCE.md) or [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) unless the
  task is explicitly about those files.
