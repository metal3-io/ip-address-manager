/*
Copyright 2026 The Metal3 Authors.

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

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("IPAddressClaim conditions", func() {
	type testCaseReady struct {
		subConditions   []metav1.Condition
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}

	DescribeTable("Test setIPAddressClaimReadyCondition",
		func(tc testCaseReady) {
			claim := &capipamv1.IPAddressClaim{}
			for _, c := range tc.subConditions {
				conditions.Set(claim, c)
			}
			Expect(setIPAddressClaimReadyCondition(claim)).To(Succeed())

			ready := conditions.Get(claim, capipamv1.IPAddressClaimReadyCondition)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(tc.expectedStatus))
			Expect(ready.Reason).To(Equal(tc.expectedReason))
			Expect(ready.Message).To(Equal(tc.expectedMessage))
		},
		Entry("allocated and created", testCaseReady{
			subConditions: []metav1.Condition{
				{Type: IPAddressAllocatedCondition, Status: metav1.ConditionTrue, Reason: IPAddressAllocatedReason},
				{Type: IPAddressCreatedCondition, Status: metav1.ConditionTrue, Reason: IPAddressCreatedReason},
			},
			expectedStatus: metav1.ConditionTrue,
			expectedReason: clusterv1.ReadyReason,
		}),
		Entry("pool exhausted", testCaseReady{
			subConditions: []metav1.Condition{
				{
					Type: IPAddressAllocatedCondition, Status: metav1.ConditionFalse,
					Reason: capipamv1.IPAddressClaimReadyPoolExhaustedReason, Message: "Exhausted IP Pools",
				},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  capipamv1.IPAddressClaimReadyPoolExhaustedReason,
			expectedMessage: "* IPAddressAllocated: Exhausted IP Pools",
		}),
		Entry("other allocation failure", testCaseReady{
			subConditions: []metav1.Condition{
				{
					Type: IPAddressAllocatedCondition, Status: metav1.ConditionFalse,
					Reason: InvalidPrefixReason, Message: "Invalid prefix value, it must be between 0 and 128",
				},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedMessage: "* IPAddressAllocated: Invalid prefix value, it must be between 0 and 128",
		}),
		Entry("IPAddress creation failed", testCaseReady{
			subConditions: []metav1.Condition{
				{Type: IPAddressAllocatedCondition, Status: metav1.ConditionTrue, Reason: IPAddressAllocatedReason},
				{
					Type: IPAddressCreatedCondition, Status: metav1.ConditionFalse,
					Reason: IPAddressCreateFailedReason, Message: "Failed to create associated IPAddress object",
				},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedMessage: "* IPAddressCreated: Failed to create associated IPAddress object",
		}),
		Entry("IPAddress creation pending", testCaseReady{
			subConditions: []metav1.Condition{
				{Type: IPAddressAllocatedCondition, Status: metav1.ConditionTrue, Reason: IPAddressAllocatedReason},
				{
					Type: IPAddressCreatedCondition, Status: metav1.ConditionUnknown,
					Reason: IPAddressCreatePendingReason, Message: "Waiting to create associated IPAddress object",
				},
			},
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  clusterv1.ReadyUnknownReason,
			expectedMessage: "* IPAddressCreated: Waiting to create associated IPAddress object",
		}),
		Entry("no sub-condition reported yet", testCaseReady{
			expectedStatus:  metav1.ConditionUnknown,
			expectedReason:  clusterv1.ReadyUnknownReason,
			expectedMessage: "* IPAddressAllocated: Condition not yet reported",
		}),
		Entry("Ready set by an older version is kept until a sub-condition is set", testCaseReady{
			subConditions: []metav1.Condition{
				{
					Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse,
					Reason: capipamv1.IPAddressClaimReadyPoolExhaustedReason, Message: "Exhausted IP Pools",
				},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  capipamv1.IPAddressClaimReadyPoolExhaustedReason,
			expectedMessage: "Exhausted IP Pools",
		}),
		Entry("Ready set by an older version is replaced once sub-conditions are set", testCaseReady{
			subConditions: []metav1.Condition{
				{
					Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse,
					Reason: capipamv1.IPAddressClaimReadyPoolExhaustedReason, Message: "Exhausted IP Pools",
				},
				{Type: IPAddressAllocatedCondition, Status: metav1.ConditionTrue, Reason: IPAddressAllocatedReason},
				{Type: IPAddressCreatedCondition, Status: metav1.ConditionTrue, Reason: IPAddressCreatedReason},
			},
			expectedStatus: metav1.ConditionTrue,
			expectedReason: clusterv1.ReadyReason,
		}),
	)

	DescribeTable("Test anyErrorInExistingClaim",
		func(claimConditions []metav1.Condition, expected bool) {
			claim := capipamv1.IPAddressClaim{
				Status: capipamv1.IPAddressClaimStatus{Conditions: claimConditions},
			}
			Expect(anyErrorInExistingClaim(claim)).To(Equal(expected))
		},
		Entry("no conditions", nil, false),
		Entry("Ready True", []metav1.Condition{
			{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionTrue, Reason: clusterv1.ReadyReason},
		}, false),
		Entry("Ready False with PoolExhausted", []metav1.Condition{
			{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse, Reason: capipamv1.IPAddressClaimReadyPoolExhaustedReason},
		}, true),
		Entry("Ready False with AllocationFailed", []metav1.Condition{
			{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse, Reason: capipamv1.IPAddressClaimReadyAllocationFailedReason},
		}, true),
		Entry("Ready Unknown", []metav1.Condition{
			{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionUnknown, Reason: clusterv1.ReadyUnknownReason},
		}, false),
		Entry("Ready False is not the first condition", []metav1.Condition{
			{Type: clusterv1.PausedCondition, Status: metav1.ConditionFalse, Reason: clusterv1.NotPausedReason},
			{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse, Reason: capipamv1.IPAddressClaimReadyPoolExhaustedReason},
		}, true),
		Entry("failure reason on a condition that is not Ready", []metav1.Condition{
			{Type: IPAddressAllocatedCondition, Status: metav1.ConditionFalse, Reason: capipamv1.IPAddressClaimReadyPoolExhaustedReason},
			{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionTrue, Reason: clusterv1.ReadyReason},
		}, false),
	)

	It("derives Ready from the sub-conditions when reconciling IPAddressClaims", func() {
		start := ipamv1.IPAddressStr("192.168.0.11")
		ipPool := &ipamv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: testIPPoolName, Namespace: "myns"},
			Spec: ipamv1.IPPoolSpec{
				NamePrefix: "test-prefix",
				Prefix:     24,
				Pools:      []ipamv1.Pool{{Start: &start, End: &start}},
			},
		}
		newClaim := func(name string) *capipamv1.IPAddressClaim {
			return &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "myns"},
				Spec: capipamv1.IPAddressClaimSpec{
					PoolRef: capipamv1.IPPoolReference{Name: testIPPoolName, Kind: "IPPool", APIGroup: APIGroup},
				},
				Status: capipamv1.IPAddressClaimStatus{
					// A condition owned by another controller must be kept.
					Conditions: []metav1.Condition{{
						Type: clusterv1.PausedCondition, Status: metav1.ConditionFalse, Reason: clusterv1.NotPausedReason,
						LastTransitionTime: metav1.Now(),
					}},
				},
			}
		}
		// Claims are listed by name, so "claim-a" gets the only address in the pool.
		claimA, claimB := newClaim("claim-a"), newClaim("claim-b")
		objects := []client.Object{ipPool, claimA, claimB}
		c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).
			WithStatusSubresource(objects...).WithObjects(objects...).Build()
		ipPoolMgr, err := NewIPPoolManager(c, ipPool, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		Expect(err).To(MatchError("exhausted IP pools"))

		got := &capipamv1.IPAddressClaim{}
		Expect(c.Get(context.TODO(), client.ObjectKey{Name: "claim-a", Namespace: "myns"}, got)).To(Succeed())
		Expect(got.Status.AddressRef.Name).NotTo(BeEmpty())
		ready := conditions.Get(got, capipamv1.IPAddressClaimReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		// The IPAddressClaim CRD rejects conditions with an empty reason.
		Expect(ready.Reason).To(Equal(clusterv1.ReadyReason))
		Expect(conditions.IsTrue(got, IPAddressAllocatedCondition)).To(BeTrue())
		Expect(conditions.IsTrue(got, IPAddressCreatedCondition)).To(BeTrue())
		Expect(conditions.Has(got, clusterv1.PausedCondition)).To(BeTrue())
		Expect(anyErrorInExistingClaim(*got)).To(BeFalse())

		Expect(c.Get(context.TODO(), client.ObjectKey{Name: "claim-b", Namespace: "myns"}, got)).To(Succeed())
		Expect(got.Status.AddressRef.Name).To(BeEmpty())
		ready = conditions.Get(got, capipamv1.IPAddressClaimReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(capipamv1.IPAddressClaimReadyPoolExhaustedReason))
		Expect(conditions.GetReason(got, IPAddressAllocatedCondition)).To(Equal(capipamv1.IPAddressClaimReadyPoolExhaustedReason))
		Expect(conditions.Has(got, IPAddressCreatedCondition)).To(BeFalse())
		Expect(conditions.Has(got, clusterv1.PausedCondition)).To(BeTrue())
		Expect(anyErrorInExistingClaim(*got)).To(BeTrue())
	})

	It("keeps the Ready condition of a claim written by an older version until a sub-condition is set", func() {
		start := ipamv1.IPAddressStr("192.168.0.11")
		ipPool := &ipamv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: testIPPoolName, Namespace: "myns"},
			Spec: ipamv1.IPPoolSpec{
				NamePrefix: "test-prefix",
				Prefix:     24,
				Pools:      []ipamv1.Pool{{Start: &start, End: &start}},
			},
		}
		// Older versions only set Ready. This claim has a stale failure and
		// its IPAddress is being deleted, so reconcile returns before any
		// sub-condition is set.
		claim := &capipamv1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "claim-old", Namespace: "myns"},
			Spec: capipamv1.IPAddressClaimSpec{
				PoolRef: capipamv1.IPPoolReference{Name: testIPPoolName, Kind: "IPPool", APIGroup: APIGroup},
			},
			Status: capipamv1.IPAddressClaimStatus{
				AddressRef: capipamv1.IPAddressReference{Name: "test-prefix-192-168-0-11"},
				Conditions: []metav1.Condition{{
					Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse,
					Reason: capipamv1.IPAddressClaimReadyPoolExhaustedReason, Message: "Exhausted IP Pools",
					LastTransitionTime: metav1.Now(),
				}},
			},
		}
		deletionTime := metav1.Now()
		address := &capipamv1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-prefix-192-168-0-11", Namespace: "myns",
				Finalizers:        []string{IPAddressFinalizer},
				DeletionTimestamp: &deletionTime,
			},
			Spec: capipamv1.IPAddressSpec{
				Address:  "192.168.0.11",
				PoolRef:  capipamv1.IPPoolReference{Name: testIPPoolName, Kind: "IPPool", APIGroup: APIGroup},
				ClaimRef: capipamv1.IPAddressClaimReference{Name: "claim-old"},
			},
		}
		objects := []client.Object{ipPool, claim, address}
		c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).
			WithStatusSubresource(objects...).WithObjects(objects...).Build()
		ipPoolMgr, err := NewIPPoolManager(c, ipPool, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		Expect(err).To(MatchError(ContainSubstring("is being deleted")))

		got := &capipamv1.IPAddressClaim{}
		Expect(c.Get(context.TODO(), client.ObjectKey{Name: "claim-old", Namespace: "myns"}, got)).To(Succeed())
		// Ready must still report the failure, or the claim would be skipped
		// on the next reconcile and never re-allocated.
		Expect(conditions.GetReason(got, capipamv1.IPAddressClaimReadyCondition)).To(
			Equal(capipamv1.IPAddressClaimReadyPoolExhaustedReason))
		Expect(anyErrorInExistingClaim(*got)).To(BeTrue())
	})
})
