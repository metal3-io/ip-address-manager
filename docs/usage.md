# Using IP Address Manager

For the full field by field reference, see the [API reference](api.md).

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

Some notes on how the IPPool fields behave:

* **clusterName**: is used to check whether the owning cluster is paused.
* **preAllocations**: maps a claim's name to a fixed IP address. It doesn't
matter whether the claim is a (metal3) IPClaim or a (CAPI) IPAddressClaim.
* **prefix**, **gateway** and **dnsServers** set defaults for the whole pool
and each entry in **pools** can override them.
* Each entry in **pools** is either a range (**start** and optionally **end**)
or a **subnet**. If both are set, **subnet** is used to verify that the allocated
address belongs to it.

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

## Metal3 dev env examples

You can find CR examples in the
[Metal3-io dev env project](https://github.com/metal3-io/metal3-dev-env)

## Handling CAPI CRs

This IPAM can be deployed and used as an
[IPAM provider](https://cluster-api.sigs.k8s.io/developer/providers/contracts/ipam)
for [CAPI](https://github.com/kubernetes-sigs/cluster-api).

IPPool reconciles (metal3) ipclaims into (metal3) ipaddresses
and (capi) ipaddressclaims into (capi) ipaddresses.

### CAPI IPAddressClaim

Check out more on [IPAddressClaim docs](https://cluster-api.sigs.k8s.io/reference/api/crd-api-reference#ipaddressclaim).

### CAPI IPAddress

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
