# How to Contribute to IP Address Manager

> **Note**: Please read the [common Metal3 contributing guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md)
> first. This document contains ip-address-manager-specific information.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Versioning](#versioning)
- [Backporting Policy](#backporting-policy)
- [Release Process](#release-process)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Versioning

See the [common versioning and release semantics](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#versioning-and-release-semantics)
in the Metal3 community contributing guide.

IP Address Manager follows [Cluster API](https://github.com/kubernetes-sigs/cluster-api)
versioning and release cadence guidelines.

## Backporting Policy

See the [common backporting guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#backporting)
in the Metal3 community contributing guide.

Additionally, for ip-address-manager:

- We generally do not accept backports to IPAM release branches that are out of
  support. Check the [Version support](https://github.com/metal3-io/metal3-docs/blob/main/docs/user-guide/src/version_support.md)
  guide for reference.

## Release Process

See the [common release process guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#release-process)
in the Metal3 community contributing guide.

For exact release steps specific to IPAM, refer to the [releasing document](./docs/releasing.md).
