/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ipam

import (
	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManagerFactoryInterface interface {
	NewIPPoolManager(*ipamv1.IPPool, logr.Logger) (
		IPPoolManagerInterface, error,
	)
}

// ManagerFactory only contains a client.
type ManagerFactory struct {
	client client.Client
}

// NewManagerFactory returns a new factory.
func NewManagerFactory(client client.Client) ManagerFactory {
	return ManagerFactory{client: client}
}

// NewIPPoolManager creates a new IPPoolManager.
func (f ManagerFactory) NewIPPoolManager(ipPool *ipamv1.IPPool, metadataLog logr.Logger) (IPPoolManagerInterface, error) {
	return NewIPPoolManager(f.client, ipPool, metadataLog)
}
