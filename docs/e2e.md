# E2E Testing

This document describes how to run the IPAM end-to-end tests.

## Prerequisites

- Go (see `hack/e2e/ensure_go.sh` for minimum version)
- kubectl (see `hack/e2e/ensure_kubectl.sh` for minimum version)
- Docker (for Kind cluster provisioning)

## Running E2E Tests

### All tests

```sh
make docker-build-e2e
make test-e2e

# or from root
./hack/e2e/ci-script.sh
```

### Run only basic tests

```sh
make docker-build-e2e
make test-e2e GINKGO_FOCUS_LABELS=basic
```

### Run only feature tests

```sh
make docker-build-e2e
make test-e2e GINKGO_FOCUS_LABELS=features
```

### Skip specific test labels

```sh
make docker-build-e2e
make test-e2e GINKGO_SKIP_LABELS=features
```

## Test Labels

Tests are organized using Ginkgo labels:

| Label | Description |
|-------|-------------|
| `basic` | Core IPAM operations: IPPool CRUD, IP allocation via Metal3 and CAPI claims, garbage collection |
| `features` | Advanced functionality: preallocations, pool exhaustion/recovery, multi-pool, status tracking, mixed claims |

## Configuration

### Using an existing cluster

To run against an already-running cluster (useful for debugging):

```sh
make docker-build-e2e
make test-e2e USE_EXISTING_CLUSTER=true
```

### Skipping cleanup

To keep resources around after tests finish (for post-mortem debugging):

```sh
make docker-build-e2e
make test-e2e SKIP_RESOURCE_CLEANUP=true
```

## CI Script

The CI entry point is `hack/e2e/ci-script.sh`. It:

1. Ensures Go and kubectl meet minimum versions
1. Builds image from local repository
1. Runs `make test-e2e`
1. Collects artifacts into a tarball

Label filtering is controlled via the `GINKGO_FOCUS_LABELS` environment variable:

```sh
# Run basic tests only in CI
GINKGO_FOCUS_LABELS=basic ./hack/e2e/ci-script.sh

# Run feature tests only in CI
GINKGO_FOCUS_LABELS=features ./hack/e2e/ci-script.sh

# Run all tests (default)
./hack/e2e/ci-script.sh
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
