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
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManagerFactoryInterface interface {
	NewIPPoolManager(*ipamv1.IPPool, logr.Logger) (
		IPPoolManagerInterface, error,
	)
}

// ManagerFactory only contains a client.
type ManagerFactory struct {
	client   client.Client
	recorder events.EventRecorder
}

// ManagerFactoryOption configures a ManagerFactory.
type ManagerFactoryOption func(*ManagerFactory)

// WithEventRecorder sets the event recorder used to emit events on IPPool objects.
func WithEventRecorder(recorder events.EventRecorder) ManagerFactoryOption {
	return func(f *ManagerFactory) { f.recorder = recorder }
}

// NewManagerFactory returns a new factory. Optional configuration such as an
// event recorder can be supplied via ManagerFactoryOption.
func NewManagerFactory(client client.Client, opts ...ManagerFactoryOption) ManagerFactory {
	f := ManagerFactory{client: client}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

// NewIPPoolManager creates a new IPPoolManager.
func (f ManagerFactory) NewIPPoolManager(ipPool *ipamv1.IPPool, metadataLog logr.Logger) (IPPoolManagerInterface, error) {
	mgr, err := NewIPPoolManager(f.client, ipPool, metadataLog)
	if err != nil {
		return nil, err
	}
	mgr.recorder = f.recorder
	return mgr, nil
}
