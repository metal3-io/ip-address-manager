---
name: Minor release tracking issue
title: Prepare for v<release-tag>
---

Please see this documentation for more details about release process: https://github.com/metal3-io/metal3-docs/blob/main/processes/releasing.md


**Note**:
* The following is based on the v1.7 minor release. Modify according to the tracked minor release.

## Tasks
* [ ] Uplift CAPI to latest minor in IPAM, also check migration guide:[Migration guide for providers](https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/book/src/developer/providers/migrations). [Prior art ](https://github.com/metal3-io/ip-address-manager/pull/497).
IMPORTANT: Always read migration guide and make sure to do the changes accordingly in CAPM3 and IPAM.
* [ ] Verify all go modules are matching with hack/verify-release.sh script.
* [ ] Release IPAM (Branch out, add branch protection and required tests, also check new image is created in quay).
* [ ] Update IPAM README.md with the new e2e triggers. [Prior art](https://github.com/metal3-io/ip-address-manager/pull/504).
* [ ] Announce the releases
