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
	"fmt"
	"net"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
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
)

var _ = Describe("IPPool manager", func() {
	DescribeTable("Test Finalizers",
		func(ipPool *ipamv1.IPPool) {
			ipPoolMgr, err := NewIPPoolManager(nil, ipPool,
				klogr.New(),
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
		cluster     *capi.Cluster
		ipPool      *ipamv1.IPPool
		expectError bool
	}

	DescribeTable("Test SetClusterOwnerRef",
		func(tc testCaseSetClusterOwnerRef) {
			ipPoolMgr, err := NewIPPoolManager(nil, tc.ipPool,
				klogr.New(),
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
			cluster: &capi.Cluster{
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
						metav1.OwnerReference{
							Name: "def",
						},
					},
				},
			},
			cluster: &capi.Cluster{
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
						metav1.OwnerReference{
							Name: "def",
						},
						metav1.OwnerReference{
							Name: "abc-cluster",
						},
					},
				},
			},
			cluster: &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc-cluster",
				},
			},
		}),
	)

	type testGetIndexes struct {
		ipPool              *ipamv1.IPPool
		addresses           []*ipamv1.IPAddress
		expectError         bool
		expectedAddresses   map[ipamv1.IPAddressStr]string
		expectedAllocations map[string]ipamv1.IPAddressStr
	}

	DescribeTable("Test getIndexes",
		func(tc testGetIndexes) {
			objects := []runtime.Object{}
			for _, address := range tc.addresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
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
				&ipamv1.IPAddress{
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
				&ipamv1.IPAddress{
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
				&ipamv1.IPAddress{
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
				&ipamv1.IPAddress{
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
	)

	var ipPoolMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: "myns",
	}

	type testCaseUpdateAddresses struct {
		ipPool                *ipamv1.IPPool
		ipClaims              []*ipamv1.IPClaim
		ipAddresses           []*ipamv1.IPAddress
		expectRequeue         bool
		expectError           bool
		expectedNbAllocations int
		expectedAllocations   map[string]ipamv1.IPAddressStr
	}

	DescribeTable("Test UpdateAddresses",
		func(tc testCaseUpdateAddresses) {
			objects := []runtime.Object{}
			for _, address := range tc.ipAddresses {
				objects = append(objects, address)
			}
			for _, claim := range tc.ipClaims {
				objects = append(objects, claim)
			}
			c := fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			nbAllocations, err := ipPoolMgr.UpdateAddresses(context.TODO())
			if tc.expectRequeue || tc.expectError {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
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
					fmt.Printf("%#v", claim)
					Expect(claim.Status.Address).NotTo(BeNil())
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
				&ipamv1.IPClaim{
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
				},
				&ipamv1.IPClaim{
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
				&ipamv1.IPClaim{
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
				&ipamv1.IPClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "abcf",
						Namespace:         "myns",
						DeletionTimestamp: &timeNow,
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
				&ipamv1.IPAddress{
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
						Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
						Prefix:  24,
					},
				},
				&ipamv1.IPAddress{
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
				&ipamv1.IPAddress{
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
			objects := []runtime.Object{}
			for _, address := range tc.ipAddresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())

			allocatedMap, err := ipPoolMgr.createAddress(context.TODO(), tc.ipClaim,
				tc.addresses,
			)
			if tc.expectRequeue || tc.expectError {
				Expect(err).To(HaveOccurred())
				if tc.expectRequeue {
					Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
				} else {
					Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
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
						ipamv1.Pool{
							Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.20")),
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
						ipamv1.Pool{
							Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.20")),
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
						ipamv1.Pool{
							Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.20")),
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
				&ipamv1.IPAddress{
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
						ipamv1.Pool{
							Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.11")),
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
				klogr.New(),
			)
			Expect(err).NotTo(HaveOccurred())
			allocatedAddress, prefix, gateway, dnsServers, err := ipPoolMgr.allocateAddress(
				tc.ipClaim, tc.addresses,
			)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
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
						ipamv1.Pool{
							Start:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.20")),
							Prefix:  26,
							Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.1.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
					},
					PreAllocations: map[string]ipamv1.IPAddressStr{
						"TestRef": ipamv1.IPAddressStr("192.168.0.15"),
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
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
			expectedGateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.4.4"),
			},
			expectedPrefix: 24,
		}),
		Entry("One pool, with start and existing address", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						ipamv1.Pool{
							Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.11")),
							End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.20")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
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
			expectedGateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
			expectedPrefix:  24,
		}),
		Entry("One pool, with subnet and override prefix", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						ipamv1.Pool{
							Start:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.11")),
							End:     (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.20")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.1.1")),
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
			expectedGateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.8.8"),
			},
			expectedPrefix: 24,
		}),
		Entry("two pools, with subnet and override prefix in first", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						ipamv1.Pool{
							Start:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
							End:     (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.1.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
						ipamv1.Pool{
							Subnet: (*ipamv1.IPSubnetStr)(pointer.StringPtr("192.168.1.10/24")),
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.2.1")),
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
			expectedGateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.2.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.4.4"),
			},
			expectedPrefix: 26,
		}),
		Entry("two pools, with subnet and override prefix", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						ipamv1.Pool{
							Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
							End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
						},
						ipamv1.Pool{
							Subnet:  (*ipamv1.IPSubnetStr)(pointer.StringPtr("192.168.1.10/24")),
							Prefix:  24,
							Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.1.1")),
							DNSServers: []ipamv1.IPAddressStr{
								ipamv1.IPAddressStr("8.8.8.8"),
							},
						},
					},
					Prefix:  26,
					Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.2.1")),
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
			expectedGateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.1.1")),
			expectedDNSServers: []ipamv1.IPAddressStr{
				ipamv1.IPAddressStr("8.8.8.8"),
			},
			expectedPrefix: 24,
		}),
		Entry("Exhausted pools start", testCaseAllocateAddress{
			ipPool: &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						ipamv1.Pool{
							Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
							End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
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
						ipamv1.Pool{
							Subnet: (*ipamv1.IPSubnetStr)(pointer.StringPtr("192.168.0.0/30")),
						},
					},
					Prefix:  24,
					Gateway: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.1")),
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
			objects := []runtime.Object{}
			for _, address := range tc.m3addresses {
				objects = append(objects, address)
			}
			c := fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)
			ipPoolMgr, err := NewIPPoolManager(c, tc.ipPool,
				klogr.New(),
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
				&ipamv1.IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc-192-168-0-1",
					},
				},
			},
		}),
	)

	type testCaseGetIPAddress struct {
		ipAddress   ipamv1.Pool
		index       int
		expectError bool
		expectedIP  ipamv1.IPAddressStr
	}

	DescribeTable("Test getIPAddress",
		func(tc testCaseGetIPAddress) {
			result, err := getIPAddress(tc.ipAddress, tc.index)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tc.expectedIP))
			}
		},
		Entry("Empty Start and Subnet", testCaseGetIPAddress{
			ipAddress:   ipamv1.Pool{},
			index:       1,
			expectError: true,
		}),
		Entry("Start set, no end or subnet", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
			},
			index:      1,
			expectedIP: ipamv1.IPAddressStr("192.168.0.11"),
		}),
		Entry("Start set, end set, subnet unset", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
				End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.100")),
			},
			index:      1,
			expectedIP: ipamv1.IPAddressStr("192.168.0.11"),
		}),
		Entry("Start set, end set, subnet unset, out of bound", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Start: (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
				End:   (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.100")),
			},
			index:       100,
			expectError: true,
		}),
		Entry("Start set, end unset, subnet set", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Start:  (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
				Subnet: (*ipamv1.IPSubnetStr)(pointer.StringPtr("192.168.0.0/24")),
			},
			index:      1,
			expectedIP: ipamv1.IPAddressStr("192.168.0.11"),
		}),
		Entry("Start set, end unset, subnet set, out of bound", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Start:  (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
				Subnet: (*ipamv1.IPSubnetStr)(pointer.StringPtr("192.168.0.0/24")),
			},
			index:       250,
			expectError: true,
		}),
		Entry("Start set, end unset, subnet empty", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Start:  (*ipamv1.IPAddressStr)(pointer.StringPtr("192.168.0.10")),
				Subnet: (*ipamv1.IPSubnetStr)(pointer.StringPtr("")),
			},
			index:       1,
			expectError: true,
		}),
		Entry("subnet empty", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Subnet: (*ipamv1.IPSubnetStr)(pointer.StringPtr("")),
			},
			index:       1,
			expectError: true,
		}),
		Entry("Start unset, end unset, subnet set", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Subnet: (*ipamv1.IPSubnetStr)(pointer.StringPtr("192.168.0.10/24")),
			},
			index:      1,
			expectedIP: ipamv1.IPAddressStr("192.168.0.12"),
		}),
		Entry("Start unset, end unset, subnet set, out of bound", testCaseGetIPAddress{
			ipAddress: ipamv1.Pool{
				Subnet: (*ipamv1.IPSubnetStr)(pointer.StringPtr("192.168.0.10/24")),
			},
			index:       250,
			expectError: true,
		}),
	)

	type testCaseAddOffsetToIP struct {
		ip          string
		endIP       string
		offset      int
		expectedIP  string
		expectError bool
	}

	DescribeTable("Test AddOffsetToIP",
		func(tc testCaseAddOffsetToIP) {
			testIP := net.ParseIP(tc.ip)
			testEndIP := net.ParseIP(tc.endIP)
			expectedIP := net.ParseIP(tc.expectedIP)

			result, err := addOffsetToIP(testIP, testEndIP, tc.offset)
			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedIP))
			}
		},
		Entry("valid IPv4", testCaseAddOffsetToIP{
			ip:         "192.168.0.10",
			endIP:      "192.168.0.200",
			offset:     10,
			expectedIP: "192.168.0.20",
		}),
		Entry("valid IPv4, no end ip", testCaseAddOffsetToIP{
			ip:         "192.168.0.10",
			offset:     1000,
			expectedIP: "192.168.3.242",
		}),
		Entry("Over bound ipv4", testCaseAddOffsetToIP{
			ip:          "192.168.0.10",
			endIP:       "192.168.0.200",
			offset:      1000,
			expectError: true,
		}),
		Entry("error ipv4", testCaseAddOffsetToIP{
			ip:          "255.255.255.250",
			offset:      10,
			expectError: true,
		}),
		Entry("valid IPv6", testCaseAddOffsetToIP{
			ip:         "2001::10",
			endIP:      "2001::fff0",
			offset:     10,
			expectedIP: "2001::1A",
		}),
		Entry("valid IPv6, no end ip", testCaseAddOffsetToIP{
			ip:         "2001::10",
			offset:     10000,
			expectedIP: "2001::2720",
		}),
		Entry("Over bound ipv6", testCaseAddOffsetToIP{
			ip:          "2001::10",
			endIP:       "2001::00f0",
			offset:      10000,
			expectError: true,
		}),
		Entry("error ipv6", testCaseAddOffsetToIP{
			ip:          "FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFF0",
			offset:      100,
			expectError: true,
		}),
	)

})
