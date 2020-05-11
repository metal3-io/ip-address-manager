# Deployment workflow

## Deploying the controllers

The following controllers need to be deployed :

* CAPI
* IPAM

The IPAM controller has a dependency on Cluster API *Cluster* objects.

## Requirements

CAPI CRDs and controllers must be deployed and the cluster objects exist for
successful deployments

## Deployment

You can create the **IPPool** object independently. It will wait for its cluster
to exist before reconciling.

If you wish to deploy **IPAddress** objects manually, then they should be
deployed before any claims. It is highly recommended to use the *preAllocations*
field itself or have the reconciliation paused (during clusterctl move for
example).

After an **IPClaim** object creation, the controller will list all existing
**IPAddress** objects. It will then select randomly an address that has not been
allocated yet and is not in the *preAllocations* map. It will then create an
**IPAddress** object containing the references to the **IPPool** and **IPClaim**
and the address, the prefix from the address pool or the default prefix, and the
gateway from the address pool or the default gateway.

## Deletion

When deleting and **IPClaim** object, the controller will simply delete the
associated **IPAddress** object. Once all **IPAddress** objects have been
deleted, the **IPPool** object can be deleted. Before that point, the finalizer
in the **IPPool** object will block the deletion.
