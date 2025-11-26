# Metal3 IP Address Manager (IPAM) - AI Coding Agent Instructions

This file provides comprehensive instructions for AI coding agents working on
the Metal3 IP Address Manager project. It covers architecture, conventions,
tooling, CI/CD, and behavioral guidelines.

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture](#architecture)
- [Development Workflows](#development-workflows)
- [Makefile Reference](#makefile-reference)
- [Hack Scripts Reference](#hack-scripts-reference)
- [CI/CD and Prow Integration](#cicd-and-prow-integration)
- [Code Patterns and Conventions](#code-patterns-and-conventions)
- [Testing Guidelines](#testing-guidelines)
- [Integration Points](#integration-points)
- [Common Pitfalls](#common-pitfalls)
- [AI Agent Behavioral Guidelines](#ai-agent-behavioral-guidelines)

---

## Project Overview

Metal3 IPAM is a Kubernetes controller providing static IP address allocation
management for CAPM3 (Cluster API Provider Metal3). It implements a simple yet
flexible IPAM solution enabling declarative IP address assignment for bare
metal machines during cluster provisioning.

**Key URLs:**

- Repository: <https://github.com/metal3-io/ip-address-manager>
- Container Image: `quay.io/metal3-io/ip-address-manager`
- Documentation: <https://book.metal3.io/ipam/introduction>

### Project Goals

Metal3 IPAM aims to be a fully compliant **Cluster API IPAM Provider**. This
means:

1. **CAPI IPAM Contract Compliance** - Implements the CAPI IPAM provider
   contract, allowing any CAPI-based infrastructure provider to use Metal3
   IPAM for IP address management (not just CAPM3).

2. **Dual API Support** - Supports both:
   - Native Metal3 `IPClaim`/`IPAddress` resources (v1alpha1)
   - CAPI standard `IPAddressClaim`/`IPAddress` resources (ipam.cluster.x-k8s.io)

3. **Provider Agnostic** - While developed for Metal3/CAPM3, the IPAM
   functionality is designed to work with any infrastructure that needs
   static IP allocation in Kubernetes environments.

4. **Production Ready** - Moving towards API stability (v1beta1) with proper
   validation, webhooks, and operational maturity.

---

## Architecture

### Core Components

1. **Custom Resources (CRDs)** - Located in `api/v1alpha1/`:
   - `IPPool` - Defines ranges/subnets of IP addresses available for allocation
   - `IPClaim` - Represents a request for an IP address from a pool
   - `IPAddress` - Represents an allocated IP address (created by controller)

2. **Controllers** - Located in `controllers/`:
   - `IPPoolReconciler` - Manages IPPool lifecycle and validations

3. **IPAM Logic** - Located in `ipam/`:
   - Core allocation algorithms
   - IP range management and tracking
   - Prefix calculations and subnet handling

4. **Webhooks** - Located in `internal/webhooks/v1alpha1/`:
   - Validation webhooks for IPPool (validate ranges, subnets)
   - Immutability checks (pool spec changes restricted)
   - Defaulting logic for optional fields

### Resource Relationships

```text
IPPool (defines available IPs)
   ↓
IPClaim (requests an IP) → IPAddress (allocated IP)
   ↑                             ↓
CAPM3 Metal3Data          Consumed by Metal3DataTemplate
```

### Directory Structure

```text
ip-address-manager/
├── api/v1alpha1/          # CRD type definitions (separate Go module)
├── config/                # Kustomize manifests
│   ├── crd/bases/        # Generated CRD YAMLs
│   ├── default/          # Default deployment
│   ├── rbac/             # RBAC manifests
│   └── webhook/          # Webhook manifests
├── controllers/           # Controller implementations
├── docs/                  # Documentation
├── examples/              # Example resources
├── hack/                  # Build and CI scripts
│   ├── boilerplate/      # License headers
│   └── tools/            # Tool dependencies
├── internal/webhooks/     # Webhook implementations
├── ipam/                  # Core IPAM logic
│   └── mocks/            # Generated mocks
├── releasenotes/          # Release notes
└── test/                  # Integration tests
```

---

## Development Workflows

### Quick Start Commands

```bash
# Full verification (generate + lint + test)
make test

# Generate code after API changes
make generate

# Run unit tests only
make unit

# Run linters only
make lint

# Build manager binary
make manager

# Verify go modules are tidy
make verify-modules
```

### Local Development with Tilt

This repository uses Tilt for local development. See `tilt-provider.json` for
configuration. Tilt provides live reloading when developing with CAPM3.

### Docker Build

```bash
# Build container image
make docker-build IMG=quay.io/metal3-io/ip-address-manager TAG=dev

# Build with debug symbols
make docker-build-debug
```

---

## Makefile Reference

### Testing Targets

| Target | Description |
|--------|-------------|
| `make unit` | Run unit tests for controllers, ipam, api, webhooks |
| `make unit-cover` | Run unit tests with coverage report |
| `make test` | Run generate + lint + unit tests |

### Build Targets

| Target | Description |
|--------|-------------|
| `make manager` | Build manager binary to `bin/manager` |
| `make build` | Build all modules (manager + api) |
| `make build-api` | Build api module only |
| `make binaries` | Alias for manager |

### Code Generation Targets

| Target | Description |
|--------|-------------|
| `make generate` | Generate Go code and manifests |
| `make generate-go` | Generate Go code (deepcopy, mocks) |
| `make generate-manifests` | Generate CRDs, RBAC, webhooks |
| `make generate-examples` | Generate example configurations |

### Linting Targets

| Target | Description |
|--------|-------------|
| `make lint` | Run golangci-lint (fast mode) |
| `make lint-fix` | Run linter with auto-fix |
| `make lint-full` | Run linter with all checks (slow) |

### Verification Targets

| Target | Description |
|--------|-------------|
| `make verify` | Run all verification checks |
| `make verify-boilerplate` | Check license headers |
| `make verify-modules` | Verify go.mod/go.sum are tidy |

### Module Management

| Target | Description |
|--------|-------------|
| `make modules` | Run go mod tidy on all modules |

### Release Targets

| Target | Description |
|--------|-------------|
| `make release-manifests` | Build release manifests to `out/` |
| `make release-notes` | Generate release notes (requires RELEASE_TAG) |
| `make release` | Full release process (requires RELEASE_TAG) |

### Cleanup Targets

| Target | Description |
|--------|-------------|
| `make clean` | Remove all generated files |
| `make clean-bin` | Remove binaries |
| `make clean-release` | Remove release directory |
| `make clean-examples` | Remove generated examples |

### Utility Targets

| Target | Description |
|--------|-------------|
| `make go-version` | Print Go version used for builds |
| `make help` | Display all available targets |

---

## Hack Scripts Reference

Scripts in `hack/` are primarily for CI (Prow jobs call them directly).
**For local development, prefer Make targets** which handle setup automatically.

### Scripts Without Make Target Equivalents

| Script | Purpose |
|--------|---------|
| `verify-release.sh` | Comprehensive release verification |

### CI-to-Make Target Mapping

| CI Job | Make Target | Script (for reference) |
|--------|-------------|------------------------|
| `unit` | `make unit` | `unit.sh` |
| `generate` | `make generate` | `codegen.sh` |
| `gomod` | `make modules` | `gomod.sh` |
| `shellcheck` | - | `shellcheck.sh` |
| `markdownlint` | - | `markdownlint.sh` |
| `manifestlint` | - | `manifestlint.sh` |

Scripts without Make targets (`shellcheck.sh`, `markdownlint.sh`, `manifestlint.sh`)
auto-spawn containers and can be run directly: `./hack/shellcheck.sh`

---

## CI/CD and Prow Integration

### Prow Jobs (metal3-io/project-infra)

CI is managed via Prow jobs defined in
[project-infra](https://github.com/metal3-io/project-infra/blob/main/prow/config/jobs/metal3-io/ip-address-manager.yaml).

**Presubmit Jobs (run on PRs):**

| Job | Trigger | What it does |
|-----|---------|--------------|
| `gomod` | Code changes | Verifies go.mod is tidy |
| `unit` | Code changes | Runs unit tests |
| `generate` | Code changes | Verifies generated code |
| `lint` | (via `make test`) | Go linting |
| `shellcheck` | `.sh` or Makefile changes | Shell script linting |
| `markdownlint` | `.md` changes | Markdown linting |
| `manifestlint` | Code changes | Kubernetes manifest validation |
| `test` | Makefile/hack changes | Verifies Makefile/scripts work |

**E2E Tests (Jenkins):**

- `metal3-centos-e2e-integration-test-main` (mandatory, manually triggered)
- `metal3-ubuntu-e2e-integration-test-main` (mandatory, manually triggered)
- Various optional feature tests (pivoting, remediation, etc.)

### Skip Patterns

Jobs skip for:

- OWNERS/OWNERS_ALIASES changes only
- Markdown-only changes (for code jobs)

---

## Code Patterns and Conventions

### Go Code Style

Go code is formatted with `gofmt`. The linter (golangci-lint) enforces style
rules defined in `.golangci.yaml`. Key conventions:

- Import aliasing is enforced for common packages (see `.golangci.yaml`)
- License headers are required (see `hack/boilerplate/`)
- Run `make lint` to check, `make lint-fix` to auto-fix

### YAML Conventions

Kubernetes manifests use 2-space indentation (Kubernetes standard).
Yamllint will be added for enforcement.

### Shell Script Conventions

**Required settings at script start:**

```bash
#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
```

Or for POSIX sh:

```sh
#!/bin/sh
set -eux
```

**Container execution pattern:**

```bash
IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
WORKDIR="${WORKDIR:-/workdir}"

if [ "${IS_CONTAINER}" != "false" ]; then
    # Run the actual logic
else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:${WORKDIR}:ro,z" \
        --entrypoint sh \
        --workdir "${WORKDIR}" \
        <image> \
        "${WORKDIR}"/hack/<script>.sh "$@"
fi
```

### Markdown Conventions

Configuration in `.markdownlint-cli2.yaml`:

- Unordered list indent: 3 spaces
- No auto-fix (lint only)

### API Modifications Workflow

1. Edit types in `api/v1alpha1/*_types.go`
2. Run `make generate` to regenerate:
   - DeepCopy functions
   - CRDs in `config/crd/bases/`
   - RBAC in `config/rbac/`
3. Update webhooks if validation changes
4. Run `make test` to verify

### Controller Patterns

**Reconciliation flow:**

```go
func (r *IPPoolReconciler) Reconcile(ctx context.Context,
    req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the resource
    // 2. Handle deletion (finalizers)
    // 3. Process claims
    // 4. Update status
    // 5. Return with requeue if needed
}
```

**Owner references for garbage collection:**

- IPAddress has ownerReference to IPClaim
- Deletion cascades automatically

---

## Testing Guidelines

### Unit Test Framework

Tests use Ginkgo (BDD) + Gomega (matchers):

```go
var _ = Describe("IPPool Controller", func() {
    Context("when processing an IPClaim", func() {
        It("should allocate an IP address", func() {
            // Test implementation
            Expect(result).To(Equal(expected))
        })
    })
})
```

### Test Setup

`controllers/suite_test.go` bootstraps envtest for controller tests:

- Starts kube-apiserver and etcd
- Loads CRDs from `config/crd/bases/`
- Registers schemes

### Running Tests

```bash
# All unit tests (recommended)
make unit

# With coverage
make unit-cover
```

Note: Use `make unit` rather than `go test` directly, as envtest requires
proper setup via the Makefile.

### Test Environment Tools

Downloaded via `hack/fetch_ext_bins.sh`:

- `kube-apiserver`
- `etcd`
- `kubectl`

Location: `/tmp/kubebuilder/bin/`

---

## Integration Points

### With CAPM3

CAPM3's Metal3DataTemplate references IPPool:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3DataTemplate
spec:
  networkData:
    networks:
      ipv4:
        - id: provisioning
          link: eth0
          ipAddressFromIPPool: pool1
```

### With Cluster API

- Supports CAPI v1beta1/v1beta2 IPAddressClaim resources
- Compatible with CAPI's InfraCluster IPAM contract

### E2E Testing with CAPM3

**Important:** IPAM does not have standalone e2e tests. All e2e testing is
performed through the CAPM3 repository, which serves as the central e2e
testing hub for the entire Metal3 stack (BMO, CAPM3, IPAM).

**How e2e works for IPAM:**

1. CAPM3 e2e tests provision real bare metal clusters using Metal3
2. Metal3DataTemplate resources reference IPPool for network configuration
3. IPAM allocates IPs during cluster provisioning
4. Tests verify IP allocation in Metal3Data and BareMetalHost network-data

**IPAM's role in e2e:**

- Provides IP addresses for provisioning, BMC, and external networks
- IPPool resources are created as part of test cluster setup
- IPClaims are automatically created by CAPM3 controllers
- Allocation failures cause cluster provisioning to fail

**When cutting new release branches:**

When creating a new IPAM release branch (e.g., `release-1.11`), corresponding
updates are needed in CAPM3:

1. CAPM3's e2e tests reference IPAM container images
2. New IPAM branches need matching Prow job configurations in project-infra
3. CAPM3's `metadata.yaml` and clusterctl configuration may need updates
4. E2E test manifests in CAPM3 may reference specific IPAM versions

**Running e2e tests:**

```bash
# E2E tests are triggered via Prow jobs on CAPM3 PRs
# They can also be triggered on IPAM PRs using /test comments:
/test metal3-centos-e2e-integration-test-main
/test metal3-ubuntu-e2e-integration-test-main
```

See [project-infra Prow jobs](https://github.com/metal3-io/project-infra/blob/main/prow/config/jobs/metal3-io/ip-address-manager.yaml)
for available e2e job triggers.

---

## Common Pitfalls

1. **Pool Exhaustion** - Claim stays pending when no IPs available
2. **Pre-allocation Conflicts** - IP outside pool ranges
3. **Namespace Mismatch** - Cross-namespace refs not supported
4. **Manual IPAddress Creation** - Always use IPClaim
5. **Pool Updates** - Avoid modifying pools in use
6. **Forgetting `make generate`** - After API changes

---

## AI Agent Behavioral Guidelines

### Critical Rules

1. **Use single commands** - Do not concatenate multiple commands with `&&` or
   `;` to avoid interactive permission prompts from the user. Run one command
   at a time.

2. **Be strategic with output filtering** - Use `head`, `tail`, or `grep` when
   output is clearly excessive (e.g., large logs), but prefer full output for
   smaller results to avoid missing context.

3. **Challenge assumptions** - Do not take user statements as granted. If you
   have evidence against an assumption, present it respectfully with
   references.

4. **Search for latest versions** - When suggesting dependencies, libraries,
   or tools, always verify and use the latest stable versions.

5. **Security first** - Take security very seriously. Review code for:
   - Hardcoded credentials
   - Insecure defaults
   - Missing input validation
   - Privilege escalation risks

6. **Pin dependencies by SHA** - All external dependencies must be SHA pinned
   when possible (container images, GitHub Actions, downloaded binaries).
   This prevents supply chain attacks and ensures reproducible builds.

7. **Provide references** - Back up suggestions with links to documentation,
   issues, PRs, or code examples from the repository.

8. **Follow existing conventions** - Match the style of existing code:
   - Shell scripts: use patterns from `hack/` scripts
   - Go code: follow golangci-lint rules in `.golangci.yaml`
   - Markdown: follow `.markdownlint-cli2.yaml` rules
   - License headers: use templates from `hack/boilerplate/`

### Before Making Changes

1. Run `make lint` to understand current linting rules
2. Run `make unit` to verify baseline test status
3. Check existing patterns in similar files
4. Verify Go version matches `Makefile` (currently 1.24)

### When Modifying Code

1. Make minimal, surgical changes
2. Run `make generate` after API changes
3. Run `make test` before submitting
4. Update documentation if behavior changes
5. Add tests for new functionality

### When Debugging CI Failures

1. Check Prow job definitions in project-infra
2. Run the same hack script locally with `IS_CONTAINER=TRUE`
3. Use the exact container images from CI
4. Check if failure is flaky vs consistent

### Commit Guidelines

- Sign commits with `-s` flag (DCO required)
- Use conventional commit prefixes:
   - ✨ `:sparkles:` - New feature
   - 🐛 `:bug:` - Bug fix
   - 📖 `:book:` - Documentation
   - 🌱 `:seedling:` - Other changes
   - ⚠️ `:warning:` - Breaking changes

---

## Git and Release Information

- **Branches**: `main` (development), `release-X.Y` (stable releases)
- **DCO Required**: All commits must be signed off (`git commit -s`)
- **PR Labels**: ⚠️ breaking, ✨ feature, 🐛 bug, 📖 docs, 🌱 other
- **Release Process**: See [docs/releasing.md](./docs/releasing.md)

---

## Additional Resources

### Related Projects

- [CAPM3](https://github.com/metal3-io/cluster-api-provider-metal3) - Cluster
  API Provider Metal3 (primary consumer, E2E test hub)
- [BMO](https://github.com/metal3-io/baremetal-operator) - Baremetal Operator
- [Cluster API](https://github.com/kubernetes-sigs/cluster-api) - Upstream CAPI
- [Metal3 Docs](https://book.metal3.io) - Project documentation

### Issue Tracking

- Issues: <https://github.com/metal3-io/ip-address-manager/issues>
- Good first issues: [good first issue label](https://github.com/metal3-io/ip-address-manager/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22)

---

## Quick Reference Card

```bash
# Most common commands
make test              # Full verification
make unit              # Unit tests only
make lint              # Linting only
make generate          # Regenerate code
make modules           # Tidy go modules
make manager           # Build binary
make docker-build      # Build container

# Verification
make verify            # All checks
make verify-boilerplate # License headers
make verify-modules    # go.mod tidy

# Local development
make kind-create       # Create test cluster
make deploy            # Deploy CRDs and controller
make deploy-examples   # Deploy test resources

# Hack scripts (containerized)
./hack/unit.sh         # Unit tests
./hack/codegen.sh      # Verify codegen
./hack/gomod.sh        # Verify modules
./hack/shellcheck.sh   # Shell linting
./hack/markdownlint.sh # Markdown linting
./hack/manifestlint.sh # K8s manifest linting

# Release verification
RELEASE_TAG=v1.x.y ./hack/verify-release.sh
```
