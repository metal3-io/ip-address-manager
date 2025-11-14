# Metal3 IP Address Manager (IPAM) - AI Coding Assistant Instructions

## Project Overview

Metal3 IPAM is a Kubernetes controller that provides static IP address
allocation management for CAPM3 (Cluster API Provider Metal3). It implements a
simple yet flexible IPAM solution enabling declarative IP address assignment
for bare metal machines during cluster provisioning.

## Architecture

### Core Components

1. **Custom Resources (CRDs)** - Located in `api/v1alpha1/`:

   - `IPPool` - Defines ranges/subnets of IP addresses available for allocation
   - `IPClaim` - Represents a request for an IP address from a pool
   - `IPAddress` - Represents an allocated IP address (created by controller)

2. **Controllers** - Located in `controllers/`:
   - `IPPoolReconciler` - Manages IPPool lifecycle and validations
   - `IPClaimReconciler` - Processes IP allocation requests, creates IPAddress objects
   - `IPAddressReconciler` - Manages IPAddress lifecycle and cleanup

3. **IPAM Logic** - Located in `ipam/`:
   - Core allocation algorithms
   - IP range management and tracking
   - Prefix calculations and subnet handling

### Resource Relationships

```text
IPPool (defines available IPs)
   ↓
IPClaim (requests an IP) → IPAddress (allocated IP)

   ↑                             ↓
CAPM3 Metal3Data          Consumed by Metal3DataTemplate
```

- Metal3DataTemplate in CAPM3 creates IPClaim resources
- IPAM controller allocates IP from pool, creates IPAddress
- Metal3Data consumes IPAddress for rendering network configuration

## Key Concepts

### IP Pool Structure

```yaml
spec:
  pools:
    - start: 192.168.0.10    # Range-based allocation
      end: 192.168.0.30
      prefix: 25
      gateway: 192.168.0.1
    - subnet: 192.168.1.0/26  # Subnet-based allocation
  prefix: 24                   # Default prefix
  gateway: 192.168.1.1        # Default gateway
  preAllocations:              # Reserve specific IPs
    claim-name: 192.168.0.12
```

**Pool Types:**

- **Range-based**: Specify start/end IP addresses
- **Subnet-based**: Specify subnet, allocate from available addresses
- **Mixed**: Combine both types in single IPPool

### Pre-allocations

Reserve specific IPs for known claims:

- Useful for control plane endpoints or load balancer IPs

- Prevents conflicts with existing infrastructure
- Associates claim name with specific IP address

### Allocation Strategy

1. Check pre-allocations first
2. Allocate from pool ranges in order defined
3. Skip already allocated addresses
4. Generate unique IPAddress name: `{pool-name}-{ip-with-dashes}`

## Development Workflows

### Build and Test

```bash
# Generate manifests after API changes
make generate-manifests

# Run unit tests (controllers, ipam package, api, webhooks)
make unit

# Run tests with coverage
make unit-cover

# Run linters
make lint

# Build manager binary
make manager

# Build container image
make docker-build IMG=quay.io/metal3-io/ip-address-manager TAG=dev
```

### Local Development

```bash
# Deploy CRDs to cluster
make install

# Scale down deployed controller (if running in cluster)
kubectl scale -n metal3-ipam-system \
  deployment.v1.apps/metal3-ipam-controller-manager --replicas 0

# Run controller locally


make run

# Deploy example pools and claims
make deploy-examples

# Delete examples
make delete-examples
```

### Testing with CAPM3

IPAM is tested as part of CAPM3 e2e tests:

- CAPM3 e2e tests create clusters with IP allocation
- Metal3DataTemplate references IPPool resources
- Verify IP allocation in Metal3Data and BareMetalHost network-data

## Code Patterns and Conventions

### API Modifications

1. Edit types in `api/v1alpha1/*_types.go`

2. Run `make generate-manifests` to regenerate CRDs
3. CRDs generated in `config/crd/bases/`
4. API is v1alpha1 - maintain forward compatibility

### Controller Patterns

**IPClaim Reconciliation:**

```go
// Controller checks if IPAddress already exists for claim

// If not, allocates IP from pool and creates IPAddress

// Allocation logic:
// 1. Get referenced IPPool
// 2. Check preAllocations map
// 3. If not pre-allocated, find next available IP in pool ranges
// 4. Create IPAddress with owner reference to IPClaim
// 5. Update IPClaim status with allocated address
```

**IPAddress Ownership:**

- IPAddress has ownerReference to IPClaim
- When IPClaim deleted, IPAddress automatically garbage collected
- When IPAddress deleted manually, IPClaimReconciler recreates it

**Pool Validation:**

- Validate IP ranges (start < end)

- Validate subnet formats (CIDR notation)
- Check prefix values (0-32 for IPv4, 0-128 for IPv6)
- Ensure no overlapping ranges within pool

### Testing

**Unit Tests:**

- Test allocation algorithms in `ipam/` package

- Test controller logic with fake clients
- Test edge cases: pool exhaustion, invalid requests, concurrent claims
- Use `gomega` for assertions

**Integration Tests:**

- Located in `test/`

- Test actual resource creation/deletion
- Verify garbage collection behavior

### Webhooks

Located in `internal/webhooks/v1alpha1/`:

- Validation webhooks for IPPool (validate ranges, subnets)

- Immutability checks (pool spec changes restricted)
- Defaulting logic for optional fields

## Integration Points

### With CAPM3

CAPM3's Metal3DataTemplate references IPPool:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: Metal3DataTemplate
spec:
  networkData:
    links:
      ethernets:
        - type: phy
          id: eth0
          mtu: 1500
    networks:
      ipv4:
        - id: provisioning
          link: eth0
          ipAddressFromIPPool: pool1  # References IPPool
          routes:
            - network: 0.0.0.0/0
              gateway:
                fromIPPool: pool1     # Uses pool's gateway
```

When Metal3Data rendered:

- CAPM3 creates IPClaim for each `ipAddressFromIPPool` reference

- IPAM allocates IP and creates IPAddress
- CAPM3 reads IPAddress and renders into network-data for BareMetalHost

### With Cluster API

- IPAM can work with CAPI v1beta1 IPAddressClaim resources
- Controllers check claim type and handle both Metal3IPClaim and CAPI IPAddressClaim
- Provides compatibility with CAPI's InfraCluster IPAM contract

## Key Files Reference

- `main.go` - Entrypoint, controller registration
- `metadata.yaml` - Provider metadata for clusterctl
- `Makefile` - Primary build/test interface
- `config/default/` - Default deployment manifests
- `examples/` - Example IPPool, IPClaim resources
- `docs/api.md` - API documentation
- `docs/deployment_workflow.md` - Workflow documentation

## Common Workflows

### Creating an IP Pool

```yaml
apiVersion: ipam.metal3.io/v1alpha1
kind: IPPool
metadata:
  name: provisioning-pool
  namespace: metal3
spec:
  clusterName: test-cluster
  namePrefix: test-cluster-prov
  pools:
    - start: 192.168.10.100
      end: 192.168.10.200
      prefix: 24
      gateway: 192.168.10.1
  preAllocations:
    control-plane-0: 192.168.10.100
    control-plane-1: 192.168.10.101
```

### Manual IP Claim

```yaml
apiVersion: ipam.metal3.io/v1alpha1
kind: IPClaim
metadata:
  name: my-claim
  namespace: metal3
spec:
  pool:
    name: provisioning-pool
    namespace: metal3
```

After applying, check allocated address:

```bash
kubectl get ipaddress -n metal3
```

### Monitoring Allocations

```bash
# View all IP pools

kubectl get ippools -A

# View pool status (shows allocations)
kubectl get ippool provisioning-pool -o yaml

# View all allocated addresses
kubectl get ipaddresses -A

# View claims
kubectl get ipclaims -A
```

## Common Pitfalls

1. **Pool Exhaustion** - No available IPs in pool ranges. Claim stays
   pending. Check pool capacity.
2. **Pre-allocation Conflicts** - Pre-allocated IP falls outside pool ranges.
   Validation should catch this.
3. **Claim Name Collisions** - Two claims with same name in namespace attempt
   allocation. First wins.
4. **Namespace Mismatch** - IPClaim references IPPool in different namespace.
   Cross-namespace refs not supported.
5. **Timing Issue** - Rapid claim create/delete can occasionally leave
   orphaned IPAddress (known limitation, low priority).
6. **Manual IPAddress Creation** - Creating IPAddress manually without claim
   causes issues. Use IPClaim.
7. **Pool Updates** - Changing pool spec after allocations can invalidate
   existing allocations. Avoid modifying pools in use.

## Versioning and Compatibility

- API version: v1alpha1 (pre-stable, breaking changes possible)
- Follows CAPM3 release cadence (v1.1.X, v1.2.X, etc.)

- Compatible with CAPI v1beta1
- IP version agnostic - works with IPv4, IPv6, or dual-stack

## CI/PR Integration

- Tests run as part of CAPM3 e2e test suite
- No standalone e2e tests (tested via CAPM3 integration)
- Sign commits with `-s` flag (DCO required)
- Unit tests must pass for all packages

## Future Considerations

- Moving from v1alpha1 to v1beta1 (API stability)
- Support for dynamic IP pools (add/remove ranges)
- IP lease duration and expiration
- Metrics for pool utilization
- IPv6 dual-stack enhancements
