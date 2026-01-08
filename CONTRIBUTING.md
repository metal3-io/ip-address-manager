# How to Contribute to IP Address Manager

> **Note**: Please read the [common Metal3 contributing guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md)
> first. This document contains IP Address Manager specific information.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Versioning](#versioning)
- [Branches](#branches)
   - [Maintenance and Guarantees](#maintenance-and-guarantees)
- [Backporting](#backporting)
- [Release Process](#release-process)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Versioning

See the [common versioning and release semantics](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#versioning-and-release-semantics)
in the Metal3 community contributing guide.

IP Address Manager follows [Cluster API](https://github.com/kubernetes-sigs/cluster-api)
versioning and release cadence guidelines.

## Branches

For general branch structure, see the [Branches](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#branches)
section in the common contributing guide.

### Maintenance and Guarantees

IP Address Manager maintains the most recent release/releases for all
supported API and contract versions. Support for this section refers to the
ability to backport and release patch versions; [backport policy](#backporting)
is defined below.

## Backporting

See the [common backporting guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#backporting)
in the Metal3 community contributing guide.

Additionally, for IP Address Manager:

- We generally do not accept backports to IP Address Manager release branches that
  are out of support. Check the [Version support](https://github.com/metal3-io/metal3-docs/blob/main/docs/user-guide/src/version_support.md)
  guide for reference.

## Release Process

See the [common release process guidelines](https://github.com/metal3-io/community/blob/main/CONTRIBUTING.md#release-process)
in the Metal3 community contributing guide.

For exact release steps specific to IP Address Manager, refer to the
[releasing document](./docs/releasing.md).
