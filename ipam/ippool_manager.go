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
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IPPoolManagerInterface is an interface for a IPPoolManager
type IPPoolManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	SetClusterOwnerRef(*capi.Cluster) error
	UpdateAddresses(context.Context) (int, error)
}

// IPPoolManager is responsible for performing machine reconciliation
type IPPoolManager struct {
	client client.Client
	IPPool *ipamv1.IPPool
	Log    logr.Logger
}

// NewIPPoolManager returns a new helper for managing a ipPool object
func NewIPPoolManager(client client.Client,
	ipPool *ipamv1.IPPool, ipPoolLog logr.Logger) (*IPPoolManager, error) {

	return &IPPoolManager{
		client: client,
		IPPool: ipPool,
		Log:    ipPoolLog,
	}, nil
}

// SetFinalizer sets finalizer
func (m *IPPoolManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !Contains(m.IPPool.Finalizers, ipamv1.IPPoolFinalizer) {
		m.IPPool.Finalizers = append(m.IPPool.Finalizers,
			ipamv1.IPPoolFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer
func (m *IPPoolManager) UnsetFinalizer() {
	// Remove the finalizer.
	m.IPPool.Finalizers = Filter(m.IPPool.Finalizers,
		ipamv1.IPPoolFinalizer,
	)
}

func (m *IPPoolManager) SetClusterOwnerRef(cluster *capi.Cluster) error {
	if cluster == nil {
		return errors.New("Missing cluster")
	}
	// Verify that the owner reference is there, if not add it and update object,
	// if error requeue.
	_, err := findOwnerRefFromList(m.IPPool.OwnerReferences,
		cluster.TypeMeta, cluster.ObjectMeta)
	if err != nil {
		if _, ok := err.(*NotFoundError); !ok {
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

// RecreateStatus recreates the status if empty
func (m *IPPoolManager) getIndexes(ctx context.Context) (map[ipamv1.IPAddressStr]string, error) {

	m.Log.Info("Fetching IPAddress objects")

	//start from empty maps
	m.IPPool.Status.Allocations = make(map[string]ipamv1.IPAddressStr)

	addresses := make(map[ipamv1.IPAddressStr]string)

	for _, address := range m.IPPool.Spec.PreAllocations {
		addresses[address] = ""
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
		m.IPPool.Status.Allocations[claimName] = addressObject.Spec.Address
		addresses[addressObject.Spec.Address] = claimName
	}
	m.updateStatusTimestamp()
	return addresses, nil
}

func (m *IPPoolManager) updateStatusTimestamp() {
	now := metav1.Now()
	m.IPPool.Status.LastUpdated = &now
}

// UpdateAddresses manages the claims and creates or deletes IPAddress accordingly.
// It returns the number of current allocations
func (m *IPPoolManager) UpdateAddresses(ctx context.Context) (int, error) {

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
		addresses, err = m.updateAddress(ctx, &addressClaim, addresses)
		if err != nil {
			return 0, err
		}
	}
	m.updateStatusTimestamp()
	return len(addresses), nil
}

func (m *IPPoolManager) updateAddress(ctx context.Context,
	addressClaim *ipamv1.IPClaim, addresses map[ipamv1.IPAddressStr]string,
) (map[ipamv1.IPAddressStr]string, error) {
	helper, err := patch.NewHelper(addressClaim, m.client)
	if err != nil {
		return addresses, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch addressClaim exiting this function so we can persist any changes.
	defer func() {
		err := helper.Patch(ctx, addressClaim)
		if err != nil {
			m.Log.Info("failed to Patch IPClaim")
		}
	}()

	addressClaim.Status.ErrorMessage = nil

	if addressClaim.DeletionTimestamp.IsZero() {
		addresses, err = m.createAddress(ctx, addressClaim, addresses)
		if err != nil {
			return addresses, err
		}
	} else {
		addresses, err = m.deleteAddress(ctx, addressClaim, addresses)
		if err != nil {
			return addresses, err
		}
	}
	return addresses, nil
}

func (m *IPPoolManager) allocateAddress(addressClaim *ipamv1.IPClaim,
	addresses map[ipamv1.IPAddressStr]string,
) (ipamv1.IPAddressStr, int, *ipamv1.IPAddressStr, error) {
	var err error

	// Get pre-allocated addresses
	allocatedAddress, ipAllocated := m.IPPool.Spec.PreAllocations[addressClaim.Name]
	// If the IP is pre-allocated, the default prefix and gateway are used
	prefix := m.IPPool.Spec.Prefix
	gateway := m.IPPool.Spec.Gateway

	for _, pool := range m.IPPool.Spec.Pools {
		if ipAllocated {
			break
		}
		if pool.Prefix != 0 {
			prefix = pool.Prefix
		}
		if pool.Gateway != nil {
			gateway = pool.Gateway
		}
		index := 0
		for !ipAllocated {
			allocatedAddress, err = getIPAddress(pool, index)
			if err != nil {
				break
			}
			index++
			if _, ok := addresses[allocatedAddress]; !ok && allocatedAddress != "" {
				ipAllocated = true
			}
		}
	}
	if !ipAllocated {
		addressClaim.Status.ErrorMessage = pointer.StringPtr("Exhausted IP Pools")
		return "", 0, nil, errors.New("Exhausted IP Pools")
	}
	return allocatedAddress, prefix, gateway, nil
}

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
	allocatedAddress, prefix, gateway, err := m.allocateAddress(addressClaim, addresses)
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

	// Create the IPAddress object, with an Owner ref to the Metal3Machine
	// (curOwnerRef) and to the IPPool
	addressObject := &ipamv1.IPAddress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPAddress",
			APIVersion: ipamv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            addressName,
			Namespace:       m.IPPool.Namespace,
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
			Prefix:  prefix,
			Gateway: gateway,
		},
	}

	// Create the IPAddress object. If we get a conflict (that will set
	// HasRequeueAfterError), then requeue to retrigger the reconciliation with
	// the new state
	if err := createObject(m.client, ctx, addressObject); err != nil {
		if _, ok := err.(*RequeueAfterError); !ok {
			addressClaim.Status.ErrorMessage = pointer.StringPtr("Failed to create associated IPAddress object")
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

// DeleteDatas deletes old secrets
func (m *IPPoolManager) deleteAddress(ctx context.Context,
	addressClaim *ipamv1.IPClaim, addresses map[ipamv1.IPAddressStr]string,
) (map[ipamv1.IPAddressStr]string, error) {

	m.Log.Info("Deleting Claim", "IPClaim", addressClaim.Name)

	allocatedAddress, ok := m.IPPool.Status.Allocations[addressClaim.Name]
	if ok {
		// Try to get the IPAddress. if it succeeds, delete it
		tmpM3Data := &ipamv1.IPAddress{}
		key := client.ObjectKey{
			Name:      m.formatAddressName(allocatedAddress),
			Namespace: m.IPPool.Namespace,
		}
		err := m.client.Get(ctx, key, tmpM3Data)
		if err != nil && !apierrors.IsNotFound(err) {
			addressClaim.Status.ErrorMessage = pointer.StringPtr("Failed to get associated IPAddress object")
			return addresses, err
		} else if err == nil {
			// Delete the secret with metadata
			err = deleteObject(m.client, ctx, tmpM3Data)
			if err != nil {
				addressClaim.Status.ErrorMessage = pointer.StringPtr("Failed to delete associated IPAddress object")
				return addresses, err
			}
		}

	}
	addressClaim.Status.Address = nil
	addressClaim.Finalizers = Filter(addressClaim.Finalizers,
		ipamv1.IPClaimFinalizer,
	)

	m.Log.Info("Deleted Claim", "IPClaim", addressClaim.Name)

	if ok {
		if _, ok := m.IPPool.Spec.PreAllocations[addressClaim.Name]; !ok {
			delete(addresses, allocatedAddress)
		}
		delete(m.IPPool.Status.Allocations, addressClaim.Name)
	}
	m.updateStatusTimestamp()
	return addresses, nil
}

// getIPAddress renders the IP address, taking the index, offset and step into
// account, it is IP version agnostic
func getIPAddress(entry ipamv1.Pool, index int) (ipamv1.IPAddressStr, error) {

	if entry.Start == nil && entry.Subnet == nil {
		return "", errors.New("Either Start or Subnet is required for ipAddress")
	}
	var ip net.IP
	var err error
	var ipNet *net.IPNet
	offset := index

	// If start is given, use it to add the offset
	if entry.Start != nil {
		var endIP net.IP
		if entry.End != nil {
			endIP = net.ParseIP(string(*entry.End))
		}
		ip, err = addOffsetToIP(net.ParseIP(string(*entry.Start)), endIP, offset)
		if err != nil {
			return "", err
		}

		// Verify that the IP is in the subnet
		if entry.Subnet != nil {
			_, ipNet, err = net.ParseCIDR(string(*entry.Subnet))
			if err != nil {
				return "", err
			}
			if !ipNet.Contains(ip) {
				return "", errors.New("IP address out of bonds")
			}
		}

		// If it is not given, use the CIDR ip address and increment the offset by 1
	} else {
		ip, ipNet, err = net.ParseCIDR(string(*entry.Subnet))
		if err != nil {
			return "", err
		}
		offset++
		ip, err = addOffsetToIP(ip, nil, offset)
		if err != nil {
			return "", err
		}

		// Verify that the ip is in the subnet
		if !ipNet.Contains(ip) {
			return "", errors.New("IP address out of bonds")
		}
	}
	return ipamv1.IPAddressStr(ip.String()), nil
}

// addOffsetToIP computes the value of the IP address with the offset. It is
// IP version agnostic
// Note that if the resulting IP address is in the format ::ffff:xxxx:xxxx then
// ip.String will fail to select the correct type of ip
func addOffsetToIP(ip, endIP net.IP, offset int) (net.IP, error) {
	ip4 := false
	//ip := net.ParseIP(ipString)
	if ip.To4() != nil {
		ip4 = true
	}

	// Create big integers
	IPInt := big.NewInt(0)
	OffsetInt := big.NewInt(int64(offset))

	// Transform the ip into an int. (big endian function)
	IPInt = IPInt.SetBytes(ip)

	// add the two integers
	IPInt = IPInt.Add(IPInt, OffsetInt)

	// return the bytes list
	IPBytes := IPInt.Bytes()

	IPBytesLen := len(IPBytes)

	// Verify that the IPv4 or IPv6 fulfills theirs constraints
	if (ip4 && IPBytesLen > 6 && IPBytes[4] != 255 && IPBytes[5] != 255) ||
		(!ip4 && IPBytesLen > 16) {
		return nil, errors.New(fmt.Sprintf("IP address overflow for : %s", ip.String()))
	}

	//transform the end ip into an Int to compare
	if endIP != nil {
		endIPInt := big.NewInt(0)
		endIPInt = endIPInt.SetBytes(endIP)
		// Computed IP is higher than the end IP
		if IPInt.Cmp(endIPInt) > 0 {
			return nil, errors.New(fmt.Sprintf("IP address out of bonds for : %s", ip.String()))
		}
	}

	// COpy the output back into an ip
	copy(ip[16-IPBytesLen:], IPBytes)
	return ip, nil
}

// formatAddressName renders the name of the IPAddress objects
func (m *IPPoolManager) formatAddressName(address ipamv1.IPAddressStr) string {
	return strings.TrimRight(m.IPPool.Spec.NamePrefix+"-"+strings.Replace(
		strings.Replace(string(address), ":", "-", -1), ".", "-", -1,
	), "-")
}
