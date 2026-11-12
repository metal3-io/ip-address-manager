# API and Resource Definitions

This describes a setup where the IPAM component is deployed. It is agnostic of
the IP version. All examples are given with IPv4 but could be IPv6.

## IPPool

An IPPool is an object representing a set of IP addresses pools to be used for
IP address allocations.

Example pool:

```yaml
apiVersion: ipam.metal3.io/v1alpha1
kind: IPPool
metadata:
  name: pool1
  namespace: default
spec:
  clusterName: cluster1
  namePrefix: test1-prov
  pools:
  - start: 192.168.0.10
    end: 192.168.0.30
    prefix: 25
    gateway: 192.168.0.1
  - subnet: 192.168.1.1/26
  - subnet: 192.168.1.128/25
  prefix: 24
  gateway: 192.168.1.1
  preAllocations:
    claim1: 192.168.0.12
```

The *spec* field contains the following :

* **clusterName**: That is the name of the cluster to which this pool belongs
  it is used to verify whether the resource is paused.
* **namePrefix**: That is the prefix used to generate the IPAddress.
* **pools**: this is a list of IP address pools
* **prefix**: This is a default prefix for this IPPool
* **gateway**: This is a default gateway for this IPPool
* **preAllocations**: This is a default preallocated IP address for this IPPool.
Preallocations associate a claim's name to an IP address. It doesn't matter if
the claim type is (metal3)IPClaim or (capi)IPAddressClaim.

The *prefix* and *gateway* can be overridden per pool. The pool definition is
as follows :

* **start**: the IP range start address. Can be omitted if **subnet** is set.
* **end**: the IP range end address. Can be omitted.
* **subnet**: the subnet for the allocation. Can be omitted if **start** is set.
  It is used to verify that the allocated address belongs to this subnet.
* **prefix**: override of the default prefix for this pool
* **gateway**: override of the default gateway for this pool
* **DNSServers**: override of the default dns servers for this pool

## IPClaim

An IPClaim is an object representing a request for an IP address allocation.

Example IPClaim:

```yaml
apiVersion: ipam.metal3.io/v1alpha1
kind: IPClaim
metadata:
  name: test1-controlplane-template-0-provisioning-pool
  namespace: default
spec:
  pool:
    name: pool1
    namespace: default
```

The *spec* field contains the following :

* **pool**: a reference to the IPPool this request is for

## IPAddress

An IPAddress is an object representing an IP address allocation.

Example IPAddress:

```yaml
apiVersion: ipam.metal3.io/v1alpha1
kind: IPAddress
metadata:
  name: pool1-192-168-0-13
  namespace: default
spec:
  pool:
    name: pool1
    namespace: default
  claim:
    name: test1-controlplane-template-0-provisioning-pool
    namespace: default
  address: 192.168.0.13
  prefix: 24
  gateway: 192.168.0.1
```

The *spec* field contains the following :

* **pool**: a reference to the IPPool this address is for
* **claim**: a reference to the IPClaim this address is for
* **address**: the allocated IP address
* **prefix**: the prefix for this address
* **gateway**: the gateway for this address
* **DNSServers**: a list of dns servers

## Metal3 dev env examples

You can find CR examples in the
[Metal3-io dev env project](https://github.com/metal3-io/metal3-dev-env)

## Handling CAPI CRs

This IPAM can be deployed and used as an
[IPAM provider](https://cluster-api.sigs.k8s.io/developer/providers/contracts/ipam)for
[CAPI](https://github.com/kubernetes-sigs/cluster-api).

IPPool reconsiles (metal3)ipclaims into (metal3)ipaddresses
and (capi)ipaddressclaims into (capi)ipaddresses.

### IPAddressClaim

Check out more on [IPAddressClaim docs](https://cluster-api.sigs.k8s.io/reference/api/crd-api-reference#ipaddressclaim).

#### IPAddressClaim conditions

The controller reports the state of a (capi)IPAddressClaim in
`status.conditions`. The (metal3)IPClaim does not use these conditions.

The *Ready* condition is the one defined by the Cluster API IPAddressClaim
contract. The controller does not set it directly: it is computed from two
sub-conditions, *IPAddressAllocated* and *IPAddressCreated*, which show where a
claim is stuck. For example, a claim that cannot get an address because the pool
is full has:

```yaml
status:
  conditions:
  - type: Ready
    status: "False"
    reason: PoolExhausted
    message: '* IPAddressAllocated: Exhausted IP Pools'
  - type: IPAddressAllocated
    status: "False"
    reason: PoolExhausted
    message: Exhausted IP Pools
```

Fields such as *lastTransitionTime* are left out of the example.

##### IPAddressAllocated

This condition tells whether an address has been picked from the pool for the
claim. The *ipAddress* annotation on a claim requests a specific address, and
*preAllocations* is a field of the [IPPool](#ippool).

| Status | Reason | Set when |
|--------|--------|----------|
| True | Allocated | An address has been picked for the claim. |
| False | InvalidRequestedIP | The *ipAddress* annotation is not a valid IP address. |
| False | PreAllocationConflict | The *ipAddress* annotation differs from the address that *preAllocations* has for the claim. |
| False | RequestedIPNotAvailable | The address requested with the *ipAddress* annotation is in a pool but already in use. It is retried after 15 seconds. |
| False | RequestedIPOutOfBounds | The address requested with the *ipAddress* annotation is in none of the pools. It is retried after 15 seconds, and as soon as the IPPool or the claim changes. |
| False | PreAllocationOutOfBounds | The address that *preAllocations* has for the claim is not in any of the pools. |
| False | PoolExhausted | No address is free in the pools. It is retried after 15 seconds. |
| False | InvalidPrefix | The prefix of the pool, or the default prefix of the IPPool, is not between 0 and 128. |

The other failures are retried too, with the controller's backoff, so a claim
recovers once the pool or the annotation is fixed.

##### IPAddressCreated

This condition tells whether the IPAddress object for the claim exists and is
referenced in `status.addressRef`. It is only set once an address has been
allocated: when allocation fails, the condition is removed from the claim.

| Status | Reason | Set when |
|--------|--------|----------|
| True | Created | The IPAddress object exists and `status.addressRef` points to it. |
| Unknown | CreatePending | Creating the IPAddress object hit a transient error (the object already exists). The creation is retried. |
| False | CreateFailed | Creating the IPAddress object failed with any other error. It is retried with backoff. |

CreatePending is not reported as a failure: *Ready* stays Unknown, not False,
until the retry succeeds or fails with another error.

##### Ready

*Ready* is recomputed from the sub-conditions each time the controller
reconciles a claim, just before the claim status is patched. It is not
recomputed while the claim is being deleted.

| Sub-conditions | Ready status | Ready reason |
|----------------|--------------|--------------|
| All True | True | Ready |
| Any False with reason PoolExhausted | False | PoolExhausted |
| Any other False | False | AllocationFailed |
| None False, any Unknown | Unknown | ReadyUnknown |

*IPAddressCreated* is ignored when it is not set. *IPAddressAllocated* is
required: when it is not set, it counts as Unknown. This happens, for example,
when a claim is created while the IPAddress of an earlier claim with the same
name is still being deleted. The controller leaves the sub-conditions unchanged
and requeues the claim after 5 seconds.

PoolExhausted and AllocationFailed are the failure reasons that the Cluster API
IPAddressClaim types define. The third one, PoolNotReady, is not used by this
controller. Ready and ReadyUnknown are the generic Cluster API reasons.

Cluster API uses PoolExhausted for a pool that is full. A claim whose requested
address is in none of the pools is not waiting for a free address, so its
*Ready* reason is AllocationFailed and its *IPAddressAllocated* reason is
RequestedIPOutOfBounds, even when the pools are also full. Older versions
reported this case as PoolExhausted. Such a claim has no `status.addressRef`,
so it is reconciled again and gets the new reason without any other action.

When *Ready* is False or Unknown, its message lists the sub-conditions that
report the problem, one per line, as `* <condition type>: <message>`. When
*Ready* is True, it has no message.

##### Claims written by an older version

Older versions set *Ready* directly and did not have the sub-conditions. A
claim that has a *Ready* condition but no *IPAddressAllocated* condition keeps
that *Ready* condition unchanged until the controller sets *IPAddressAllocated*
on it. After that, *Ready* is computed as described above.

A claim that already has `status.addressRef` is only reconciled again when its
*Ready* reason is AllocationFailed or PoolExhausted, or when it is deleted. Any
other claim written by an older version is skipped, so it does not get the
sub-conditions.

### IpAddress

Check out more on [IPAddress docs](https://cluster-api.sigs.k8s.io/reference/api/crd-api-reference#ipaddress).

### Set up via clusterctl

Metal IPAM is an official IPAMProvider for CAPI. You can install Metal3 IPAM
on a cluster with:

```bash
clusterctl init --ipam metal3
```

Install older version by specifying version number:

```bash
clusterctl init --ipam metal3:v1.13.0
```

With infrastructure provider metal3:

```bash
clusterctl init --infrastructure metal3 --ipam metal3
```
