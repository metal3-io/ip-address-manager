# Metal3 IP Address Manager for Cluster API Provider Metal4

[![Ubuntu E2E Integration 1.5 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_release-1-5_e2e_integration_test_ubuntu&subject=Ubuntu%20e2e%20integration%201.5)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3_daily_release-1-5_integration_test_ubuntu/)
[![CentOS E2E Integration 1.5 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_release-1-5_e2e_integration_test_centos&subject=Centos%20e2e%20integration%201.5)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3_daily_release-1-5_integration_test_centos/)
[![Ubuntu E2E feature 1.5 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_release-1-5_e2e_feature_test_ubuntu/&subject=Ubuntu%20E2E%20feature%201.5)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3_daily_release-1-5_e2e_feature_test_ubuntu/)
[![CentOS E2E feature 1.5 build status](https://jenkins.nordix.org/buildStatus/icon?job=metal3_daily_release-1-5_e2e_feature_test_centos/&subject=CentOS%20E2E%20feature%201.5)](https://jenkins.nordix.org/view/Metal3%20Periodic/job/metal3_daily_release-1-5_e2e_feature_test_centos/)

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
