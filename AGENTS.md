# Metal3 IP Address Manager (IPAM)

Kubernetes controller that allocates IP addresses from `IPPool` resources
to Metal3 `IPClaim` and Cluster API `IPAddressClaim` objects, for use by
Cluster API Provider Metal3.
Project overview: [README.md](README.md). Contribution rules:
[CONTRIBUTING.md](CONTRIBUTING.md) and the
[Metal3 community guide](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md).

This file holds only what you cannot cheaply infer from the code. Where it
disagrees with the code or the Makefile, trust those and report the
mismatch.

## AI-assisted contributions

Metal3 follows the CNCF and
[Linux Foundation generative AI policy](https://www.linuxfoundation.org/legal/generative-ai).

- AI is a tool. The human who opens a PR is its author and is responsible
  for every line of it, including licensing and third-party content.
- Review is a conversation between humans. A PR where an agent or bot
  answers reviewers will be closed. Agents may draft changes; the
  contributor reads them, understands them, and replies personally.

- Keep changes surgical. The smallest complete change that solves the
  stated problem wins. Every added file, function, flag, dependency or
  test helper must trace to that problem; anything that does not is
  bloat and gets removed. Prefer reusing existing code over new code.
  Report unrelated findings instead of fixing them in the same PR.

## Repository map

Four Go modules (root, `api/`, `test/`, `hack/tools/`), wired together
with `replace` directives. Root `go test ./...` does not cover the other
three. Tidy them with `make modules`, not `go mod tidy`; `hack/gomod.sh`
fails on any diff.

- Allocation logic: `ipam/ippool_manager.go`. It serves both claim APIs:
  `ipam.metal3.io` `IPClaim`/`IPAddress` and `ipam.cluster.x-k8s.io`
  `IPAddressClaim`/`IPAddress`.
- Controllers: `controllers/ippool_controller.go`, registered once per
  claim API in `main.go`.
- Webhooks: `internal/webhooks/v1alpha1/`.

## Testing

- Targets: `make help` and [docs/testing.md](docs/testing.md). E2E:
  [docs/e2e.md](docs/e2e.md). CI jobs:
  [Prow](https://github.com/metal3-io/project-infra/blob/main/prow/config/jobs/metal3-io/ip-address-manager.yaml)
  and `.github/workflows/`.
- `controllers/` and `ipam/` tests use envtest. `make unit` sets it up;
  for plain `go test`, export the `KUBEBUILDER_ASSETS` line printed by
  `make setup-envtest` first.
- Conventions are whatever the existing code, tests, and lint configs do.
  Read neighboring files and `_test.go` files before writing new ones.
- Before finishing, add or update tests for every behavior change, then
  run `make test` and the `hack/` checks for the file types you touched.
  All must pass.
- Never hand-edit generated output (`zz_generated.*.go` including
  `ipam/mocks/`, CRDs, RBAC and webhook manifests under `config/`).
  Change the source and run `make generate`. The `generate` CI job fails
  on any diff.
- Do not touch `releasenotes/` outside a release PR. `make release-notes`
  generates them from PR titles; only release PRs curate them by hand
  ([docs/releasing.md](docs/releasing.md)).
- PR titles start with `:warning:` (breaking), `:sparkles:` (feature),
  `:bug:` (fix), `:book:` (docs) or `:seedling:` (other). The release
  notes tool sorts PRs by this prefix.
- Do not run `hack/e2e/ci-script.sh` or `hack/e2e/ensure_*.sh` unless the
  user asks: they install Go into `/usr/local` and kubectl into
  `/usr/local/bin` with `sudo`, and raise host inotify sysctls. CI runs E2E
  on every PR that changes code.

## Invariants

These apply to writing and to reviewing changes.

- **API version.** `v1alpha1` is the only version, the storage version and
  the conversion hub (`api/v1alpha1/conversion.go`). Validation lives in
  the webhooks.
- **API compatibility.** Never remove or rename a served field, and never
  tighten validation on existing fields, without a deprecation path.
  Mark such PRs as breaking.
- **Both claim APIs.** A change to allocation or release behavior usually
  needs both the `IPClaim` and the `IPAddressClaim` code paths, with tests
  for each.
- **Cluster API provider behavior.** Reconcilers must keep honoring a
  paused `Cluster` and the `cluster.x-k8s.io/paused` annotation
  (`IsPaused` in `controllers/ippool_controller.go`); `clusterctl move`
  depends on it. Created `IPAddress` objects keep owner references to
  their claim and pool; `test/e2e/pivoting_test.go` and
  `test/e2e/owner_ref_enforcement_test.go` cover both.
- **RBAC.** Grant permissions with `+kubebuilder:rbac` markers on the
  controller that needs them, then `make generate`.
- **Security.** Never weaken security to make something work: no
  disabled TLS verification, broadened RBAC, secrets in code, logs or
  fixtures, or skipped validation. Treat claim and pool specs as
  untrusted input. Call out any change to authentication, TLS, RBAC,
  Secrets or webhooks in the PR description.
- **Tenancy and isolation.** One IPAM serves every namespace, and pools
  and claims are namespaced. Review these areas with extra care:
   - Cross-namespace access. A claim must only draw from a pool in its own
     namespace. `IPClaim` `spec.pool.namespace` must be empty or equal to
     the claim's namespace; both the webhook
     (`internal/webhooks/v1alpha1/ipclaim_webhook.go`) and the
     claim-to-pool mapping (`controllers/ippool_controller.go`) enforce
     this. `IPAddressClaim` pools resolve in the claim's namespace.
   - Role isolation. The manager role is generated from markers; keep it
     to the verbs and resources the controller uses.
   - Multi-tenancy. Status, events, logs and metrics must not leak one
     tenant's pools, addresses or claims to another.
- **Pinning.** Pin new external references: GitHub Actions by commit SHA,
  container images by digest, downloaded binaries by checksum.

## External references

- [Metal3 Book, IPAM user guide](https://book.metal3.io/ipam/introduction)
- [IPAM design proposals](https://github.com/metal3-io/metal3-docs/tree/main/design/ip-address-manager)
- [Metal3 API design principles](https://github.com/metal3-io/metal3-docs/blob/main/design/api-design-principles.md)
