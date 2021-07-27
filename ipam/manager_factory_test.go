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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Manager factory testing", func() {
	var managerClient client.Client
	var managerFactory ManagerFactory
	clusterLog := klogr.New()

	BeforeEach(func() {
		managerClient = fakeclient.NewClientBuilder().WithScheme(setupScheme()).Build()
		managerFactory = NewManagerFactory(managerClient)
	})

	It("returns a manager factory", func() {
		Expect(managerFactory.client).To(Equal(managerClient))
	})

	It("returns an IPPool manager", func() {
		_, err := managerFactory.NewIPPoolManager(&ipamv1.IPPool{}, clusterLog)
		Expect(err).NotTo(HaveOccurred())
	})

})
