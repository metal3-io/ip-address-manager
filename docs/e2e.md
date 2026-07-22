# E2E Testing

This document describes how to run the IPAM end-to-end tests.

## Prerequisites

- Go (see `hack/e2e/ensure_go.sh` for minimum version)
- kubectl (see `hack/e2e/ensure_kubectl.sh` for minimum version)
- Docker (for Kind cluster provisioning)

## Running E2E Tests

```sh
make docker-build-e2e
make test-e2e

# or from root
./hack/e2e/ci-script.sh
```

## CI Script

The CI entry point is `hack/e2e/ci-script.sh`. It:

1. Ensures required tools meet minimum versions
1. Builds image from local repository
1. Runs `make test-e2e`
1. Collects artifacts into a tarball

Label filtering is controlled via the `GINKGO_FOCUS_LABELS` environment variable.

### Labels

Use labels to run specific tests

```sh
GINKGO_FOCUS_LABELS=basic ./hack/e2e/ci-script.sh
```

Tests are organized using Ginkgo labels:

| Label | Description |
|-------|-------------|
| `ipam` | Base label for all IPAM e2e coverage |
| `basic` | Core IPAM operations: IPPool CRUD, IP allocation via Metal3 and CAPI claims, garbage collection |
| `features` | Advanced functionality: preallocations, pool exhaustion/recovery, multi-pool, status tracking, mixed claims |
| `pivot` | `clusterctl move` / pivot workflow validation for IPAM resources across management clusters |
| `clusterctl-upgrade` | `clusterctl upgrade` workflow validation from previous release to current provider version |

### Skip specific test labels

```sh
make docker-build-e2e
make test-e2e GINKGO_SKIP_LABELS=features
```

## Configuration

### Using an existing cluster

To run against an already-running cluster:

```sh
USE_EXISTING_CLUSTER=true ./hack/e2e/ci-script.sh
```

### Skipping cleanup

To keep resources around after tests finish (for post-mortem debugging):

```sh
SKIP_RESOURCE_CLEANUP=true ./hack/e2e/ci-script.sh
```

## Artifacts

Test artifacts are written to `test/e2e/_artifacts/`:

- JUnit XML report (`junit.e2e_suite.1.xml`)
- Controller logs
- Cluster resource dumps
- Kind cluster logs (when not using an existing cluster)

## Parallel Execution

Tests run with 2 parallel Ginkgo nodes by default. Each test generates its own
namespace from the spec name (e.g., `e2e-allocate-an-ipaddress-via-metal3-ipclaim-a1b2c3`)
to avoid resource conflicts.
