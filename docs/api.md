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
* **preAllocations**: This is a default preallocated IP address for this IPPool

The *prefix* and *gateway* can be overridden per pool. The pool definition is
as follows :

* **start**: the IP range start address. Can be omitted if **subnet** is set.
* **end**: the IP range end address. Can be omitted.
* **subnet**: the subnet for the allocation. Can be omitted if **start** is set.
  It is used to verify that the allocated address belongs to this subnet.
* **prefix**: override of the default prefix for this pool
* **gateway**: override of the default gateway for this pool

## IPClaim

An IPClaim is an object representing a request for an IP address allocation.

Example pool:

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

Example pool:

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

## Metal3 dev env examples

You can find CR examples in the
[Metal3-io dev env project](https://github.com/metal3-io/metal3-dev-env)
