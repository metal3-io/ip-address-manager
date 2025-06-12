/*
Copyright 2020 The Kubernetes Authors.

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
	"strings"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	APIGroup = "ipam.metal3.io"
)

const (
	IPAddressClaimFinalizer = "ipam.metal3.io/ipaddressclaim"
	IPAddressFinalizer      = "ipam.metal3.io/ipaddress"
	IPAddressAnnotation     = "ipAddress"
)

// IPPoolManagerInterface is an interface for a IPPoolManager.
type IPPoolManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	SetClusterOwnerRef(*clusterv1.Cluster) error
	UpdateAddresses(context.Context) (int, error)
}

// IPPoolManager is responsible for performing machine reconciliation.
type IPPoolManager struct {
	client client.Client
	IPPool *ipamv1.IPPool
	Log    logr.Logger
}

// NewIPPoolManager returns a new helper for managing a ipPool object.
func NewIPPoolManager(client client.Client, ipPool *ipamv1.IPPool, ipPoolLog logr.Logger) (*IPPoolManager, error) {
	return &IPPoolManager{
		client: client,
		IPPool: ipPool,
		Log:    ipPoolLog,
	}, nil
}

// SetFinalizer sets finalizer.
func (m *IPPoolManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !Contains(m.IPPool.Finalizers, ipamv1.IPPoolFinalizer) {
		m.IPPool.Finalizers = append(m.IPPool.Finalizers,
			ipamv1.IPPoolFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer.
func (m *IPPoolManager) UnsetFinalizer() {
	// Remove the finalizer.
	m.IPPool.Finalizers = Filter(m.IPPool.Finalizers,
		ipamv1.IPPoolFinalizer,
	)
}

func (m *IPPoolManager) SetClusterOwnerRef(cluster *clusterv1.Cluster) error {
	if cluster == nil {
		return errors.New("Missing cluster")
	}
	// Verify that the owner reference is there, if not add it and update object,
	// if error requeue.
	_, err := findOwnerRefFromList(m.IPPool.OwnerReferences,
		cluster.TypeMeta, cluster.ObjectMeta)
	if err != nil {
		if ok := errors.As(err, &errNotFound); !ok {
			return err
		}
		m.IPPool.OwnerReferences, err = setOwnerRefInList(
			m.IPPool.OwnerReferences, false, cluster.TypeMeta,
			cluster.ObjectMeta,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// RecreateStatus recreates the status if empty.
func (m *IPPoolManager) getIndexes(ctx context.Context) (map[ipamv1.IPAddressStr]string, error) {
	m.Log.Info("Fetching IPAddress objects")

	// start from empty maps
	if m.IPPool.Status.Allocations == nil {
		m.IPPool.Status.Allocations = make(map[string]ipamv1.IPAddressStr)
	}
	updatedAllocations := make(map[string]ipamv1.IPAddressStr)

	addresses := make(map[ipamv1.IPAddressStr]string)

	// After addresses map is populated, we consider that there are still addresses in use.
	// However, when IPPool.Spec.PreAllocations is given, it can still hold addresses even
	// though they are already deleted. This consideration should be valid only when
	// IPPool does not have DeletionTimestamp set.
	if m.IPPool.DeletionTimestamp.IsZero() {
		for _, address := range m.IPPool.Spec.PreAllocations {
			addresses[address] = ""
		}
	}

	// get list of IPAddress objects
	addressObjects := ipamv1.IPAddressList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.IPPool.Namespace,
	}

	err := m.client.List(ctx, &addressObjects, opts)
	if err != nil {
		return addresses, err
	}

	// Iterate over the IPAddress objects to find all addresses and objects
	for _, addressObject := range addressObjects.Items {
		// If IPPool does not point to this object, discard
		if addressObject.Spec.Pool.Name == "" {
			continue
		}
		if addressObject.Spec.Pool.Name != m.IPPool.Name {
			continue
		}

		// Get the claim Name, if unset use empty string, to still record the
		// index being used, to avoid conflicts
		claimName := ""
		if addressObject.Spec.Claim.Name != "" {
			claimName = addressObject.Spec.Claim.Name
		}
		updatedAllocations[claimName] = addressObject.Spec.Address
		addresses[addressObject.Spec.Address] = claimName
	}

	// get list of IPAddress objects for cluster.x-k8s.io addresses
	capiAddressObjects := capipamv1.IPAddressList{}
	err = m.client.List(ctx, &capiAddressObjects, opts)
	if err != nil {
		return addresses, err
	}

	// Iterate over the IPAddress objects to find all addresses and objects
	for _, addressObject := range capiAddressObjects.Items {
		// If IPPool does not point to this object, discard
		if addressObject.Spec.PoolRef.Name == "" {
			continue
		}
		if addressObject.Spec.PoolRef.Name != m.IPPool.Name {
			continue
		}

		// Get the claim Name, if unset use empty string, to still record the
		// index being used, to avoid conflicts
		claimName := ""
		if addressObject.Spec.ClaimRef.Name != "" {
			claimName = addressObject.Spec.ClaimRef.Name
		}
		updatedAllocations[claimName] = ipamv1.IPAddressStr(addressObject.Spec.Address)
		addresses[ipamv1.IPAddressStr(addressObject.Spec.Address)] = claimName
	}

	if !reflect.DeepEqual(updatedAllocations, m.IPPool.Status.Allocations) {
		m.IPPool.Status.Allocations = updatedAllocations
		m.updateStatusTimestamp()
	}

	return addresses, nil
}

func (m *IPPoolManager) updateStatusTimestamp() {
	now := metav1.Now()
	m.IPPool.Status.LastUpdated = &now
}

// UpdateAddresses manages the claims and creates or deletes IPAddress accordingly.
// It returns the number of current allocations. Current allocation include
// both capi and metal3 type ipaddress objects.
func (m *IPPoolManager) UpdateAddresses(ctx context.Context) (int, error) {
	_, err := m.m3UpdateAddresses(ctx)
	if err != nil {
		return 0, err
	}
	count, err := m.capiUpdateAddresses(ctx)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateM3Addresses manages the ipclaims.ipam.metal3.io and creates or deletes IPAddress.ipam.metal3.io accordingly.
// It returns the number of current allocations. Current allocation include
// both capi and metal3 type ipaddress objects.
func (m *IPPoolManager) m3UpdateAddresses(ctx context.Context) (int, error) {
	addresses, err := m.getIndexes(ctx)
	if err != nil {
		return 0, err
	}

	// get list of IPClaim objects
	addressClaimObjects := ipamv1.IPClaimList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.IPPool.Namespace,
	}

	err = m.client.List(ctx, &addressClaimObjects, opts)
	if err != nil {
		return 0, err
	}

	// Iterate over the IPClaim objects to find all addresses and objects
	for _, addressClaim := range addressClaimObjects.Items {
		// If IPPool does not point to this object, discard
		if addressClaim.Spec.Pool.Name != m.IPPool.Name {
			continue
		}

		if addressClaim.Status.Address != nil && addressClaim.DeletionTimestamp.IsZero() {
			continue
		}

		if addressClaim.Status.ErrorMessage != nil && addressClaim.DeletionTimestamp.IsZero() {
			continue
		}
		addresses, err = m.updateAddress(ctx, &addressClaim, addresses)
		if err != nil {
			return 0, err
		}
	}
	m.updateStatusTimestamp()
	return len(addresses), nil
}

// UpdateCAPIAddresses manages the ipaddressclaims.ipam.cluster.x-k8s.io and creates or deletes IPAddress.ipam.cluster.x-k8s.io accordingly.
// It returns the number of current allocations.
func (m *IPPoolManager) capiUpdateAddresses(ctx context.Context) (int, error) {
	addresses, err := m.getIndexes(ctx)
	if err != nil {
		return 0, err
	}
	// get list of IPClaim objects
	addressClaimObjects := capipamv1.IPAddressClaimList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.IPPool.Namespace,
	}

	err = m.client.List(ctx, &addressClaimObjects, opts)
	if err != nil {
		return 0, err
	}

	// Iterate over the IPAddressClaim objects to find all addresses and objects
	for _, addressClaim := range addressClaimObjects.Items {
		// If IPPool does not point to this object, discard
		if addressClaim.Spec.PoolRef.Name != m.IPPool.Name {
			continue
		}

		if addressClaim.Status.AddressRef.Name != "" && addressClaim.DeletionTimestamp.IsZero() {
			continue
		}

		if anyErrorInExistingClaim(addressClaim) && addressClaim.DeletionTimestamp.IsZero() {
			continue
		}
		addresses, err = m.capiUpdateAddress(ctx, &addressClaim, addresses)
		if err != nil {
			return 0, err
		}
	}
	m.updateStatusTimestamp()
	return len(addresses), nil
}

// UpdateAddress creates metal3 ipaddress or deletes it. Address can be deleted if it
// doesn't have finalizers or only has the ipamv1.IPClaimFinalizer
// Return a map where IP addresses is key and value is the associated (metal3 and/or capi) claim's name.
func (m *IPPoolManager) updateAddress(ctx context.Context,
	addressClaim *ipamv1.IPClaim, addresses map[ipamv1.IPAddressStr]string,
) (_ map[ipamv1.IPAddressStr]string, rerr error) {
	var err error
	var helper *patch.Helper
	helper, err = patch.NewHelper(addressClaim, m.client)
	if err != nil {
		return addresses, errors.Wrap(err, "failed to init patch helper")
	}
	deleted := false
	// Always patch addressClaim exiting this function so we can persist any changes.
	defer func() {
		if deleted {
			return
		}
		err = helper.Patch(ctx, addressClaim)
		if err != nil {
			m.Log.Error(err, "failed to Patch IPClaim")
			rerr = err
		}
	}()

	addressClaim.Status.ErrorMessage = nil

	if addressClaim.DeletionTimestamp.IsZero() {
		addresses, err = m.createAddress(ctx, addressClaim, addresses)
		if err != nil {
			return addresses, err
		}
	} else {
		// Check if this claim is in use. Does it have any other finalizers than our own?
		// If it is no longer in use, proceed to delete the associated IPAddress
		if len(addressClaim.Finalizers) > 1 ||
			(len(addressClaim.Finalizers) == 1 && !Contains(addressClaim.Finalizers, ipamv1.IPClaimFinalizer)) {
			m.Log.Info("IPClaim is still in use (has other finalizers). Cannot delete IPAddress.",
				"IPClaim", addressClaim.Name, "Finalizers", addressClaim.Finalizers)
			return addresses, nil
		}

		addresses, err = m.deleteAddress(ctx, addressClaim, addresses)
		if err != nil {
			return addresses, err
		}
		deleted = true
	}
	return addresses, nil
}

// capiUpdateAddress creates capi ipaddress or deletes it. Address can be deleted if it
// doesn't have finalizers or only has the ipamv1.IPClaimFinalizer
// Return a map where IP addresses is key and value is the associated (metal3 and/or capi) claim's name.
func (m *IPPoolManager) capiUpdateAddress(ctx context.Context,
	addressClaim *capipamv1.IPAddressClaim, addresses map[ipamv1.IPAddressStr]string,
) (map[ipamv1.IPAddressStr]string, error) {
	var err error
	var helper *patch.Helper
	helper, err = patch.NewHelper(addressClaim, m.client)
	if err != nil {
		return addresses, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch addressClaim exiting this function so we can persist any changes.
	defer func() {
		err = helper.Patch(ctx, addressClaim)
		if err != nil {
			m.Log.Error(err, "failed to Patch IPAddressClaim")
		}
	}()

	conditions := clusterv1.Conditions{}
	conditions = append(conditions, clusterv1.Condition{
		Type:               "ErrorMessage",
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Severity:           "Info",
		Reason:             "ErrorMessage",
		Message:            "",
	})
	addressClaim.SetConditions(conditions)

	if addressClaim.DeletionTimestamp.IsZero() {
		addresses, err = m.capiCreateAddress(ctx, addressClaim, addresses)
		if err != nil {
			return addresses, err
		}
	} else {
		// Check if this claim is in use. Does it have any other finalizers than our own?
		// If it is no longer in use, proceed to delete the associated IPAddress
		if len(addressClaim.Finalizers) > 1 ||
			(len(addressClaim.Finalizers) == 1 && !Contains(addressClaim.Finalizers, IPAddressClaimFinalizer)) {
			m.Log.Info("IPAddressClaim is still in use (has other finalizers). Cannot delete IPAddress.",
				"IPAddressClaim", addressClaim.Name, "Finalizers", addressClaim.Finalizers)
			return addresses, nil
		}

		addresses, err = m.capiDeleteAddress(ctx, addressClaim, addresses)
		if err != nil {
			return addresses, err
		}
	}
	return addresses, nil
}

// allocateAddress gets an (metal3)IpAddress for a (metal3)IPClaim.
// it takes into consideration the possible preallocations.
// Returns an IP address, a prefix, a gateway and a list of DNS servers.
func (m *IPPoolManager) allocateAddress(addressClaim *ipamv1.IPClaim,
	addresses map[ipamv1.IPAddressStr]string,
) (ipamv1.IPAddressStr, int, *ipamv1.IPAddressStr, []ipamv1.IPAddressStr, error) {
	var allocatedAddress ipamv1.IPAddressStr
	var err error

	// Get pre-allocated addresses
	preAllocatedAddress, ipPreAllocated := m.IPPool.Spec.PreAllocations[addressClaim.Name]
	// If the IP is pre-allocated, the default prefix and gateway are used
	prefix := m.IPPool.Spec.Prefix
	gateway := m.IPPool.Spec.Gateway
	dnsServers := m.IPPool.Spec.DNSServers

	ipAllocated := false

	requestedIP := ipamv1.IPAddressStr(addressClaim.ObjectMeta.Annotations[IPAddressAnnotation])
	isRequestedIPAllocated := false

	// Conflict-case, claim is preAllocated but has requested different IP
	if requestedIP != "" && ipPreAllocated && requestedIP != preAllocatedAddress {
		addressClaim.Status.ErrorMessage = ptr.To("PreAllocation and requested ip address are conflicting")
		return "", 0, nil, []ipamv1.IPAddressStr{}, errors.New("PreAllocation and requested ip address are conflicting")
	}

	for _, pool := range m.IPPool.Spec.Pools {
		if ipAllocated {
			break
		}
		index := 0
		for !ipAllocated {
			allocatedAddress, err = ipamv1.GetIPAddress(pool, index)
			if err != nil {
				break
			}
			index++
			// Check if requestedIP is present and matches the current address
			if requestedIP != "" && allocatedAddress != requestedIP {
				continue
			}
			if requestedIP != "" && allocatedAddress == requestedIP {
				isRequestedIPAllocated = true
			}
			// We have a pre-allocated ip, we just need to ensure that it matches the current address
			// if it does not, continue and try the next address
			if ipPreAllocated && allocatedAddress != preAllocatedAddress {
				continue
			}
			// Here the two addresses match, so we continue with that one
			if ipPreAllocated {
				ipAllocated = true
			}
			// If we have a preallocated address, this is useless, otherwise, check if the
			// ip is free
			if _, ok := addresses[allocatedAddress]; !ok && allocatedAddress != "" {
				ipAllocated = true
			}
			if !ipAllocated {
				continue
			}

			if pool.Prefix != 0 {
				prefix = pool.Prefix
			}
			if pool.Gateway != nil {
				gateway = pool.Gateway
			}
			if len(pool.DNSServers) != 0 {
				dnsServers = pool.DNSServers
			}
		}
	}
	// We did not get requestedIp as it did not match with any available IP
	if requestedIP != "" && isRequestedIPAllocated && !ipAllocated {
		addressClaim.Status.ErrorMessage = ptr.To("Requested IP not available")
		return "", 0, nil, []ipamv1.IPAddressStr{}, errors.New("Requested IP not available")
	}
	// We have a preallocated IP but we did not find it in the pools! It means it is
	// misconfigured
	if !ipAllocated && ipPreAllocated {
		addressClaim.Status.ErrorMessage = ptr.To("Pre-allocated IP out of bond")
		return "", 0, nil, []ipamv1.IPAddressStr{}, errors.New("Pre-allocated IP out of bond")
	}
	if !ipAllocated {
		addressClaim.Status.ErrorMessage = ptr.To("Exhausted IP Pools")
		return "", 0, nil, []ipamv1.IPAddressStr{}, errors.New("Exhausted IP Pools")
	}
	return allocatedAddress, prefix, gateway, dnsServers, nil
}

// capiAllocateAddress gets an (capi)IpAddress for a (capi)IPAddressClaim.
// it takes into consideration the possible preallocations.
// Returns an IP address, a prefix and a gateway.
func (m *IPPoolManager) capiAllocateAddress(addressClaim *capipamv1.IPAddressClaim,
	addresses map[ipamv1.IPAddressStr]string,
) (ipamv1.IPAddressStr, int, *ipamv1.IPAddressStr, error) {
	var allocatedAddress ipamv1.IPAddressStr
	var err error

	// Get pre-allocated addresses
	preAllocatedAddress, ipPreAllocated := m.IPPool.Spec.PreAllocations[addressClaim.Name]
	// If the IP is pre-allocated, the default prefix and gateway are used
	prefix := m.IPPool.Spec.Prefix
	gateway := m.IPPool.Spec.Gateway

	ipAllocated := false

	requestedIP := ipamv1.IPAddressStr(addressClaim.ObjectMeta.Annotations[IPAddressAnnotation])
	isRequestedIPAllocated := false

	// Conflict-case, claim is preAllocated but has requested different IP
	if requestedIP != "" && ipPreAllocated && requestedIP != preAllocatedAddress {
		conditions := clusterv1.Conditions{}
		conditions = append(conditions, clusterv1.Condition{
			Type:               "ErrorMessage",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Severity:           "Error",
			Reason:             "ErrorMessage",
			Message:            "PreAllocation and requested ip address are conflicting",
		})
		addressClaim.SetConditions(conditions)
		return "", 0, nil, errors.New("PreAllocation and requested ip address are conflicting")
	}

	for _, pool := range m.IPPool.Spec.Pools {
		if ipAllocated {
			break
		}
		index := 0
		for !ipAllocated {
			allocatedAddress, err = ipamv1.GetIPAddress(pool, index)
			if err != nil {
				break
			}
			index++
			// Check if requestedIP is present and matches the current address
			if requestedIP != "" && allocatedAddress != requestedIP {
				continue
			}
			if requestedIP != "" && allocatedAddress == requestedIP {
				isRequestedIPAllocated = true
			}
			// We have a pre-allocated ip, we just need to ensure that it matches the current address
			// if it does not, continue and try the next address
			if ipPreAllocated && allocatedAddress != preAllocatedAddress {
				continue
			}
			// Here the two addresses match, so we continue with that one
			if ipPreAllocated {
				ipAllocated = true
			}
			// If we have a preallocated address, this is useless, otherwise, check if the
			// ip is free
			if _, ok := addresses[allocatedAddress]; !ok && allocatedAddress != "" {
				ipAllocated = true
			}
			if !ipAllocated {
				continue
			}

			if pool.Prefix != 0 {
				prefix = pool.Prefix
			}
			if pool.Gateway != nil {
				gateway = pool.Gateway
			}
		}
	}
	// We did not get requestedIp as it did not match with any available IP
	if requestedIP != "" && isRequestedIPAllocated && !ipAllocated {
		conditions := clusterv1.Conditions{}
		conditions = append(conditions, clusterv1.Condition{
			Type:               "ErrorMessage",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Severity:           "Error",
			Reason:             "ErrorMessage",
			Message:            "Requested IP not available",
		})
		addressClaim.SetConditions(conditions)
		return "", 0, nil, errors.New("Requested IP not available")
	}
	// We have a preallocated IP but we did not find it in the pools! It means it is
	// misconfigured
	if !ipAllocated && ipPreAllocated {
		conditions := clusterv1.Conditions{}
		conditions = append(conditions, clusterv1.Condition{
			Type:               "ErrorMessage",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Severity:           "Error",
			Reason:             "ErrorMessage",
			Message:            "Pre-allocated IP out of bond",
		})
		addressClaim.SetConditions(conditions)
		return "", 0, nil, errors.New("Pre-allocated IP out of bond")
	}
	if !ipAllocated {
		conditions := clusterv1.Conditions{}
		conditions = append(conditions, clusterv1.Condition{
			Type:               "ErrorMessage",
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Severity:           "Error",
			Reason:             "ErrorMessage",
			Message:            "Exhausted IP Pools",
		})
		addressClaim.SetConditions(conditions)
		return "", 0, nil, errors.New("Exhausted IP Pools")
	}
	return allocatedAddress, prefix, gateway, nil
}

// createAddress creates a (metal3)IPAddress object
// Returns an updated map where IP addresses is key
// and value is the associated (metal3 and/or capi) claim's name.
func (m *IPPoolManager) createAddress(ctx context.Context,
	addressClaim *ipamv1.IPClaim, addresses map[ipamv1.IPAddressStr]string,
) (map[ipamv1.IPAddressStr]string, error) {
	if !Contains(addressClaim.Finalizers, ipamv1.IPClaimFinalizer) {
		addressClaim.Finalizers = append(addressClaim.Finalizers,
			ipamv1.IPClaimFinalizer,
		)
	}

	if allocatedAddress, ok := m.IPPool.Status.Allocations[addressClaim.Name]; ok {
		addressClaim.Status.Address = &corev1.ObjectReference{
			Name:      m.formatAddressName(allocatedAddress),
			Namespace: m.IPPool.Namespace,
		}
		return addresses, nil
	}

	// Get a new index for this machine
	m.Log.Info("Getting address", "Claim", addressClaim.Name)
	// Get a new IP for this owner
	allocatedAddress, prefix, gateway, dnsServers, err := m.allocateAddress(addressClaim, addresses)
	if err != nil {
		return addresses, err
	}

	// Set the index and IPAddress names
	addressName := m.formatAddressName(allocatedAddress)

	m.Log.Info("Address allocated", "Claim", addressClaim.Name, "address", allocatedAddress)

	ownerRefs := addressClaim.OwnerReferences
	ownerRefs = append(ownerRefs,
		metav1.OwnerReference{
			APIVersion: m.IPPool.APIVersion,
			Kind:       m.IPPool.Kind,
			Name:       m.IPPool.Name,
			UID:        m.IPPool.UID,
		},
		metav1.OwnerReference{
			APIVersion: addressClaim.APIVersion,
			Kind:       addressClaim.Kind,
			Name:       addressClaim.Name,
			UID:        addressClaim.UID,
		},
	)

	// Create the IPAddress object, with an Owner ref to the IPClaim,
	// the IPPool, and the IPClaims owners. Also add a finalizer.
	addressObject := &ipamv1.IPAddress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPAddress",
			APIVersion: ipamv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            addressName,
			Namespace:       m.IPPool.Namespace,
			Finalizers:      []string{ipamv1.IPAddressFinalizer},
			OwnerReferences: ownerRefs,
			Labels:          addressClaim.Labels,
		},
		Spec: ipamv1.IPAddressSpec{
			Address: allocatedAddress,
			Pool: corev1.ObjectReference{
				Name:      m.IPPool.Name,
				Namespace: m.IPPool.Namespace,
			},
			Claim: corev1.ObjectReference{
				Name:      addressClaim.Name,
				Namespace: m.IPPool.Namespace,
			},
			Prefix:     prefix,
			Gateway:    gateway,
			DNSServers: dnsServers,
		},
	}

	// Create the IPAddress object. If we get a conflict (that will set
	// Transient error), then requeue to retrigger the reconciliation with
	// the new state
	if err := createObject(ctx, m.client, addressObject); err != nil {
		var reconcileError ReconcileError
		if !errors.As(err, &reconcileError) {
			addressClaim.Status.ErrorMessage = ptr.To("Failed to create associated IPAddress object")
		}
		return addresses, err
	}

	m.IPPool.Status.Allocations[addressClaim.Name] = allocatedAddress
	addresses[allocatedAddress] = addressClaim.Name

	addressClaim.Status.Address = &corev1.ObjectReference{
		Name:      addressName,
		Namespace: m.IPPool.Namespace,
	}

	return addresses, nil
}

// capiCreateAddress creates a (capi)IPAddress object
// Returns an updated map where IP addresses is key
// and value is the associated (metal3 and/or capi) claim's name.
func (m *IPPoolManager) capiCreateAddress(ctx context.Context,
	addressClaim *capipamv1.IPAddressClaim, addresses map[ipamv1.IPAddressStr]string,
) (map[ipamv1.IPAddressStr]string, error) {
	if !Contains(addressClaim.Finalizers, IPAddressClaimFinalizer) {
		addressClaim.Finalizers = append(addressClaim.Finalizers,
			IPAddressClaimFinalizer,
		)
	}

	if allocatedAddress, ok := m.IPPool.Status.Allocations[addressClaim.Name]; ok {
		addressClaim.Status.AddressRef = corev1.LocalObjectReference{
			Name: m.formatAddressName(allocatedAddress),
		}
		return addresses, nil
	}

	// Get a new index for this machine
	m.Log.Info("Getting address", "Claim", addressClaim.Name)
	// Get a new IP for this owner
	allocatedAddress, prefix, gateway, err := m.capiAllocateAddress(addressClaim, addresses)
	if err != nil {
		return addresses, err
	}

	var gatewayStr string
	if gateway != nil {
		gatewayStr = string(*gateway)
	} else {
		gatewayStr = ""
	}

	// Set the index and IPAddress names
	addressName := m.formatAddressName(allocatedAddress)

	m.Log.Info("Address allocated", "Claim", addressClaim.Name, "address", allocatedAddress)

	ownerRefs := addressClaim.OwnerReferences
	ownerRefs = append(ownerRefs,
		metav1.OwnerReference{
			APIVersion: m.IPPool.APIVersion,
			Kind:       m.IPPool.Kind,
			Name:       m.IPPool.Name,
			UID:        m.IPPool.UID,
		},
		metav1.OwnerReference{
			APIVersion: addressClaim.APIVersion,
			Kind:       addressClaim.Kind,
			Name:       addressClaim.Name,
			UID:        addressClaim.UID,
		},
	)

	// Create the IPAddress object, with an Owner ref to the IPAddressClaim,
	// the IPPool, and the IPAddressClaim owners. Also add a finalizer.
	addressObject := &capipamv1.IPAddress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPAddress",
			APIVersion: capipamv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            addressName,
			Namespace:       m.IPPool.Namespace,
			Finalizers:      []string{IPAddressFinalizer},
			OwnerReferences: ownerRefs,
			Labels:          addressClaim.Labels,
		},
		Spec: capipamv1.IPAddressSpec{
			Address: string(allocatedAddress),
			PoolRef: corev1.TypedLocalObjectReference{
				Name:     m.IPPool.Name,
				Kind:     m.IPPool.Kind,
				APIGroup: &APIGroup,
			},
			ClaimRef: corev1.LocalObjectReference{
				Name: addressClaim.Name,
			},
			Prefix:  prefix,
			Gateway: gatewayStr,
		},
	}

	// Create the IPAddress object. If we get a conflict (that will set
	// Transient error), then requeue to retrigger the reconciliation with
	// the new state
	if err := createObject(ctx, m.client, addressObject); err != nil {
		var reconcileError ReconcileError
		if !errors.As(err, &reconcileError) {
			conditions := clusterv1.Conditions{}
			conditions = append(conditions, clusterv1.Condition{
				Type:               "ErrorMessage",
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Severity:           "Error",
				Reason:             "ErrorMessage",
				Message:            "Failed to create associated IPAddress object",
			})
			addressClaim.SetConditions(conditions)
		}
		return addresses, err
	}

	m.IPPool.Status.Allocations[addressClaim.Name] = allocatedAddress
	addresses[allocatedAddress] = addressClaim.Name

	addressClaim.Status.AddressRef = corev1.LocalObjectReference{
		Name: addressName,
	}

	return addresses, nil
}

// deleteAddress removes the finalizer from the IPClaim and deletes the associated IPAddress.
func (m *IPPoolManager) deleteAddress(ctx context.Context,
	addressClaim *ipamv1.IPClaim, addresses map[ipamv1.IPAddressStr]string,
) (map[ipamv1.IPAddressStr]string, error) {
	m.Log.Info("Deleting IPAddress associated with IPClaim", "IPClaim", addressClaim.Name)

	allocatedAddress, ok := m.IPPool.Status.Allocations[addressClaim.Name]
	if ok {
		// Try to get the IPAddress. if it succeeds, delete it
		ipAddress := &ipamv1.IPAddress{}
		key := client.ObjectKey{
			Name:      m.formatAddressName(allocatedAddress),
			Namespace: m.IPPool.Namespace,
		}
		err := m.client.Get(ctx, key, ipAddress)
		if err != nil && !apierrors.IsNotFound(err) {
			addressClaim.Status.ErrorMessage = ptr.To("Failed to get associated IPAddress object")
			return addresses, err
		} else if err == nil {
			// Remove the finalizer
			ipAddress.Finalizers = Filter(ipAddress.Finalizers,
				ipamv1.IPAddressFinalizer,
			)
			err = updateObject(ctx, m.client, ipAddress)
			if err != nil && !apierrors.IsNotFound(err) {
				m.Log.Info("Unable to remove finalizer from IPAddress", "IPAddress", ipAddress.Name)
				return addresses, err
			}
			// Delete the IPAddress
			err = deleteObject(ctx, m.client, ipAddress)
			if err != nil {
				addressClaim.Status.ErrorMessage = ptr.To("Failed to delete associated IPAddress object")
				return addresses, err
			}
			m.Log.Info("Deleted IPAddress", "IPAddress", ipAddress.Name)
		}
	}
	addressClaim.Status.Address = nil
	addressClaim.Finalizers = Filter(addressClaim.Finalizers,
		ipamv1.IPClaimFinalizer,
	)
	err := updateObject(ctx, m.client, addressClaim)
	if err != nil && !apierrors.IsNotFound(err) {
		m.Log.Info("Unable to remove finalizer from IPClaim", "IPClaim", addressClaim.Name)
		return addresses, err
	}

	if ok {
		if _, ok := m.IPPool.Spec.PreAllocations[addressClaim.Name]; !ok {
			delete(addresses, allocatedAddress)
		}
		delete(m.IPPool.Status.Allocations, addressClaim.Name)
		m.Log.Info("IPAddressClaim removed from IPPool allocations", "IPAddressClaim", addressClaim.Name)
	}
	m.updateStatusTimestamp()
	return addresses, nil
}

// capideleteAddress removes the finalizer from the IPClaim and deletes the associated IPAddress.
func (m *IPPoolManager) capiDeleteAddress(ctx context.Context,
	addressClaim *capipamv1.IPAddressClaim, addresses map[ipamv1.IPAddressStr]string,
) (map[ipamv1.IPAddressStr]string, error) {
	m.Log.Info("Deleting IPAddress associated with IPAddressClaim", "IPAddressClaim", addressClaim.Name)

	allocatedAddress, ok := m.IPPool.Status.Allocations[addressClaim.Name]
	if ok {
		// Try to get the IPAddress. if it succeeds, delete it
		ipAddress := &capipamv1.IPAddress{}
		key := client.ObjectKey{
			Name:      m.formatAddressName(allocatedAddress),
			Namespace: m.IPPool.Namespace,
		}
		err := m.client.Get(ctx, key, ipAddress)
		if err != nil && !apierrors.IsNotFound(err) {
			conditions := clusterv1.Conditions{}
			conditions = append(conditions, clusterv1.Condition{
				Type:               "ErrorMessage",
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Severity:           "Error",
				Reason:             "ErrorMessage",
				Message:            "Failed to get associated IPAddress object",
			})
			addressClaim.SetConditions(conditions)
			return addresses, err
		} else if err == nil {
			// Remove the finalizer
			ipAddress.Finalizers = Filter(ipAddress.Finalizers,
				IPAddressFinalizer,
			)
			err = updateObject(ctx, m.client, ipAddress)
			if err != nil && !apierrors.IsNotFound(err) {
				m.Log.Info("Unable to remove finalizer from IPAddress", "IPAddress", ipAddress.Name)
				return addresses, err
			}
			// Delete the IPAddress
			err = deleteObject(ctx, m.client, ipAddress)
			if err != nil {
				conditions := clusterv1.Conditions{}
				conditions = append(conditions, clusterv1.Condition{
					Type:               "ErrorMessage",
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Severity:           "Error",
					Reason:             "ErrorMessage",
					Message:            "Failed to delete associated IPAddress object",
				})
				addressClaim.SetConditions(conditions)
				return addresses, err
			}
			m.Log.Info("Deleted IPAddress", "IPAddress", ipAddress.Name)
		}
	}
	addressClaim.Status.AddressRef.Name = ""
	addressClaim.Finalizers = Filter(addressClaim.Finalizers,
		IPAddressClaimFinalizer,
	)
	err := updateObject(ctx, m.client, addressClaim)
	if err != nil && !apierrors.IsNotFound(err) {
		m.Log.Info("Unable to remove finalizer from IPAddressClaim", "IPAddressClaim", addressClaim.Name)
		return addresses, err
	}

	if ok {
		if _, ok := m.IPPool.Spec.PreAllocations[addressClaim.Name]; !ok {
			delete(addresses, allocatedAddress)
		}
		delete(m.IPPool.Status.Allocations, addressClaim.Name)
		m.Log.Info("IPAddressClaim removed from IPPool allocations", "IPAddressClaim", addressClaim.Name)
	}
	m.updateStatusTimestamp()
	return addresses, nil
}

// formatAddressName renders the name of the IPAddress objects.
func (m *IPPoolManager) formatAddressName(address ipamv1.IPAddressStr) string {
	return strings.TrimRight(m.IPPool.Spec.NamePrefix+"-"+strings.Replace(
		strings.Replace(string(address), ":", "-", -1), ".", "-", -1,
	), "-")
}

// check if IPAddressClaim is stamped with an error.
func anyErrorInExistingClaim(addressClaim capipamv1.IPAddressClaim) bool {
	return len(addressClaim.Status.Conditions) > 0 &&
		addressClaim.Status.Conditions[0].Severity == clusterv1.ConditionSeverityError
}
