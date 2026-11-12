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
	"errors"
	"time"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
		Entry("requested IP out of bounds is not a pool exhaustion", testCaseReady{
			subConditions: []metav1.Condition{
				{
					Type: IPAddressAllocatedCondition, Status: metav1.ConditionFalse,
					Reason: RequestedIPOutOfBoundsReason, Message: "Requested IP 192.168.2.11 is not in any pool",
				},
			},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedMessage: "* IPAddressAllocated: Requested IP 192.168.2.11 is not in any pool",
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
		Expect(err).To(MatchError(ContainSubstring("exhausted IP pools")))
		var re ReconcileError
		Expect(errors.As(err, &re)).To(BeTrue())
		Expect(re.IsTransient()).To(BeTrue())

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

	It("replaces the Ready condition of a claim written by an older version when the claim is re-linked", func() {
		start := ipamv1.IPAddressStr("192.168.0.11")
		ipPool := &ipamv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: testIPPoolName, Namespace: "myns"},
			Spec: ipamv1.IPPoolSpec{
				NamePrefix: "test-prefix",
				Prefix:     24,
				Pools:      []ipamv1.Pool{{Start: &start, End: &start}},
			},
		}
		// The claim got its address after an exhaustion, but an older version
		// could not write Ready=True, so it kept the stale failure.
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
		address := &capipamv1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-prefix-192-168-0-11", Namespace: "myns",
				Finalizers: []string{IPAddressFinalizer},
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
		Expect(err).NotTo(HaveOccurred())

		got := &capipamv1.IPAddressClaim{}
		Expect(c.Get(context.TODO(), client.ObjectKey{Name: "claim-old", Namespace: "myns"}, got)).To(Succeed())
		ready := conditions.Get(got, capipamv1.IPAddressClaimReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal(clusterv1.ReadyReason))
		Expect(conditions.IsTrue(got, IPAddressAllocatedCondition)).To(BeTrue())
		Expect(conditions.IsTrue(got, IPAddressCreatedCondition)).To(BeTrue())
		Expect(anyErrorInExistingClaim(*got)).To(BeFalse())
	})

	It("does not patch an IPAddressClaim again when its conditions did not change", func() {
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
			}
		}
		// "claim-a" gets the only address in the pool, "claim-b" stays exhausted.
		objects := []client.Object{ipPool, newClaim("claim-a"), newClaim("claim-b")}
		c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).
			WithStatusSubresource(objects...).WithObjects(objects...).Build()
		ipPoolMgr, err := NewIPPoolManager(c, ipPool, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		Expect(err).To(HaveOccurred())
		first := map[string]*capipamv1.IPAddressClaim{"claim-a": {}, "claim-b": {}}
		for name, claim := range first {
			Expect(c.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: "myns"}, claim)).To(Succeed())
		}

		// The exhausted claim is retried, but it ends in the same state: it
		// must not be patched, or the IPAddressClaim watch would enqueue the
		// pool again and defeat the transient requeue delay.
		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		Expect(err).To(HaveOccurred())
		for name, before := range first {
			after := &capipamv1.IPAddressClaim{}
			Expect(c.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: "myns"}, after)).To(Succeed())
			Expect(after.ResourceVersion).To(Equal(before.ResourceVersion), name)
			Expect(after.Status.Conditions).To(Equal(before.Status.Conditions), name)
		}
	})

	It("reports Ready Unknown while a transient IPAddress create error is retried", func() {
		start := ipamv1.IPAddressStr("192.168.0.11")
		ipPool := &ipamv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: testIPPoolName, Namespace: "myns"},
			Spec: ipamv1.IPPoolSpec{
				NamePrefix: "test-prefix",
				Prefix:     24,
				Pools:      []ipamv1.Pool{{Start: &start, End: &start}},
			},
		}
		claim := &capipamv1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "claim-a", Namespace: "myns"},
			Spec: capipamv1.IPAddressClaimSpec{
				PoolRef: capipamv1.IPPoolReference{Name: testIPPoolName, Kind: "IPPool", APIGroup: APIGroup},
			},
		}
		failCreate := true
		objects := []client.Object{ipPool, claim}
		c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).
			WithStatusSubresource(objects...).WithObjects(objects...).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*capipamv1.IPAddress); ok && failCreate {
						return apierrors.NewAlreadyExists(schema.GroupResource{Group: capipamv1.GroupVersion.Group, Resource: "ipaddresses"}, obj.GetName())
					}
					return cl.Create(ctx, obj, opts...)
				},
			}).Build()
		ipPoolMgr, err := NewIPPoolManager(c, ipPool, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		var re ReconcileError
		Expect(errors.As(err, &re)).To(BeTrue())
		Expect(re.IsTransient()).To(BeTrue())

		got := &capipamv1.IPAddressClaim{}
		Expect(c.Get(context.TODO(), client.ObjectKey{Name: "claim-a", Namespace: "myns"}, got)).To(Succeed())
		Expect(got.Status.AddressRef.Name).To(BeEmpty())
		Expect(conditions.IsTrue(got, IPAddressAllocatedCondition)).To(BeTrue())
		Expect(conditions.GetReason(got, IPAddressCreatedCondition)).To(Equal(IPAddressCreatePendingReason))
		ready := conditions.Get(got, capipamv1.IPAddressClaimReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionUnknown))
		Expect(ready.Reason).To(Equal(clusterv1.ReadyUnknownReason))

		// Retried on the next pass, the claim becomes Ready.
		failCreate = false
		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Get(context.TODO(), client.ObjectKey{Name: "claim-a", Namespace: "myns"}, got)).To(Succeed())
		Expect(got.Status.AddressRef.Name).NotTo(BeEmpty())
		Expect(conditions.IsTrue(got, capipamv1.IPAddressClaimReadyCondition)).To(BeTrue())
	})

	type testCaseRequestedIP struct {
		pools          []ipamv1.Pool
		preAllocations map[string]ipamv1.IPAddressStr
		annotation     string
		addresses      map[ipamv1.IPAddressStr]string
		// expectedAddress is set when the allocation succeeds. Otherwise the
		// expected* fields below describe the failure.
		expectedAddress      ipamv1.IPAddressStr
		expectedReason       string
		expectedMessage      string
		expectedReadyReason  string
		expectedRequeueAfter time.Duration
	}

	rangePool := func(start, end string) ipamv1.Pool {
		return ipamv1.Pool{
			Start: (*ipamv1.IPAddressStr)(&start),
			End:   (*ipamv1.IPAddressStr)(&end),
		}
	}
	subnetPool := func(subnet string) ipamv1.Pool {
		return ipamv1.Pool{Subnet: (*ipamv1.IPSubnetStr)(&subnet)}
	}

	DescribeTable("Test the IPAddressAllocated reason of a requested IP",
		func(tc testCaseRequestedIP) {
			ipPool := &ipamv1.IPPool{
				Spec: ipamv1.IPPoolSpec{
					Pools:          tc.pools,
					PreAllocations: tc.preAllocations,
					Prefix:         24,
				},
			}
			claim := &capipamv1.IPAddressClaim{ObjectMeta: metav1.ObjectMeta{Name: "claim-x"}}
			if tc.annotation != "" {
				claim.Annotations = map[string]string{IPAddressAnnotation: tc.annotation}
			}
			ipPoolMgr, err := NewIPPoolManager(nil, ipPool, logr.Discard())
			Expect(err).NotTo(HaveOccurred())

			address, _, _, err := ipPoolMgr.capiAllocateAddress(claim, tc.addresses)
			if tc.expectedReason == "" {
				Expect(err).NotTo(HaveOccurred())
				Expect(address).To(Equal(tc.expectedAddress))
				return
			}
			Expect(err).To(HaveOccurred())

			var re ReconcileError
			Expect(errors.As(err, &re)).To(Equal(tc.expectedRequeueAfter != 0), "transient error expected: %v", err)
			if tc.expectedRequeueAfter != 0 {
				Expect(re.IsTransient()).To(BeTrue())
				Expect(re.GetRequeueAfter()).To(Equal(tc.expectedRequeueAfter))
			}

			allocated := conditions.Get(claim, IPAddressAllocatedCondition)
			Expect(allocated).NotTo(BeNil())
			Expect(allocated.Status).To(Equal(metav1.ConditionFalse))
			Expect(allocated.Reason).To(Equal(tc.expectedReason))
			Expect(allocated.Message).To(Equal(tc.expectedMessage))
			Expect(conditions.Has(claim, IPAddressCreatedCondition)).To(BeFalse())

			Expect(setIPAddressClaimReadyCondition(claim)).To(Succeed())
			ready := conditions.Get(claim, capipamv1.IPAddressClaimReadyCondition)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			Expect(ready.Reason).To(Equal(tc.expectedReadyReason))
			Expect(ready.Message).To(Equal("* IPAddressAllocated: " + tc.expectedMessage))
			// Every failure keeps the claim in the retry set of capiUpdateAddresses.
			Expect(anyErrorInExistingClaim(*claim)).To(BeTrue())
		},
		Entry("requested IP outside the only pool", testCaseRequestedIP{
			pools:                []ipamv1.Pool{rangePool("192.168.0.11", "192.168.0.20")},
			annotation:           "192.168.2.11",
			expectedReason:       RequestedIPOutOfBoundsReason,
			expectedMessage:      "Requested IP 192.168.2.11 is not in any pool",
			expectedReadyReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedRequeueAfter: requeueAfterNoFreeIP,
		}),
		Entry("requested IP between two pools", testCaseRequestedIP{
			pools:                []ipamv1.Pool{rangePool("192.168.0.11", "192.168.0.20"), rangePool("192.168.0.30", "192.168.0.40")},
			annotation:           "192.168.0.25",
			expectedReason:       RequestedIPOutOfBoundsReason,
			expectedMessage:      "Requested IP 192.168.0.25 is not in any pool",
			expectedReadyReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedRequeueAfter: requeueAfterNoFreeIP,
		}),
		Entry("requested IP outside a subnet pool", testCaseRequestedIP{
			pools:                []ipamv1.Pool{subnetPool("192.168.0.0/24")},
			annotation:           "192.168.1.5",
			expectedReason:       RequestedIPOutOfBoundsReason,
			expectedMessage:      "Requested IP 192.168.1.5 is not in any pool",
			expectedReadyReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedRequeueAfter: requeueAfterNoFreeIP,
		}),
		Entry("requested IPv6 outside the pool", testCaseRequestedIP{
			pools:                []ipamv1.Pool{rangePool("2001:db8::10", "2001:db8::20")},
			annotation:           "2001:db8::30",
			expectedReason:       RequestedIPOutOfBoundsReason,
			expectedMessage:      "Requested IP 2001:db8::30 is not in any pool",
			expectedReadyReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedRequeueAfter: requeueAfterNoFreeIP,
		}),
		Entry("requested IP outside the pool, and the pool is also full", testCaseRequestedIP{
			pools:      []ipamv1.Pool{rangePool("192.168.0.11", "192.168.0.12")},
			annotation: "192.168.2.11",
			addresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.11": "claim-a",
				"192.168.0.12": "claim-b",
			},
			expectedReason:       RequestedIPOutOfBoundsReason,
			expectedMessage:      "Requested IP 192.168.2.11 is not in any pool",
			expectedReadyReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedRequeueAfter: requeueAfterNoFreeIP,
		}),
		Entry("requested IP in the pool but already allocated", testCaseRequestedIP{
			pools:                []ipamv1.Pool{rangePool("192.168.0.11", "192.168.0.20")},
			annotation:           "192.168.0.12",
			addresses:            map[ipamv1.IPAddressStr]string{"192.168.0.12": "claim-a"},
			expectedReason:       RequestedIPNotAvailableReason,
			expectedMessage:      "Requested IP not available",
			expectedReadyReason:  capipamv1.IPAddressClaimReadyAllocationFailedReason,
			expectedRequeueAfter: requeueAfterNoFreeIP,
		}),
		Entry("no requested IP and the pool is full", testCaseRequestedIP{
			pools: []ipamv1.Pool{rangePool("192.168.0.11", "192.168.0.12")},
			addresses: map[ipamv1.IPAddressStr]string{
				"192.168.0.11": "claim-a",
				"192.168.0.12": "claim-b",
			},
			expectedReason:       capipamv1.IPAddressClaimReadyPoolExhaustedReason,
			expectedMessage:      "Exhausted IP Pools",
			expectedReadyReason:  capipamv1.IPAddressClaimReadyPoolExhaustedReason,
			expectedRequeueAfter: requeueAfterPoolExhausted,
		}),
		Entry("requested IP in the second pool", testCaseRequestedIP{
			pools:           []ipamv1.Pool{rangePool("192.168.0.11", "192.168.0.20"), rangePool("192.168.0.30", "192.168.0.40")},
			annotation:      "192.168.0.35",
			expectedAddress: "192.168.0.35",
		}),
		Entry("requested IPv6 written in another form than the pool address", testCaseRequestedIP{
			pools:           []ipamv1.Pool{rangePool("2001:db8::10", "2001:db8::20")},
			annotation:      "2001:DB8:0:0:0:0:0:15",
			expectedAddress: "2001:db8::15",
		}),
		Entry("requested IP is the pre-allocated IP and outside the pool", testCaseRequestedIP{
			pools:               []ipamv1.Pool{rangePool("192.168.0.11", "192.168.0.20")},
			preAllocations:      map[string]ipamv1.IPAddressStr{"claim-x": "192.168.2.11"},
			annotation:          "192.168.2.11",
			expectedReason:      PreAllocationOutOfBoundsReason,
			expectedMessage:     "Pre-allocated IP out of bounds",
			expectedReadyReason: capipamv1.IPAddressClaimReadyAllocationFailedReason,
		}),
	)

	It("recovers a claim whose requested IP is outside the pools once the pool includes it", func() {
		first, second := ipamv1.IPAddressStr("192.168.0.11"), ipamv1.IPAddressStr("192.168.2.11")
		ipPool := &ipamv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: testIPPoolName, Namespace: "myns"},
			Spec: ipamv1.IPPoolSpec{
				NamePrefix: "test-prefix",
				Prefix:     24,
				Pools:      []ipamv1.Pool{{Start: &first, End: &first}},
			},
		}
		poolRef := capipamv1.IPPoolReference{Name: testIPPoolName, Kind: "IPPool", APIGroup: APIGroup}
		claim := &capipamv1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "claim-a", Namespace: "myns",
				Annotations: map[string]string{IPAddressAnnotation: string(second)},
			},
			Spec: capipamv1.IPAddressClaimSpec{PoolRef: poolRef},
		}
		// An older version reported this case as PoolExhausted, and set no
		// sub-condition. The claim has no status.addressRef, so it is retried.
		oldClaim := claim.DeepCopy()
		oldClaim.Name = "claim-old"
		oldClaim.Annotations = map[string]string{IPAddressAnnotation: "192.168.9.9"}
		oldClaim.Status.Conditions = []metav1.Condition{{
			Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse,
			Reason: capipamv1.IPAddressClaimReadyPoolExhaustedReason, Message: "Exhausted IP Pools",
			LastTransitionTime: metav1.Now(),
		}}
		objects := []client.Object{ipPool, claim, oldClaim}
		c := fakeclient.NewClientBuilder().WithScheme(setupScheme()).
			WithStatusSubresource(objects...).WithObjects(objects...).Build()
		ipPoolMgr, err := NewIPPoolManager(c, ipPool, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		var re ReconcileError
		Expect(errors.As(err, &re)).To(BeTrue())
		Expect(re.IsTransient()).To(BeTrue())
		Expect(re.GetRequeueAfter()).To(Equal(requeueAfterNoFreeIP))

		for _, name := range []string{"claim-a", "claim-old"} {
			got := &capipamv1.IPAddressClaim{}
			Expect(c.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: "myns"}, got)).To(Succeed())
			Expect(got.Status.AddressRef.Name).To(BeEmpty(), name)
			Expect(conditions.GetReason(got, IPAddressAllocatedCondition)).To(Equal(RequestedIPOutOfBoundsReason), name)
			ready := conditions.Get(got, capipamv1.IPAddressClaimReadyCondition)
			Expect(ready).NotTo(BeNil(), name)
			Expect(ready.Status).To(Equal(metav1.ConditionFalse), name)
			Expect(ready.Reason).To(Equal(capipamv1.IPAddressClaimReadyAllocationFailedReason), name)
			Expect(anyErrorInExistingClaim(*got)).To(BeTrue(), name)
		}

		// The pool is edited to include the requested address. The next
		// reconcile of the pool allocates it, without any change to the claim.
		livePool := &ipamv1.IPPool{}
		Expect(c.Get(context.TODO(), client.ObjectKeyFromObject(ipPool), livePool)).To(Succeed())
		livePool.Spec.Pools = append(livePool.Spec.Pools, ipamv1.Pool{Start: &second, End: &second})
		Expect(c.Update(context.TODO(), livePool)).To(Succeed())
		ipPoolMgr, err = NewIPPoolManager(c, livePool, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		// claim-old still asks for an address that is in no pool.
		Expect(errors.As(err, &re)).To(BeTrue())

		got := &capipamv1.IPAddressClaim{}
		Expect(c.Get(context.TODO(), client.ObjectKey{Name: "claim-a", Namespace: "myns"}, got)).To(Succeed())
		Expect(got.Status.AddressRef.Name).To(Equal("test-prefix-192-168-2-11"))
		Expect(conditions.IsTrue(got, IPAddressAllocatedCondition)).To(BeTrue())
		Expect(conditions.IsTrue(got, IPAddressCreatedCondition)).To(BeTrue())
		Expect(conditions.IsTrue(got, capipamv1.IPAddressClaimReadyCondition)).To(BeTrue())
		Expect(anyErrorInExistingClaim(*got)).To(BeFalse())
	})
})
