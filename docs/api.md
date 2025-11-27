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
Preallocations asossiate a claim's name to an IP address. It doesn't matter if
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
[IPAM provider](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/book/src/reference/glossary.md#ipam-provider)for
[CAPI](https://github.com/kubernetes-sigs/cluster-api).

IPPool reconsiles (metal3)ipclaims into (metal3)ipaddresses
and (capi)ipaddressclaims into (capi)ipaddresses.

### IPAddressClaim

Check out more on [IPAddressClaim docs](https://docs.openshift.com/container-platform/4.16/rest_api/network_apis/ipaddressclaim-ipam-cluster-x-k8s-io-v1beta1.html).

### IpAddress

Check out more on [IPAddress docs](https://docs.openshift.com/container-platform/4.16/rest_api/network_apis/ipaddress-ipam-cluster-x-k8s-io-v1beta1.html).

### Set up via clusterctl

Since it's not added to the built-in list of providers yet,
you'll need to add the following to your
```$XDG_CONFIG_HOME/cluster-api/clusterctl.yaml```
if you want to install it using ```clusterctl init --ipam metal3```:

```yaml
providers:
- name: metal3
  url: https://github.com/metal3-io/ip-address-manager/releases/latest/ipam-components.yaml
  type: IPAMProvider
```

If you are also specifying infrastructure provider
metal3 liko so:
```clusterctl init --infrastructure metal3 --ipam metal3```.
It might cause a problem to have the same name
for both providers if you are creating local
[overrides layers](https://cluster-api.sigs.k8s.io/clusterctl/configuration#overrides-layer).
Solution is to change the ipam providers name:
```clusterctl init --infrastructure metal3 --ipam m3ipam``

```yaml
providers:
- name: m3ipam
  url: https://github.com/metal3-io/ip-address-manager/releases/latest/ipam-components.yaml
  type: IPAMProvider
```
