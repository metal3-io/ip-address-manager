# Metal3 IP Address Manager for Cluster API Provider Metal3

[![CLOMonitor](https://img.shields.io/endpoint?url=https://clomonitor.io/api/projects/cncf/metal3-io/badge)](https://clomonitor.io/projects/cncf/metal3-io)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/9968/badge)](https://www.bestpractices.dev/projects/9968)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/metal3-io/ip-address-manager/badge)](https://securityscorecards.dev/viewer/?uri=github.com/metal3-io/ip-address-manager)
[![Ubuntu E2E Integration 1.10 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3-periodic-ubuntu-e2e-integration-test-release-1-10&subject=Ubuntu%20E2E%20integration%201.10)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3-periodic-ubuntu-e2e-integration-test-release-1-10/)
[![CentOS E2E Integration 1.10 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3-periodic-centos-e2e-integration-test-release-1-10&subject=Centos%20E2E%20integration%201.10)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3-periodic-centos-e2e-integration-test-release-1-10/)

This repository contains a controller to manage static IP address allocations
in [Cluster API Provider Metal3](https://github.com/metal3-io/cluster-api-provider-metal3/).

For more information about this controller and related repositories, see
[metal3.io](http://metal3.io/).

## Compatibility with Cluster API

| IPAM version      | CAPM3 version     | Cluster API version | IPAM Release |
|-------------------|-------------------|---------------------|--------------|
| v1alpha1 (v1.1.X) | v1beta1 (v1.1.X)  | v1beta1 (v1.1.X)    | v1.1.X       |
| v1alpha1 (v1.2.X) | v1beta1 (v1.2.X)  | v1beta1 (v1.2.X)    | v1.2.X       |
| v1alpha1 (v1.3.X) | v1beta1 (v1.3.X)  | v1beta1 (v1.3.X)    | v1.3.X       |
| v1alpha1 (v1.4.X) | v1beta1 (v1.4.X)  | v1beta1 (v1.4.X)    | v1.4.X       |
| v1alpha1 (v1.5.X) | v1beta1 (v1.5.X)  | v1beta1 (v1.5.X)    | v1.5.X       |
| v1alpha1 (v1.6.X) | v1beta1 (v1.6.X)  | v1beta1 (v1.6.X)    | v1.6.X       |
| v1alpha1 (v1.7.X) | v1beta1 (v1.7.X)  | v1beta1 (v1.7.X)    | v1.7.X       |
| v1alpha1 (v1.8.X) | v1beta1 (v1.8.X)  | v1beta1 (v1.8.X)    | v1.8.X       |
| v1alpha1 (v1.9.X)  | v1beta1 (v1.9.X)  | v1beta1 (v1.9.X)    | v1.9.X       |
| v1alpha1 (v1.10.X) | v1beta1 (v1.10.X) | v1beta1 (v1.10.X)   | v1.10.X      |

## Development Environment

See [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env) for an
end-to-end development and test environment for
[cluster-api-provider-metal3](https://github.com/metal3-io/cluster-api-provider-metal3/)
and [baremetal-operator](https://github.com/metal3-io/baremetal-operator).

## API

See the [API Documentation](docs/api.md) for details about the objects used with
this controller. You can also see the [cluster deployment
workflow](docs/deployment_workflow.md) for the outline of the
deployment process.

## Deployment and examples

### Deploy IPAM

Deploys IPAM CRDs and deploys IPAM controllers

```sh
    make deploy
```

### Run locally

Runs IPAM controller locally

```sh
    kubectl scale -n metal3-ipam-system \
      deployment.v1.apps/metal3-ipam-controller-manager --replicas 0
    make run
```

### Deploy an example pool

```sh
    make deploy-examples
```

### Delete the example pool

```sh
    make delete-examples
```

#### Note

There is a known limitation when `kubectl apply` and `kubectl delete` for an
`IPClaim` are executed in rapid succession (within the same second). In such
cases, the `IPClaim` may be deleted, but the associated `IPAddress` might not
be removed as expected, potentially leading to resource inconsistencies. This
is not a common or typical use case. In normal scenarios, the operations work
as expected. Since this issue occurs only under rare timing conditions, it has
been classified as a low-priority item. We plan to address it in a future and
it is currently documented as a known limitation.
