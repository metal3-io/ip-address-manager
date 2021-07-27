# Metal3 IP Address Manager for Cluster API Provider Metal3

[![Ubuntu V1alpha4 build status](https://jenkins.nordix.org/view/Airship/job/airship_master_v1a4_integration_test_ubuntu/badge/icon?subject=Ubuntu%20E2E%20V1alpha4)](https://jenkins.nordix.org/view/Airship/job/airship_master_v1a4_integration_test_ubuntu)
[![CentOS V1alpha4 build status](https://jenkins.nordix.org/view/Airship/job/airship_master_v1a4_integration_test_centos/badge/icon?subject=CentOS%20E2E%20V1alpha4)](https://jenkins.nordix.org/view/Airship/job/airship_master_v1a4_integration_test_centos)

This repository contains a controller to manage static IP address allocations
in [Cluster API Provider Metal3](https://github.com/metal3-io/cluster-api-provider-metal3/).

For more information about this controller and related repositories, see
[metal3.io](http://metal3.io/).

## Compatibility with Cluster API

| IPAM version      | CAPM3 version     | Cluster API version |
|-------------------|-------------------|---------------------|
| v1alpha1 (v0.1.X) | v1alpha4 (v0.4.X) | v1alpha3 (v0.3.X)   |

## Development Environment.

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

Runs CAPM3 controller locally

```sh
    kubectl scale -n capm3-system deployment.v1.apps/metal3-ipam-controller-manager \
      --replicas 0
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
