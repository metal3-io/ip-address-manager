# Metal3 IP Address Manager for Cluster API Provider Metal3 - do not merge - this is a test

[![CLOMonitor](https://img.shields.io/endpoint?url=https://clomonitor.io/api/projects/cncf/metal3-io/badge)](https://clomonitor.io/projects/cncf/metal3-io)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/metal3-io/ip-address-manager/badge)](https://securityscorecards.dev/viewer/?uri=github.com/metal3-io/ip-address-manager)
[![Ubuntu daily main build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_main_integration_test_ubuntu&subject=Ubuntu%20daily%20main)](https://jenkins.nordix.org/view/Metal3/job/metal3_daily_main_integration_test_ubuntu/)
[![CentOS daily main build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_main_integration_test_centos&subject=CentOS%20daily%20main)](https://jenkins.nordix.org/view/Metal3/job/metal3_daily_main_integration_test_centos/)

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
    kubectl scale -n capm3-system \
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
