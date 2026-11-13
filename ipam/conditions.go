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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
)

// Sub-conditions set on a (capi)IPAddressClaim. The Ready condition
// (capipamv1.IPAddressClaimReadyCondition) is not set directly: it is a
// summary of these sub-conditions, computed by setIPAddressClaimReadyCondition.
const (
	// IPAddressAllocatedCondition is True when an address from the IPPool has
	// been picked for the claim.
	IPAddressAllocatedCondition = "IPAddressAllocated"

	// IPAddressCreatedCondition is True when the IPAddress object for the
	// claim exists and is set in status.addressRef. It is only set once an
	// address has been allocated.
	IPAddressCreatedCondition = "IPAddressCreated"
)

// Reasons used on the IPAddressClaim sub-conditions. The PoolExhausted
// reason is capipamv1.IPAddressClaimReadyPoolExhaustedReason.
const (
	// IPAddressAllocatedReason is used when an address has been allocated.
	IPAddressAllocatedReason = "Allocated"
	// InvalidRequestedIPReason is used when the ipAddress annotation is not a valid IP.
	InvalidRequestedIPReason = "InvalidRequestedIP"
	// PreAllocationConflictReason is used when the requested IP and the pre-allocated IP differ.
	PreAllocationConflictReason = "PreAllocationConflict"
	// RequestedIPNotAvailableReason is used when the requested IP is already in use.
	RequestedIPNotAvailableReason = "RequestedIPNotAvailable"
	// RequestedIPOutOfBoundsReason is used when the requested IP is not in any pool.
	RequestedIPOutOfBoundsReason = "RequestedIPOutOfBounds"
	// PreAllocationOutOfBoundsReason is used when the pre-allocated IP is not in any pool.
	PreAllocationOutOfBoundsReason = "PreAllocationOutOfBounds"
	// InvalidPrefixReason is used when the resolved prefix is out of range.
	InvalidPrefixReason = "InvalidPrefix"

	// IPAddressCreatedReason is used when the IPAddress object exists and is referenced by the claim.
	IPAddressCreatedReason = "Created"
	// IPAddressCreateFailedReason is used when creating the IPAddress object failed.
	IPAddressCreateFailedReason = "CreateFailed"
	// IPAddressCreatePendingReason is used when creating the IPAddress object
	// hit a transient error and will be retried.
	IPAddressCreatePendingReason = "CreatePending"
)

// ipAddressClaimOwnedConditions lists the condition types this controller
// owns on an IPAddressClaim. They are passed to the patch helper.
var ipAddressClaimOwnedConditions = []string{
	capipamv1.IPAddressClaimReadyCondition,
	IPAddressAllocatedCondition,
	IPAddressCreatedCondition,
}

// setIPAddressClaimSubCondition sets one of the IPAddressClaim sub-conditions.
func setIPAddressClaimSubCondition(addressClaim *capipamv1.IPAddressClaim,
	conditionType string, status metav1.ConditionStatus, reason, message string,
) {
	conditions.Set(addressClaim, metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}

// setIPAddressClaimReadyCondition derives the Ready condition of an
// IPAddressClaim from its sub-conditions.
//
// The reasons on Ready follow the Cluster API IPAddressClaim contract: a
// PoolExhausted sub-condition gives Ready the PoolExhausted reason, any other
// failure gives AllocationFailed.
//
// A claim written by an older version has a Ready condition but no
// sub-conditions. Its Ready is kept until a sub-condition is set, so a
// reconcile that returns early does not turn a failed claim into Unknown.
// capiUpdateAddresses skips a claim that has a status.addressRef and no
// error in Ready, so an Unknown Ready would stop it from retrying that claim.
func setIPAddressClaimReadyCondition(addressClaim *capipamv1.IPAddressClaim) error {
	if !conditions.Has(addressClaim, IPAddressAllocatedCondition) &&
		conditions.Has(addressClaim, capipamv1.IPAddressClaimReadyCondition) {
		return nil
	}
	return conditions.SetSummaryCondition(addressClaim, addressClaim, capipamv1.IPAddressClaimReadyCondition,
		conditions.ForConditionTypes{
			IPAddressAllocatedCondition,
			IPAddressCreatedCondition,
		},
		// IPAddressCreated is only set after an address is allocated.
		conditions.IgnoreTypesIfMissing{
			IPAddressCreatedCondition,
		},
		conditions.CustomMergeStrategy{
			MergeStrategy: conditions.DefaultMergeStrategy(
				conditions.ComputeReasonFunc(ipAddressClaimReadyReason),
			),
		},
	)
}

// ipAddressClaimReadyReason picks the reason for the Ready summary condition.
func ipAddressClaimReadyReason(issueConditions, unknownConditions, _ []conditions.ConditionWithOwnerInfo) string {
	switch {
	case len(issueConditions) > 0:
		for _, c := range issueConditions {
			if c.Reason == capipamv1.IPAddressClaimReadyPoolExhaustedReason {
				return capipamv1.IPAddressClaimReadyPoolExhaustedReason
			}
		}
		return capipamv1.IPAddressClaimReadyAllocationFailedReason
	case len(unknownConditions) > 0:
		return clusterv1.ReadyUnknownReason
	default:
		return clusterv1.ReadyReason
	}
}

// setIPAddressAllocationFailed marks the allocation as failed. The
// IPAddressCreated condition is removed, as no IPAddress can exist without an
// allocated address.
func setIPAddressAllocationFailed(addressClaim *capipamv1.IPAddressClaim, reason, message string) {
	setIPAddressClaimSubCondition(addressClaim, IPAddressAllocatedCondition, metav1.ConditionFalse, reason, message)
	conditions.Delete(addressClaim, IPAddressCreatedCondition)
}

// needsConditionBackfill reports whether a bound claim was written by a
// version that did not set the sub-conditions.
//
// capiUpdateAddresses skips a claim that has a status.addressRef and no error
// in Ready, so such a claim would never get the sub-conditions. It is
// reconciled once instead, through the re-link path of capiCreateAddress,
// which sets IPAddressAllocated and IPAddressCreated, and capiUpdateAddress
// then derives Ready. From then on the claim is skipped like any other bound
// claim.
//
// Only a claim that the re-link path can serve unchanged is picked: the pool
// has an allocation for it and status.addressRef is the IPAddress name that
// the re-link computes from that allocation. The pass can therefore only
// re-link: it does not allocate an address, create an IPAddress or change
// status.addressRef. A claim whose IPAddress is gone, or was created under
// another name prefix, is left as before.
func (m *IPPoolManager) needsConditionBackfill(addressClaim *capipamv1.IPAddressClaim) bool {
	allocatedAddress, ok := m.IPPool.Status.Allocations[addressClaim.Name]
	return ok &&
		addressClaim.Status.AddressRef.Name == m.formatAddressName(allocatedAddress) &&
		!conditions.Has(addressClaim, IPAddressAllocatedCondition)
}
