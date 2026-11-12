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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These tests run against the API server of the envtest suite, with the
// Cluster API IPAddressClaim and IPAddress CRDs installed. The fake client
// does not validate objects, so it cannot tell whether the conditions written
// to an IPAddressClaim are accepted by the CRD.
var _ = Describe("IPAddressClaim conditions validated by the CRD", func() {
	const (
		namespace = "claim-conditions"
		// The pool that the capiUpdateAddresses test creates. The other tests use
		// claims of a pool that does not exist, which capiUpdateAddresses skips.
		poolName    = "envtest-conditions-pool"
		unknownPool = "envtest-unknown-pool"
	)

	BeforeEach(func() {
		err := k8sClient.Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
		if !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	newClaim := func(name, pool string) *capipamv1.IPAddressClaim {
		return &capipamv1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: capipamv1.IPAddressClaimSpec{
				PoolRef: capipamv1.IPPoolReference{Name: pool, Kind: "IPPool", APIGroup: APIGroup},
			},
		}
	}

	// patchStatus patches the claim like capiUpdateAddress does: Ready is
	// derived from the sub-conditions and all of them are sent by the patch
	// helper, so the API server validates every condition.
	patchStatus := func(claim *capipamv1.IPAddressClaim, update func()) error {
		helper, err := patch.NewHelper(claim, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		update()
		Expect(setIPAddressClaimReadyCondition(claim)).To(Succeed())
		return helper.Patch(context.TODO(), claim,
			patch.WithOwnedConditions{Conditions: ipAddressClaimOwnedConditions})
	}

	It("rejects a condition with an empty reason", func() {
		// Guards the tests below: they only mean something if the CRD is
		// installed and enforces the condition schema.
		claim := newClaim("crd-empty-reason", unknownPool)
		Expect(k8sClient.Create(context.TODO(), claim)).To(Succeed())

		err := patchStatus(claim, func() {
			conditions.Set(claim, metav1.Condition{
				Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionTrue, Reason: "",
			})
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("status.conditions[0].reason: Invalid value"))
		Expect(err.Error()).To(ContainSubstring("should be at least 1 chars long"))
	})

	type testCaseCRD struct {
		update func(claim *capipamv1.IPAddressClaim)
		// Type, Status and Reason of the conditions that must be persisted.
		expected []metav1.Condition
	}

	allocated := func(claim *capipamv1.IPAddressClaim) {
		setIPAddressClaimSubCondition(claim, IPAddressAllocatedCondition, metav1.ConditionTrue,
			IPAddressAllocatedReason, "")
	}

	// One entry per allocation failure, so that every reason is checked
	// against the CRD pattern for condition reasons.
	allocationFailures := []string{
		capipamv1.IPAddressClaimReadyPoolExhaustedReason,
		InvalidRequestedIPReason,
		PreAllocationConflictReason,
		RequestedIPNotAvailableReason,
		RequestedIPOutOfBoundsReason,
		PreAllocationOutOfBoundsReason,
		InvalidPrefixReason,
	}
	failureEntries := make([]TableEntry, 0, len(allocationFailures))
	for _, reason := range allocationFailures {
		readyReason := capipamv1.IPAddressClaimReadyAllocationFailedReason
		if reason == capipamv1.IPAddressClaimReadyPoolExhaustedReason {
			readyReason = capipamv1.IPAddressClaimReadyPoolExhaustedReason
		}
		failureEntries = append(failureEntries, Entry("allocation failed: "+reason, testCaseCRD{
			update: func(claim *capipamv1.IPAddressClaim) {
				setIPAddressAllocationFailed(claim, reason, "allocation failed for "+reason)
			},
			expected: []metav1.Condition{
				{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse, Reason: readyReason},
				{Type: IPAddressAllocatedCondition, Status: metav1.ConditionFalse, Reason: reason},
			},
		}))
	}

	DescribeTable("accepts the conditions written by the IPAddressClaim reconcile",
		func(tc testCaseCRD) {
			claim := newClaim("", unknownPool)
			claim.GenerateName = "crd-claim-"
			Expect(k8sClient.Create(context.TODO(), claim)).To(Succeed())

			Expect(patchStatus(claim, func() { tc.update(claim) })).To(Succeed())

			got := &capipamv1.IPAddressClaim{}
			Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(claim), got)).To(Succeed())
			for _, want := range tc.expected {
				Expect(conditions.Get(got, want.Type)).To(SatisfyAll(
					HaveField("Status", want.Status),
					HaveField("Reason", want.Reason),
				), "condition %s", want.Type)
			}
		},
		Entry("allocated and created", testCaseCRD{
			update: func(claim *capipamv1.IPAddressClaim) {
				allocated(claim)
				setIPAddressClaimSubCondition(claim, IPAddressCreatedCondition, metav1.ConditionTrue,
					IPAddressCreatedReason, "")
			},
			expected: []metav1.Condition{
				{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionTrue, Reason: clusterv1.ReadyReason},
				{Type: IPAddressAllocatedCondition, Status: metav1.ConditionTrue, Reason: IPAddressAllocatedReason},
				{Type: IPAddressCreatedCondition, Status: metav1.ConditionTrue, Reason: IPAddressCreatedReason},
			},
		}),
		Entry("allocated, IPAddress creation failed", testCaseCRD{
			update: func(claim *capipamv1.IPAddressClaim) {
				allocated(claim)
				setIPAddressClaimSubCondition(claim, IPAddressCreatedCondition, metav1.ConditionFalse,
					IPAddressCreateFailedReason, "Failed to create associated IPAddress object")
			},
			expected: []metav1.Condition{
				{
					Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionFalse,
					Reason: capipamv1.IPAddressClaimReadyAllocationFailedReason,
				},
				{Type: IPAddressCreatedCondition, Status: metav1.ConditionFalse, Reason: IPAddressCreateFailedReason},
			},
		}),
		Entry("allocated, IPAddress creation pending", testCaseCRD{
			update: func(claim *capipamv1.IPAddressClaim) {
				allocated(claim)
				setIPAddressClaimSubCondition(claim, IPAddressCreatedCondition, metav1.ConditionUnknown,
					IPAddressCreatePendingReason, "Waiting to create associated IPAddress object")
			},
			expected: []metav1.Condition{
				{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionUnknown, Reason: clusterv1.ReadyUnknownReason},
				{Type: IPAddressCreatedCondition, Status: metav1.ConditionUnknown, Reason: IPAddressCreatePendingReason},
			},
		}),
		Entry("no sub-condition reported yet", testCaseCRD{
			update: func(*capipamv1.IPAddressClaim) {},
			expected: []metav1.Condition{
				{Type: capipamv1.IPAddressClaimReadyCondition, Status: metav1.ConditionUnknown, Reason: clusterv1.ReadyUnknownReason},
			},
		}),
		failureEntries,
	)

	It("persists the conditions set by capiUpdateAddresses", func() {
		start := ipamv1.IPAddressStr("192.168.100.11")
		ipPool := &ipamv1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: poolName, Namespace: namespace},
			Spec: ipamv1.IPPoolSpec{
				NamePrefix: "envtest-prefix",
				Prefix:     24,
				Pools:      []ipamv1.Pool{{Start: &start, End: &start}},
			},
		}
		Expect(k8sClient.Create(context.TODO(), ipPool)).To(Succeed())
		// The controller reads the IPPool through a cached client, which sets
		// the type information that capiCreateAddress copies into the IPAddress.
		// Create does not set it.
		ipPool.TypeMeta = metav1.TypeMeta{APIVersion: ipamv1.GroupVersion.String(), Kind: "IPPool"}

		// Claims are listed by name, so "envtest-claim-a" gets the only address.
		claimA, claimB := newClaim("envtest-claim-a", poolName), newClaim("envtest-claim-b", poolName)
		Expect(k8sClient.Create(context.TODO(), claimA)).To(Succeed())
		Expect(k8sClient.Create(context.TODO(), claimB)).To(Succeed())

		ipPoolMgr, err := NewIPPoolManager(k8sClient, ipPool, logr.Discard())
		Expect(err).NotTo(HaveOccurred())
		_, err = ipPoolMgr.capiUpdateAddresses(context.TODO())
		Expect(err).To(MatchError(ContainSubstring("exhausted IP pools")))

		got := &capipamv1.IPAddressClaim{}
		Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(claimA), got)).To(Succeed())
		Expect(got.Status.AddressRef.Name).To(Equal("envtest-prefix-192-168-100-11"))
		Expect(conditions.Get(got, capipamv1.IPAddressClaimReadyCondition)).To(SatisfyAll(
			HaveField("Status", metav1.ConditionTrue),
			HaveField("Reason", clusterv1.ReadyReason),
		))
		Expect(conditions.IsTrue(got, IPAddressAllocatedCondition)).To(BeTrue())
		Expect(conditions.IsTrue(got, IPAddressCreatedCondition)).To(BeTrue())
		address := &capipamv1.IPAddress{}
		Expect(k8sClient.Get(context.TODO(),
			client.ObjectKey{Name: got.Status.AddressRef.Name, Namespace: namespace}, address)).To(Succeed())
		Expect(address.Spec.ClaimRef.Name).To(Equal(claimA.Name))

		Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(claimB), got)).To(Succeed())
		Expect(got.Status.AddressRef.Name).To(BeEmpty())
		Expect(conditions.Get(got, capipamv1.IPAddressClaimReadyCondition)).To(SatisfyAll(
			HaveField("Status", metav1.ConditionFalse),
			HaveField("Reason", capipamv1.IPAddressClaimReadyPoolExhaustedReason),
		))
		Expect(conditions.GetReason(got, IPAddressAllocatedCondition)).To(
			Equal(capipamv1.IPAddressClaimReadyPoolExhaustedReason))
		Expect(conditions.Has(got, IPAddressCreatedCondition)).To(BeFalse())
	})
})
