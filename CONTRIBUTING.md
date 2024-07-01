# How to Contribute

Metal3 projects are [Apache 2.0 licensed](LICENSE) and accept contributions via
GitHub pull requests. Those guidelines are the same as the
[Cluster API guidelines](https://github.com/kubernetes-sigs/cluster-api/blob/main/CONTRIBUTING.md)

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Certificate of Origin](#certificate-of-origin)
   - [Git commit Sign-off](#git-commit-sign-off)
- [Finding Things That Need Help](#finding-things-that-need-help)
- [Contributing a Patch](#contributing-a-patch)
- [Backporting](#backporting)
   - [Merge Approval](#merge-approval)
   - [Google Doc Viewing Permissions](#google-doc-viewing-permissions)
   - [Issue and Pull Request Management](#issue-and-pull-request-management)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](DCO) file for details.

### Git commit Sign-off

Commit message should contain signed off section with full name and email. For example:

 ```text
  Signed-off-by: John Doe <jdoe@example.com>
 ```

When making commits, include the `-s` flag and `Signed-off-by` section will be
automatically added to your commit message. If you want GPG signing too, add
the `-S` flag alongside `-s`.

```bash
  # Signing off commit
  git commit -s

  # Signing off commit and also additional signing with GPG
  git commit -s -S
```

## Finding Things That Need Help

If you're new to the project and want to help, but don't know where to start, we
have a semi-curated list of issues that
should not need deep knowledge of the system. [Have a look and see if anything
sounds interesting](https://github.com/metal3-io/ip-address-manager/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22).
Alternatively, read some of the docs on other controllers and try to write your
own, file and fix any/all issues that come up, including gaps in documentation!

## Contributing a Patch

1. If you haven't already done so, sign a Contributor License Agreement (see
   details above).
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.

All code PR must be labeled with one of

- ⚠️ (:warning:, major or breaking changes)
- ✨ (:sparkles:, minor or feature additions)
- 🐛 (:bug:, patch and bugfixes)
- 📖 (:book:, documentation or proposals)
- 🌱 (:seedling:, minor or other)

All changes must be code reviewed. Coding conventions and standards are
explained in the official [developer
docs](https://github.com/kubernetes/community/tree/master/contributors/devel).
Expect reviewers to request that you
avoid common [go style
mistakes](https://github.com/golang/go/wiki/CodeReviewComments) in your PRs.

## Backporting

We generally do not accept PRs directly against release branches, while we might
accept backports of fixes/changes already merged into the main branch.

We generally allow backports of following changes to all supported branches:

- Critical bug fixes, security issue fixes, or fixes for bugs without easy
workarounds.
- Dependency bumps for CVE (usually limited to CVE resolution; backports of
non-CVE related version bumps are considered exceptions to be evaluated case by
case)
- Changes required to support new Kubernetes patch versions, when possible.
- Changes to use the latest Go patch version to build controller images.
- Changes to bump the Go minor version used to build controller images, if the
Go minor version of a supported branch goes out of support (e.g. to pick up
bug and CVE fixes). This has no impact on users importing IP Address Manager
as we won't modify the version in go.mod and the version in the Makefile
does not affect them.
- Improvements to existing docs
- Improvements to the test framework

Like any other activity in the project, backporting a fix/change is a
community-driven effort and requires that someone volunteers to own the task.
In most cases, the cherry-pick bot can (and should) be used to automate
opening a cherry-pick PR.

We generally do not accept backports to IPAM release branches that are out of
support. Check the [Version support](https://github.com/metal3-io/metal3-docs/blob/main/docs/user-guide/src/version_support.md)
guide for reference.

## Breaking Changes

Breaking changes are generally allowed in the `main` branch, as this is the
branch used to develop the next minor release of Cluster API.

There may be times, however, when `main` is closed for breaking changes. This
is likely to happen as we near the release of a new minor version.

Breaking changes are not allowed in release branches, as these represent minor
versions that have already been released.
These versions have consumers who expect the APIs, behaviors, etc. to remain
stable during the life time of the patch stream for the minor release.

Examples of breaking changes include:

- Removing or renaming a field in a CRD
- Removing or renaming a CRD
- Removing or renaming an exported constant, variable, type, or function
- Updating the version of critical libraries such as controller-runtime,
  client-go, apimachinery, etc.
- Some version updates may be acceptable, for picking up bug fixes, but
  maintainers must exercise caution when reviewing.

There may, at times, need to be exceptions where breaking changes are allowed in
release branches. These are at the discretion of the project's maintainers, and
must be carefully considered before merging. An example of an allowed
breaking change might be a fix for a behavioral bug that was released in an
initial minor version (such as `v0.3.0`).

### Merge Approval

Please see the [Kubernetes community document on pull
requests](https://git.k8s.io/community/contributors/guide/pull-requests.md) for
more information about the merge process.

### Google Doc Viewing Permissions

To gain viewing permissions to google docs in this project, please join the
[metal3-dev](https://groups.google.com/forum/#!forum/metal3-dev) google
group.

### Issue and Pull Request Management

Anyone may comment on issues and submit reviews for pull requests. However, in
order to be assigned an issue or pull request, you must be a member of the
[Metal3-io organization](https://github.com/metal3-io) GitHub organization.

Metal3 maintainers can assign you an issue or pull request by leaving a
`/assign <your Github ID>` comment on the issue or pull request.
