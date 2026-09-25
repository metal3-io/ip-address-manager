# API Reference

Packages:

- [ipam.metal3.io/v1alpha1](#ipammetal3iov1alpha1)

# ipam.metal3.io/v1alpha1

Resource Types:

- [IPAddress](#ipaddress)

- [IPClaim](#ipclaim)

- [IPPool](#ippool)




## IPAddress
<sup><sup>[↩ Parent](#ipammetal3iov1alpha1 )</sup></sup>






IPAddress is the Schema for the ipaddresses API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>ipam.metal3.io/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>IPAddress</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#ipaddressspec">spec</a></b></td>
        <td>object</td>
        <td>
          IPAddressSpec defines the desired state of IPAddress.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPAddress.spec
<sup><sup>[↩ Parent](#ipaddress)</sup></sup>



IPAddressSpec defines the desired state of IPAddress.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>address</b></td>
        <td>string</td>
        <td>
          Address contains the IP address<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#ipaddressspecclaim">claim</a></b></td>
        <td>object</td>
        <td>
          Claim points to the object the IPClaim was created for.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#ipaddressspecpool">pool</a></b></td>
        <td>object</td>
        <td>
          Pool is the IPPool this was generated from.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>dnsServers</b></td>
        <td>[]string</td>
        <td>
          DNSServers is the list of dns servers<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>gateway</b></td>
        <td>string</td>
        <td>
          Gateway is the gateway ip address<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>integer</td>
        <td>
          Prefix is the mask of the network as integer (max 128)<br/>
          <br/>
            <i>Maximum</i>: 128<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPAddress.spec.claim
<sup><sup>[↩ Parent](#ipaddressspec)</sup></sup>



Claim points to the object the IPClaim was created for.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>apiVersion</b></td>
        <td>string</td>
        <td>
          API version of the referent.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>fieldPath</b></td>
        <td>string</td>
        <td>
          If referring to a piece of an object instead of an entire object, this string
should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].
For example, if the object reference is to a container within a pod, this would take on a value like:
"spec.containers{name}" (where "name" refers to the name of the container that triggered
the event) or if no container name is specified "spec.containers[2]" (container with
index 2 in this pod). This syntax is chosen only to have some well-defined way of
referencing a part of an object.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the referent.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>resourceVersion</b></td>
        <td>string</td>
        <td>
          Specific resourceVersion to which this reference is made, if any.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>uid</b></td>
        <td>string</td>
        <td>
          UID of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPAddress.spec.pool
<sup><sup>[↩ Parent](#ipaddressspec)</sup></sup>



Pool is the IPPool this was generated from.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>apiVersion</b></td>
        <td>string</td>
        <td>
          API version of the referent.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>fieldPath</b></td>
        <td>string</td>
        <td>
          If referring to a piece of an object instead of an entire object, this string
should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].
For example, if the object reference is to a container within a pod, this would take on a value like:
"spec.containers{name}" (where "name" refers to the name of the container that triggered
the event) or if no container name is specified "spec.containers[2]" (container with
index 2 in this pod). This syntax is chosen only to have some well-defined way of
referencing a part of an object.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the referent.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>resourceVersion</b></td>
        <td>string</td>
        <td>
          Specific resourceVersion to which this reference is made, if any.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>uid</b></td>
        <td>string</td>
        <td>
          UID of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## IPClaim
<sup><sup>[↩ Parent](#ipammetal3iov1alpha1 )</sup></sup>






IPClaim is the Schema for the ipclaims API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>ipam.metal3.io/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>IPClaim</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#ipclaimspec">spec</a></b></td>
        <td>object</td>
        <td>
          IPClaimSpec defines the desired state of IPClaim.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#ipclaimstatus">status</a></b></td>
        <td>object</td>
        <td>
          IPClaimStatus defines the observed state of IPClaim.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPClaim.spec
<sup><sup>[↩ Parent](#ipclaim)</sup></sup>



IPClaimSpec defines the desired state of IPClaim.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#ipclaimspecpool">pool</a></b></td>
        <td>object</td>
        <td>
          Pool is the IPPool this was generated from.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### IPClaim.spec.pool
<sup><sup>[↩ Parent](#ipclaimspec)</sup></sup>



Pool is the IPPool this was generated from.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>apiVersion</b></td>
        <td>string</td>
        <td>
          API version of the referent.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>fieldPath</b></td>
        <td>string</td>
        <td>
          If referring to a piece of an object instead of an entire object, this string
should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].
For example, if the object reference is to a container within a pod, this would take on a value like:
"spec.containers{name}" (where "name" refers to the name of the container that triggered
the event) or if no container name is specified "spec.containers[2]" (container with
index 2 in this pod). This syntax is chosen only to have some well-defined way of
referencing a part of an object.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the referent.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>resourceVersion</b></td>
        <td>string</td>
        <td>
          Specific resourceVersion to which this reference is made, if any.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>uid</b></td>
        <td>string</td>
        <td>
          UID of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPClaim.status
<sup><sup>[↩ Parent](#ipclaim)</sup></sup>



IPClaimStatus defines the observed state of IPClaim.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#ipclaimstatusaddress">address</a></b></td>
        <td>object</td>
        <td>
          Address is the IPAddress that was generated for this claim.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>errorMessage</b></td>
        <td>string</td>
        <td>
          ErrorMessage contains the error message<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPClaim.status.address
<sup><sup>[↩ Parent](#ipclaimstatus)</sup></sup>



Address is the IPAddress that was generated for this claim.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>apiVersion</b></td>
        <td>string</td>
        <td>
          API version of the referent.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>fieldPath</b></td>
        <td>string</td>
        <td>
          If referring to a piece of an object instead of an entire object, this string
should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].
For example, if the object reference is to a container within a pod, this would take on a value like:
"spec.containers{name}" (where "name" refers to the name of the container that triggered
the event) or if no container name is specified "spec.containers[2]" (container with
index 2 in this pod). This syntax is chosen only to have some well-defined way of
referencing a part of an object.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the referent.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>resourceVersion</b></td>
        <td>string</td>
        <td>
          Specific resourceVersion to which this reference is made, if any.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>uid</b></td>
        <td>string</td>
        <td>
          UID of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## IPPool
<sup><sup>[↩ Parent](#ipammetal3iov1alpha1 )</sup></sup>






IPPool is the Schema for the ippools API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>ipam.metal3.io/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>IPPool</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#ippoolspec">spec</a></b></td>
        <td>object</td>
        <td>
          IPPoolSpec defines the desired state of IPPool.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#ippoolstatus">status</a></b></td>
        <td>object</td>
        <td>
          IPPoolStatus defines the observed state of IPPool.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPPool.spec
<sup><sup>[↩ Parent](#ippool)</sup></sup>



IPPoolSpec defines the desired state of IPPool.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>namePrefix</b></td>
        <td>string</td>
        <td>
          namePrefix is the prefix used to generate the IPAddress object names<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>allocationStrategy</b></td>
        <td>enum</td>
        <td>
          AllocationStrategy defines how IP addresses are allocated from the pools.
"sequential" (default) allocates the first available IP.
"random" allocates a random available IP.
In both strategies, multiple pools are consumed in declaration order: a
pool is fully exhausted before the next one is used, and the strategy only
changes how an address is selected within a single pool.<br/>
          <br/>
            <i>Enum</i>: sequential, random<br/>
            <i>Default</i>: sequential<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>clusterName</b></td>
        <td>string</td>
        <td>
          ClusterName is the name of the Cluster this object belongs to.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dnsServers</b></td>
        <td>[]string</td>
        <td>
          DNSServers is the list of dns servers<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>gateway</b></td>
        <td>string</td>
        <td>
          Gateway is the gateway ip address<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#ippoolspecpoolsindex">pools</a></b></td>
        <td>[]object</td>
        <td>
          Pools contains the list of IP addresses pools<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>preAllocations</b></td>
        <td>map[string]string</td>
        <td>
          PreAllocations contains the preallocated IP addresses<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>integer</td>
        <td>
          Prefix is the mask of the network as integer (max 128)<br/>
          <br/>
            <i>Maximum</i>: 128<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPPool.spec.pools[index]
<sup><sup>[↩ Parent](#ippoolspec)</sup></sup>



MetaDataIPAddress contains the info to render th ip address. It is IP-version
agnostic.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>dnsServers</b></td>
        <td>[]string</td>
        <td>
          DNSServers is the list of dns servers<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>end</b></td>
        <td>string</td>
        <td>
          End is the last IP address that can be rendered. It is used as a validation
that the rendered IP is in bound.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>gateway</b></td>
        <td>string</td>
        <td>
          Gateway is the gateway ip address<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>integer</td>
        <td>
          Prefix is the mask of the network as integer (max 128)<br/>
          <br/>
            <i>Maximum</i>: 128<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>start</b></td>
        <td>string</td>
        <td>
          Start is the first ip address that can be rendered<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>subnet</b></td>
        <td>string</td>
        <td>
          Subnet is used to validate that the rendered IP is in bounds. In case the
Start value is not given, it is derived from the subnet ip incremented by 1
(`192.168.0.1` for `192.168.0.0/24`)<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### IPPool.status
<sup><sup>[↩ Parent](#ippool)</sup></sup>



IPPoolStatus defines the observed state of IPPool.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>indexes</b></td>
        <td>map[string]string</td>
        <td>
          Allocations contains the map of objects and IP addresses they have<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdated</b></td>
        <td>string</td>
        <td>
          LastUpdated identifies when this status was last observed.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>