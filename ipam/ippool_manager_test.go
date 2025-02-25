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
	"context"
	"reflect"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testObjectMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: "myns",
	}
	testObjectReference = &corev1.ObjectReference{
		Name: "abc",
	}
	localtestObjectReference = &corev1.LocalObjectReference{
		Name: "abc",
	}
	typedtestObjectReference = &corev1.TypedLocalObjectReference{
		Name: "abc",
	}
)

var _ = Describe("IPPool manager", func() {
	DescribeTable("Test Finalizers",
		func(ipPool *ipamv1.IPPool) {
			ipPoolMgr, err := NewIPPoolManager(nil, ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			ipPoolMgr.SetFinalizer()

			Expect(ipPool.ObjectMeta.Finalizers).To(ContainElement(
				ipamv1.IPPoolFinalizer,
			))

			ipPoolMgr.UnsetFinalizer()

			Expect(ipPool.ObjectMeta.Finalizers).NotTo(ContainElement(
				ipamv1.IPPoolFinalizer,
			))
		},
		Entry("No finalizers", &ipamv1.IPPool{}),
		Entry("Additional Finalizers", &ipamv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"foo"},
			},
		}),
	)

	type testCaseSetClusterOwnerRef struct {
		cluster     *clusterv1.Cluster
		ipPool      *ipamv1.IPPool
		expectError bool
	}

	DescribeTable("Test SetClusterOwnerRef",
		func(tc testCaseSetClusterOwnerRef) {
			ipPoolMgr, err := NewIPPoolManager(nil, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			err = ipPoolMgr.SetClusterOwnerRef(tc.cluster)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				_, err := findOwnerRefFromList(tc.ipPool.OwnerReferences,
					tc.cluster.TypeMeta, tc.cluster.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("Cluster missing", testCaseSetClusterOwnerRef{
			expectError: true,
		}),
		Entry("no previous ownerref", testCaseSetClusterOwnerRef{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
		Entry("previous ownerref", testCaseSetClusterOwnerRef{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "def",
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
		Entry("ownerref present", testCaseSetClusterOwnerRef{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "def",
						},
						{
							Name: "abc-cluster",
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
	)

	type testGetIndexes struct {
		ipPool              *ipamv1.IPPool
		addresses           []*ipamv1.IPAddress
		capiAddresses       []*capipamv1.IPAddress
		expectError         bool
		expectedAddresses   map[ipamv1.IPAddressStr]string
		expectedAllocations map[string]ipamv1.IPAddressStr
	}

	DescribeTable("Test getIndexes",
		func(tc testGetIndexes) {
			objects := []client.Object{}
			for _, address := range tc.addresses {
				objects = append(objects, address)
			}
			for _, address := range tc.capiAddresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			previousAllocations := tc.ipPool.Status.Allocations
			if previousAllocations == nil {
				previousAllocations = make(map[string]ipamv1.IPAddressStr)
			}

			addressMap, err := ipPoolMgr.getIndexes(context.TODO())
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(addressMap).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
			if !reflect.DeepEqual(previousAllocations, tc.ipPool.Status.Allocations) {
				Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
			} else {
				Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeTrue())
			}

		},
		Entry("No addresses", testGetIndexes{
			ipPool:              &ipamv1.IPPool{},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
		}),
		Entry("addresses", testGetIndexes{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec: ipamv1.IPPoolSpec{
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"bcd": ipamv1.IPAddressStr("bcde"),
					},
				},
			},
			addresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd1",
						Pool:    *testObjectReference,
						Claim:   *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bbc-1",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd2",
						Pool: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-2",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd3",
						Pool:    corev1.ObjectReference{},
						Claim:   *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd4",
						Pool: corev1.ObjectReference{
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{},
					},
				},
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("abcd1"): "abc",
				ipamv1.IPAddressStr("bcde"):  "",
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": ipamv1.IPAddressStr("abcd1"),
			},
		}),
		Entry("capi addresses", testGetIndexes{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec: ipamv1.IPPoolSpec{
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"cbcd": "cbcde",
					},
				},
			},
			capiAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabc-0",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address:  "cabcd1",
						PoolRef:  *typedtestObjectReference,
						ClaimRef: *localtestObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cbbc-1",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address: "cabcd2",
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "cbbc",
						},
						ClaimRef: corev1.LocalObjectReference{
							Name: "cbbc",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabc-2",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address:  "cabcd3",
						PoolRef:  corev1.TypedLocalObjectReference{},
						ClaimRef: *localtestObjectReference,
					},
				},
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				"cabcd1": "abc",
				"cbcde":  "",
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": "cabcd1",
			},
		}),
		Entry("metal3 addresses and capi addresses", testGetIndexes{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec: ipamv1.IPPoolSpec{
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"bcd":  "bcde",
						"cbcd": "cbcde",
					},
				},
			},
			addresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-0",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd1",
						Pool:    *testObjectReference,
						Claim:   *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bbc-1",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd2",
						Pool: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-2",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd3",
						Pool:    corev1.ObjectReference{},
						Claim:   *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd4",
						Pool: corev1.ObjectReference{
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{},
					},
				},
			},
			capiAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cbbc-1",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address: "cabcd2",
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "cbbc",
						},
						ClaimRef: corev1.LocalObjectReference{
							Name: "cbbc",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabc-2",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address:  "cabcd3",
						PoolRef:  corev1.TypedLocalObjectReference{},
						ClaimRef: *localtestObjectReference,
					},
				},
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				"abcd1": "abc",
				"bcde":  "",
				"cbcde": "",
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": "abcd1",
			},
		}),
		Entry("metal3 addresses and capi addresses 2", testGetIndexes{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec: ipamv1.IPPoolSpec{
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"bcd":  "bcde",
						"cbcd": "cbcde",
					},
				},
			},
			addresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bbc-1",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd2",
						Pool: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "bbc",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-2",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd3",
						Pool:    corev1.ObjectReference{},
						Claim:   *testObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc-3",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "abcd4",
						Pool: corev1.ObjectReference{
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{},
					},
				},
			},
			capiAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabc-0",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address:  "cabcd1",
						PoolRef:  *typedtestObjectReference,
						ClaimRef: *localtestObjectReference,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cbbc-1",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address: "cabcd2",
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "cbbc",
						},
						ClaimRef: corev1.LocalObjectReference{
							Name: "cbbc",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabc-2",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address:  "cabcd3",
						PoolRef:  corev1.TypedLocalObjectReference{},
						ClaimRef: *localtestObjectReference,
					},
				},
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				"bcde":   "",
				"cabcd1": "abc",
				"cbcde":  "",
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": "cabcd1",
			},
		}),
		Entry("IPPool with deletion timestamp (metal3 addresses)", testGetIndexes{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &timeNow,
				},
				Spec: ipamv1.IPPoolSpec{
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"bcd": ipamv1.IPAddressStr("bcde"),
					},
				},
			},
			addresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-11",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "inUseClaim",
							Namespace: "myns",
						},
						Address: ipamv1.IPAddressStr("192.168.1.11"),
						Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
						Prefix:  24,
					},
				},
			},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
		}),
		Entry("IPPool with deletion timestamp (capi addresses)", testGetIndexes{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &timeNow,
				},
				Spec: ipamv1.IPPoolSpec{
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"bcd": "bcde",
					},
				},
			},
			capiAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-11",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						PoolRef: *typedtestObjectReference,
						ClaimRef: corev1.LocalObjectReference{
							Name: "inUseClaim",
						},
						Address: "192.168.1.11",
						Gateway: *ptr.To("192.168.0.1"),
						Prefix:  24,
					},
				},
			},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
		}),
		Entry("IPPool with deletion timestamp (metal3 and capi addresses)", testGetIndexes{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &timeNow,
				},
				Spec: ipamv1.IPPoolSpec{
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"bcd": "bcde",
					},
				},
			},
			addresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-11",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "inUseClaim",
							Namespace: "myns",
						},
						Address: "192.168.1.11",
						Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
						Prefix:  24,
					},
				},
			},
			capiAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabcpref-192-168-1-12",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "cabc",
						},
						ClaimRef: corev1.LocalObjectReference{
							Name: "inUseClaim",
						},
						Address: "192.168.1.12",
						Gateway: *(ptr.To("192.168.0.1")),
						Prefix:  24,
					},
				},
			},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
		}),
	)

	var ipPoolMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: "myns",
	}

	type testCaseUpdateAddresses struct {
		ipPool                *ipamv1.IPPool
		ipClaims              []*ipamv1.IPClaim
		ipAddresses           []*ipamv1.IPAddress
		ipAddressClaims       []*capipamv1.IPAddressClaim
		capiAddresses         []*capipamv1.IPAddress
		expectRequeue         bool
		expectError           bool
		expectedNbAllocations int
		expectedAllocations   map[string]ipamv1.IPAddressStr
	}

	DescribeTable("Test UpdateAddresses",
		func(tc testCaseUpdateAddresses) {
			objects := []client.Object{}
			for _, address := range tc.ipAddresses {
				objects = append(objects, address)
			}
			for _, claim := range tc.ipClaims {
				objects = append(objects, claim)
			}
			for _, address := range tc.capiAddresses {
				objects = append(objects, address)
			}
			for _, claim := range tc.ipAddressClaims {
				objects = append(objects, claim)
			}
			c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).WithStatusSubresource(objects...).WithObjects(objects...).Build()
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			nbAllocations, err := ipPoolMgr.UpdateAddresses(context.TODO())
			if tc.expectRequeue || tc.expectError {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(nbAllocations).To(Equal(tc.expectedNbAllocations))
			Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))

			// get list of IPAddress objects
			addressObjects := ipamv1.IPClaimList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			// Iterate over the IPAddress objects to find all indexes and objects
			for _, claim := range addressObjects.Items {
				if claim.DeletionTimestamp.IsZero() {
					Expect(claim.Status.Address).NotTo(BeNil())
				}
			}

			// get list of IPAddress objects
			capiAddressObjects := capipamv1.IPAddressClaimList{}
			err = c.List(context.TODO(), &capiAddressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			// Iterate over the IPAddress objects to find all indexes and objects
			for _, claim := range capiAddressObjects.Items {
				if claim.DeletionTimestamp.IsZero() {
					Expect(claim.Status.AddressRef).NotTo(BeNil())
				}
			}

		},
		Entry("No Claims", testCaseUpdateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
		}),
		Entry("Claim and IP exist", testCaseUpdateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					NamePrefix: "abcpref",
				},
			},
			ipClaims: []*ipamv1.IPClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "myns",
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: ipamv1.IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-11",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcd",
						Namespace: "myns",
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abcd",
							Namespace: "myns",
						},
					},
					Status: ipamv1.IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-12",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abce",
						Namespace: "myns",
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: ipamv1.IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-12",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "abcf",
						Namespace:         "myns",
						DeletionTimestamp: &timeNow,
						Finalizers: []string{
							ipamv1.IPClaimFinalizer,
						},
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: ipamv1.IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-13",
							Namespace: "myns",
						},
					},
				},
			},
			ipAddresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-11",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Address: ipamv1.IPAddressStr("192.168.1.11"),
						Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
						Prefix:  24,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-12",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abce",
							Namespace: "myns",
						},
						Address: ipamv1.IPAddressStr("192.168.1.12"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-13",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abcf",
							Namespace: "myns",
						},
						Address: ipamv1.IPAddressStr("192.168.1.13"),
					},
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc":  ipamv1.IPAddressStr("192.168.1.11"),
				"abce": ipamv1.IPAddressStr("192.168.1.12"),
			},
			expectedNbAllocations: 2,
		}),
		Entry("IPAddressClaim and IP exist", testCaseUpdateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					NamePrefix: "abcpref",
				},
			},
			ipAddressClaims: []*capipamv1.IPAddressClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressClaimSpec{
						PoolRef: *typedtestObjectReference,
					},
					Status: capipamv1.IPAddressClaimStatus{
						AddressRef: corev1.LocalObjectReference{
							Name: "abcpref-192-168-1-11",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcd",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressClaimSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "abcd",
						},
					},
					Status: capipamv1.IPAddressClaimStatus{
						AddressRef: corev1.LocalObjectReference{
							Name: "abcpref-192-168-1-12",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abce",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressClaimSpec{
						PoolRef: *typedtestObjectReference,
					},
					Status: capipamv1.IPAddressClaimStatus{
						AddressRef: corev1.LocalObjectReference{
							Name: "abcpref-192-168-1-12",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "abcf",
						Namespace:         "myns",
						DeletionTimestamp: &timeNow,
						Finalizers: []string{
							IPAddressClaimFinalizer,
						},
					},
					Spec: capipamv1.IPAddressClaimSpec{
						PoolRef: *typedtestObjectReference,
					},
					Status: capipamv1.IPAddressClaimStatus{
						AddressRef: corev1.LocalObjectReference{
							Name: "abcpref-192-168-1-13",
						},
					},
				},
			},
			capiAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-11",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						PoolRef:  *typedtestObjectReference,
						ClaimRef: *localtestObjectReference,
						Address:  "192.168.1.11",
						Gateway:  *ptr.To("192.168.0.1"),
						Prefix:   24,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-12",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						PoolRef: *typedtestObjectReference,
						ClaimRef: corev1.LocalObjectReference{
							Name: "abce",
						},
						Address: "192.168.1.12",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-13",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						PoolRef: *typedtestObjectReference,
						ClaimRef: corev1.LocalObjectReference{
							Name: "abcf",
						},
						Address: "192.168.1.13",
					},
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc":  "192.168.1.11",
				"abce": "192.168.1.12",
			},
			expectedNbAllocations: 2,
		}),
		Entry("Both IPClaim and IPAddressClaim and IP exist", testCaseUpdateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					NamePrefix: "abcpref",
				},
			},
			ipClaims: []*ipamv1.IPClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abc",
						Namespace: "myns",
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: ipamv1.IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-11",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcd",
						Namespace: "myns",
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abcd",
							Namespace: "myns",
						},
					},
					Status: ipamv1.IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-12",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abce",
						Namespace: "myns",
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: ipamv1.IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-12",
							Namespace: "myns",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "abcf",
						Namespace:         "myns",
						DeletionTimestamp: &timeNow,
						Finalizers: []string{
							ipamv1.IPClaimFinalizer,
						},
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
					Status: ipamv1.IPClaimStatus{
						Address: &corev1.ObjectReference{
							Name:      "abcpref-192-168-1-13",
							Namespace: "myns",
						},
					},
				},
			},
			ipAddresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-11",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Address: "192.168.1.11",
						Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
						Prefix:  24,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-12",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abce",
							Namespace: "myns",
						},
						Address: "192.168.1.12",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-13",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "abcf",
							Namespace: "myns",
						},
						Address: "192.168.1.13",
					},
				},
			},
			ipAddressClaims: []*capipamv1.IPAddressClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabc",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressClaimSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "cabc",
						},
					},
					Status: capipamv1.IPAddressClaimStatus{
						AddressRef: corev1.LocalObjectReference{
							Name: "cabcpref-192-168-1-14",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabcd",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressClaimSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "cabcd",
						},
					},
					Status: capipamv1.IPAddressClaimStatus{
						AddressRef: corev1.LocalObjectReference{
							Name: "cabcpref-192-168-1-15",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabce",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressClaimSpec{
						PoolRef: *typedtestObjectReference,
					},
					Status: capipamv1.IPAddressClaimStatus{
						AddressRef: corev1.LocalObjectReference{
							Name: "cabcpref-192-168-1-15",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cabcf",
						Namespace:         "myns",
						DeletionTimestamp: &timeNow,
						Finalizers: []string{
							IPAddressClaimFinalizer,
						},
					},
					Spec: capipamv1.IPAddressClaimSpec{
						PoolRef: *typedtestObjectReference,
					},
					Status: capipamv1.IPAddressClaimStatus{
						AddressRef: corev1.LocalObjectReference{
							Name: "cabcpref-192-168-1-16",
						},
					},
				},
			},
			capiAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabcpref-192-168-1-14",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "cabc",
						},
						ClaimRef: corev1.LocalObjectReference{
							Name: "cabc",
						},
						Address: "192.168.1.14",
						Gateway: *(ptr.To("192.168.0.1")),
						Prefix:  24,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabcpref-192-168-1-15",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						PoolRef: *typedtestObjectReference,
						ClaimRef: corev1.LocalObjectReference{
							Name: "cabce",
						},
						Address: "192.168.1.15",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cabcpref-192-168-1-16",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						PoolRef: *typedtestObjectReference,
						ClaimRef: corev1.LocalObjectReference{
							Name: "cabcf",
						},
						Address: "192.168.1.16",
					},
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc":   "192.168.1.11",
				"abce":  "192.168.1.12",
				"cabce": "192.168.1.15",
			},
			expectedNbAllocations: 3,
		}),
		Entry("IPClaim with deletion timestamp and finalizers", testCaseUpdateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{},
			},
			ipClaims: []*ipamv1.IPClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "inUseClaim",
						Namespace:         "myns",
						DeletionTimestamp: &timeNow,
						Finalizers: []string{
							ipamv1.IPClaimFinalizer,
							"metal3data.infrastructure.cluster.x-k8s.io",
						},
					},
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
				},
			},
			ipAddresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-1-11",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
						Claim: corev1.ObjectReference{
							Name:      "inUseClaim",
							Namespace: "myns",
						},
						Address: ipamv1.IPAddressStr("192.168.1.11"),
						Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
						Prefix:  24,
					},
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"inUseClaim": ipamv1.IPAddressStr("192.168.1.11"),
			},
			expectedNbAllocations: 1,
		}),
	)

	type testCaseCreateAddresses struct {
		ipPool              *ipamv1.IPPool
		ipClaim             *ipamv1.IPClaim
		ipAddresses         []*ipamv1.IPAddress
		addresses           map[ipamv1.IPAddressStr]string
		expectRequeue       bool
		expectError         bool
		expectedIPAddresses []string
		expectedAddresses   map[ipamv1.IPAddressStr]string
		expectedAllocations map[string]ipamv1.IPAddressStr
	}

	DescribeTable("Test CreateAddresses",
		func(tc testCaseCreateAddresses) {
			objects := []client.Object{}
			for _, address := range tc.ipAddresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := ipPoolMgr.createAddress(context.TODO(), tc.ipClaim,
				tc.addresses,
			)
			if tc.expectRequeue || tc.expectError {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			// get list of IPAddress objects
			addressObjects := ipamv1.IPAddressList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(tc.expectedIPAddresses)).To(Equal(len(addressObjects.Items)))
			// Iterate over the IPAddress objects to find all indexes and objects
			for _, address := range addressObjects.Items {
				Expect(tc.expectedIPAddresses).To(ContainElement(address.Name))
				// TODO add further testing later
			}
			Expect(len(tc.ipClaim.Finalizers)).To(Equal(1))

			Expect(allocatedMap).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
		},
		Entry("Already exists", testCaseCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{
						"abc": ipamv1.IPAddressStr("foo-0"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": ipamv1.IPAddressStr("foo-0"),
			},
		}),
		Entry("Not allocated yet, pre-allocated", testCaseCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"abc": ipamv1.IPAddressStr("192.168.0.15"),
					},
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": ipamv1.IPAddressStr("192.168.0.15"),
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.15"): "abc",
			},
			expectedIPAddresses: []string{"abcpref-192-168-0-15"},
		}),
		Entry("Not allocated yet", testCaseCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.11"): "bcd",
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": ipamv1.IPAddressStr("192.168.0.12"),
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.12"): "abc",
				ipamv1.IPAddressStr("192.168.0.11"): "bcd",
			},
			expectedIPAddresses: []string{"abcpref-192-168-0-12"},
		}),
		Entry("Not allocated yet, conflict", testCaseCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			ipAddresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-0-11",
						Namespace: "myns",
					},
					Spec: ipamv1.IPAddressSpec{
						Address: "192.168.0.11",
						Pool: corev1.ObjectReference{
							Name: "abc",
						},
						Claim: corev1.ObjectReference{
							Name: "bcd",
						},
					},
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
			expectedIPAddresses: []string{"abcpref-192-168-0-11"},
			expectRequeue:       true,
		}),
		Entry("Not allocated yet, exhausted pool", testCaseCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.11"): "bcd",
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.11"): "bcd",
			},
			expectedIPAddresses: []string{},
			expectError:         true,
		}),
	)

	type testCaseCapiCreateAddresses struct {
		ipPool              *ipamv1.IPPool
		ipAddressClaim      *capipamv1.IPAddressClaim
		ipAddresses         []*capipamv1.IPAddress
		addresses           map[ipamv1.IPAddressStr]string
		expectRequeue       bool
		expectError         bool
		expectedIPAddresses []string
		expectedAddresses   map[ipamv1.IPAddressStr]string
		expectedAllocations map[string]ipamv1.IPAddressStr
	}

	DescribeTable("Test capiCreateAddresses",
		func(tc testCaseCapiCreateAddresses) {
			objects := []client.Object{}
			for _, address := range tc.ipAddresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := ipPoolMgr.capiCreateAddress(context.TODO(), tc.ipAddressClaim,
				tc.addresses,
			)
			if tc.expectRequeue || tc.expectError {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(ReconcileError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(ReconcileError{}))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			// get list of IPAddress objects
			addressObjects := capipamv1.IPAddressList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(tc.expectedIPAddresses)).To(Equal(len(addressObjects.Items)))
			// Iterate over the IPAddress objects to find all indexes and objects
			for _, address := range addressObjects.Items {
				Expect(tc.expectedIPAddresses).To(ContainElement(address.Name))
				// TODO add further testing later
			}

			Expect(allocatedMap).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
		},
		Entry("Already exists", testCaseCapiCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{
						"abc": "foo-0",
					},
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": "foo-0",
			},
		}),
		Entry("Not allocated yet, pre-allocated", testCaseCapiCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"abc": "192.168.0.15",
					},
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": ipamv1.IPAddressStr("192.168.0.15"),
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.15"): "abc",
			},
			expectedIPAddresses: []string{"abcpref-192-168-0-15"},
		}),
		Entry("Not allocated yet", testCaseCapiCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.11": "bcd",
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{
				"abc": "192.168.0.12",
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.12": "abc",
				"192.168.0.11": "bcd",
			},
			expectedIPAddresses: []string{"abcpref-192-168-0-12"},
		}),
		Entry("Not allocated yet, conflict", testCaseCapiCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			ipAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "abcpref-192-168-0-11",
						Namespace: "myns",
					},
					Spec: capipamv1.IPAddressSpec{
						Address: "192.168.0.11",
						PoolRef: *typedtestObjectReference,
						ClaimRef: corev1.LocalObjectReference{
							Name: "bcd",
						},
					},
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
			expectedIPAddresses: []string{"abcpref-192-168-0-11"},
			expectRequeue:       true,
		}),
		Entry("Not allocated yet, exhausted pool", testCaseCapiCreateAddresses{
			ipPool: &ipamv1.IPPool{
				ObjectMeta: ipPoolMeta,
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
						},
					},
					NamePrefix: "abcpref",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.11": "bcd",
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
			expectedAddresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.11": "bcd",
			},
			expectedIPAddresses: []string{},
			expectError:         true,
		}),
	)

	type testCaseAllocateAddress struct {
		ipPool             *ipamv1.IPPool
		ipClaim            *ipamv1.IPClaim
		addresses          map[ipamv1.IPAddressStr]string
		expectedAddress    ipamv1.IPAddressStr
		expectedPrefix     int
		expectedGateway    *ipamv1.IPAddressStr
		expectedDNSServers []ipamv1.IPAddressStr
		expectError        bool
	}

	DescribeTable("Test AllocateAddress",
		func(tc testCaseAllocateAddress) {
			ipPoolMgr, err := NewIPPoolManager(nil, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			allocatedAddress, prefix, gateway, dnsServers, err := ipPoolMgr.allocateAddress(
				tc.ipClaim, tc.addresses,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(allocatedAddress).To(Equal(tc.expectedAddress))
			Expect(prefix).To(Equal(tc.expectedPrefix))
			Expect(*gateway).To(Equal(*tc.expectedGateway))
			Expect(dnsServers).To(Equal(tc.expectedDNSServers))
		},
		Entry("Empty pools", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectError: true,
		}),
		Entry("One pool, pre-allocated", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
							Prefix:  26,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.21")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.30")),
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.21"),
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
					DNSServers: []ipamv1.IPAddressStr{
						ipamv1.IPAddressStr("8.8.4.4"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.0.21"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.4.4"),
			},
			expectedPrefix: 24,
		}),
		Entry("One pool, pre-allocated, with overrides", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
							Prefix:  26,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.15"),
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
					DNSServers: []ipamv1.IPAddressStr{
						ipamv1.IPAddressStr("8.8.4.4"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.0.15"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.8.8"),
			},
			expectedPrefix: 26,
		}),
		Entry("One pool, pre-allocated, out of bonds", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
							Prefix:  26,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.21"),
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
					DNSServers: []ipamv1.IPAddressStr{
						ipamv1.IPAddressStr("8.8.4.4"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectError: true,
		}),
		Entry("One pool, with start and existing address", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.12"): "bcde",
				ipamv1.IPAddressStr("192.168.0.11"): "abcd",
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.0.13"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
			expectedPrefix:  24,
		}),
		Entry("One pool, with subnet and override prefix", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
					DNSServers: []ipamv1.IPAddressStr{
						ipamv1.IPAddressStr("8.8.4.4"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.12"): "bcde",
				ipamv1.IPAddressStr("192.168.0.11"): "abcd",
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.0.13"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.8.8"),
			},
			expectedPrefix: 24,
		}),
		Entry("two pools, with subnet and override prefix in first", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
						{
							Subnet: (*ipamv1.IPSubnetStr)(ptr.To("192.168.1.10/24")),
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.2.1")),
					DNSServers: []ipamv1.IPAddressStr{
						ipamv1.IPAddressStr("8.8.4.4"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.1.11"): "bcde",
				ipamv1.IPAddressStr("192.168.0.10"): "abcd",
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.1.12"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.2.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.4.4"),
			},
			expectedPrefix: 26,
		}),
		Entry("two pools, with subnet and override prefix", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
						},
						{
							Subnet:  (*ipamv1.IPSubnetStr)(ptr.To("192.168.1.10/24")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.2.1")),
					DNSServers: []ipamv1.IPAddressStr{
						ipamv1.IPAddressStr("8.8.4.4"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.1.11"): "bcde",
				ipamv1.IPAddressStr("192.168.0.10"): "abcd",
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.1.12"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.8.8"),
			},
			expectedPrefix: 24,
		}),
		Entry("Exhausted pools start", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.10"): "abcd",
			},
			expectError: true,
		}),
		Entry("Exhausted pools subnet", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Subnet: (*ipamv1.IPSubnetStr)(ptr.To("192.168.0.0/30")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.1"): "abcd",
				ipamv1.IPAddressStr("192.168.0.2"): "abcd",
				ipamv1.IPAddressStr("192.168.0.3"): "abcd",
			},
			expectError: true,
		}),
	)

	type testCapiCaseAllocateAddress struct {
		ipPool          *ipamv1.IPPool
		addresses       map[ipamv1.IPAddressStr]string
		ipAddressClaim  *capipamv1.IPAddressClaim
		expectedAddress ipamv1.IPAddressStr
		expectedPrefix  int
		expectedGateway *ipamv1.IPAddressStr
		expectError     bool
	}

	DescribeTable("Test capiAllocateAddress",
		func(tc testCapiCaseAllocateAddress) {
			ipPoolMgr, err := NewIPPoolManager(nil, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())
			allocatedAddress, prefix, gateway, err := ipPoolMgr.capiAllocateAddress(
				tc.ipAddressClaim, tc.addresses,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(allocatedAddress).To(Equal(tc.expectedAddress))
			Expect(prefix).To(Equal(tc.expectedPrefix))
			Expect(*gateway).To(Equal(*tc.expectedGateway))
		},
		Entry("Empty pools", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
			},
			expectError: true,
		}),
		Entry("One pool, pre-allocated", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
							Prefix:  26,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
						},
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.21")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.30")),
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.21"),
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.0.21"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
			expectedPrefix:  24,
		}),
		Entry("One pool, pre-allocated, with overrides", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
							Prefix:  26,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.15"),
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.0.15"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
			expectedPrefix:  26,
		}),
		Entry("One pool, pre-allocated, out of bonds", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
							Prefix:  26,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.21"),
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectError: true,
		}),
		Entry("One pool, with start and existing address", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.12"): "bcde",
				ipamv1.IPAddressStr("192.168.0.11"): "abcd",
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.0.13"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
			expectedPrefix:  24,
		}),
		Entry("One pool, with subnet and override prefix", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.20")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.12"): "bcde",
				ipamv1.IPAddressStr("192.168.0.11"): "abcd",
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.0.13"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
			expectedPrefix:  24,
		}),
		Entry("two pools, with subnet and override prefix in first", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
							End:     (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
						},
						{
							Subnet: (*ipamv1.IPSubnetStr)(ptr.To("192.168.1.10/24")),
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.2.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.1.11"): "bcde",
				ipamv1.IPAddressStr("192.168.0.10"): "abcd",
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.1.12"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.2.1")),
			expectedPrefix:  26,
		}),
		Entry("two pools, with subnet and override prefix", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
						},
						{
							Subnet:  (*ipamv1.IPSubnetStr)(ptr.To("192.168.1.10/24")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.2.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.1.11"): "bcde",
				ipamv1.IPAddressStr("192.168.0.10"): "abcd",
			},
			expectedAddress: ipamv1.IPAddressStr("192.168.1.12"),
			expectedGateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.1.1")),
			expectedPrefix:  24,
		}),
		Entry("Exhausted pools start", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
							End:   (*ipamv1.IPAddressStr)(ptr.To("192.168.0.10")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.10"): "abcd",
			},
			expectError: true,
		}),
		Entry("Exhausted pools subnet", testCapiCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Subnet: (*ipamv1.IPSubnetStr)(ptr.To("192.168.0.0/30")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(ptr.To("192.168.0.1")),
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.1"): "abcd",
				ipamv1.IPAddressStr("192.168.0.2"): "abcd",
				ipamv1.IPAddressStr("192.168.0.3"): "abcd",
			},
			expectError: true,
		}),
	)

	type testCaseDeleteAddresses struct {
		ipPool              *ipamv1.IPPool
		ipClaim             *ipamv1.IPClaim
		m3addresses         []*ipamv1.IPAddress
		addresses           map[ipamv1.IPAddressStr]string
		expectedAddresses   map[ipamv1.IPAddressStr]string
		expectedAllocations map[string]ipamv1.IPAddressStr
		expectError         bool
	}

	DescribeTable("Test DeleteAddresses",
		func(tc testCaseDeleteAddresses) {
			objects := []client.Object{}
			for _, address := range tc.m3addresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := ipPoolMgr.deleteAddress(context.TODO(), tc.ipClaim, tc.addresses)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// get list of IPAddress objects
			addressObjects := ipamv1.IPAddressList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(addressObjects.Items)).To(Equal(0))

			Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(allocatedMap).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
			Expect(len(tc.ipClaim.Finalizers)).To(Equal(0))
		},
		Entry("Empty IPPool", testCaseDeleteAddresses{
			ipPool: &ipamv1.IPPool{},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
		}),
		Entry("No Deletion needed", testCaseDeleteAddresses{
			ipPool: &ipamv1.IPPool{},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{ipamv1.IPAddressStr("192.168.0.1"): "abcd"},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.1"): "abcd",
			},
		}),
		Entry("Deletion needed, not found", testCaseDeleteAddresses{
			ipPool: &ipamv1.IPPool{
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.1"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.1"): "TestRef",
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
		}),
		Entry("Deletion needed", testCaseDeleteAddresses{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					NamePrefix: "abc",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.1"),
					},
				},
			},
			ipClaim: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
					Finalizers: []string{
						ipamv1.IPClaimFinalizer,
					},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				ipamv1.IPAddressStr("192.168.0.1"): "TestRef",
			},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
			m3addresses: []*ipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-192-168-0-1",
					},
				},
			},
		}),
	)

	type testCaseCapiDeleteAddresses struct {
		ipPool              *ipamv1.IPPool
		ipAddressClaim      *capipamv1.IPAddressClaim
		capiAddresses       []*capipamv1.IPAddress
		addresses           map[ipamv1.IPAddressStr]string
		expectedAddresses   map[ipamv1.IPAddressStr]string
		expectedAllocations map[string]ipamv1.IPAddressStr
		expectError         bool
	}

	DescribeTable("Test capiDeleteAddresses",
		func(tc testCaseCapiDeleteAddresses) {
			objects := []client.Object{}
			for _, address := range tc.capiAddresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				logr.Discard(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := ipPoolMgr.capiDeleteAddress(context.TODO(), tc.ipAddressClaim, tc.addresses)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			// get list of IPAddress objects
			addressObjects := capipamv1.IPAddressList{}
			opts := &client.ListOptions{}
			err = c.List(context.TODO(), &addressObjects, opts)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(addressObjects.Items)).To(Equal(0))

			Expect(tc.ipPool.Status.LastUpdated.IsZero()).To(BeFalse())
			Expect(allocatedMap).To(Equal(tc.expectedAddresses))
			Expect(tc.ipPool.Status.Allocations).To(Equal(tc.expectedAllocations))
			Expect(len(tc.ipAddressClaim.Finalizers)).To(Equal(0))
		},
		Entry("Empty IPPool", testCaseCapiDeleteAddresses{
			ipPool: &ipamv1.IPPool{},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
		}),
		Entry("No Deletion needed", testCaseCapiDeleteAddresses{
			ipPool: &ipamv1.IPPool{},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			expectedAddresses: map[ipamv1.IPAddressStr]string{"192.168.0.1": "abcd"},
			addresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.1": "abcd",
			},
		}),
		Entry("Deletion needed, not found", testCaseCapiDeleteAddresses{
			ipPool: &ipamv1.IPPool{
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{
						"TestRef": "192.168.0.1",
					},
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.1": "TestRef",
			},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
		}),
		Entry("Deletion needed", testCaseCapiDeleteAddresses{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					NamePrefix: "abc",
				},
				Status: ipamv1.IPPoolStatus{
					Allocations: map[string]ipamv1.IPAddressStr{
						"TestRef": "192.168.0.1",
					},
				},
			},
			ipAddressClaim: &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "TestRef",
					Finalizers: []string{
						IPAddressClaimFinalizer,
					},
				},
			},
			addresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.1": "TestRef",
			},
			expectedAddresses:   map[ipamv1.IPAddressStr]string{},
			expectedAllocations: map[string]ipamv1.IPAddressStr{},
			capiAddresses: []*capipamv1.IPAddress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-192-168-0-1",
					},
				},
			},
		}),
	)

})
