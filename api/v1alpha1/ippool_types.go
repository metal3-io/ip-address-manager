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

package v1alpha1

import (
	"bytes"
	"fmt"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// IPPoolFinalizer allows IPPoolReconciler to clean up resources
	// associated with IPPool before removing it from the apiserver.
	IPPoolFinalizer = "ippool.ipam.metal3.io"
)

// AllocationStrategy defines the strategy for IP address allocation from a pool.
type AllocationStrategy string

const (
	// AllocationStrategySequential allocates IPs sequentially from pools (default).
	AllocationStrategySequential AllocationStrategy = "sequential"
	// AllocationStrategyRandom allocates IPs randomly from available pool addresses.
	AllocationStrategyRandom AllocationStrategy = "random"
)

// MetaDataIPAddress contains the info to render th ip address. It is IP-version
// agnostic.
type Pool struct {

	// Start is the first ip address that can be rendered
	Start *IPAddressStr `json:"start,omitempty"`

	// End is the last IP address that can be rendered. It is used as a validation
	// that the rendered IP is in bound.
	End *IPAddressStr `json:"end,omitempty"`

	// Subnet is used to validate that the rendered IP is in bounds. In case the
	// Start value is not given, it is derived from the subnet ip incremented by 1
	// (`192.168.0.1` for `192.168.0.0/24`)
	Subnet *IPSubnetStr `json:"subnet,omitempty"`

	// +kubebuilder:validation:Maximum=128
	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the gateway ip address
	Gateway *IPAddressStr `json:"gateway,omitempty"`

	// DNSServers is the list of dns servers
	DNSServers []IPAddressStr `json:"dnsServers,omitempty"`
}

// IPPoolSpec defines the desired state of IPPool.
type IPPoolSpec struct {

	// ClusterName is the name of the Cluster this object belongs to.
	ClusterName *string `json:"clusterName,omitempty"`

	// Pools contains the list of IP addresses pools
	Pools []Pool `json:"pools,omitempty"`

	// +kubebuilder:default=sequential
	// +kubebuilder:validation:Enum=sequential;random
	// AllocationStrategy defines how IP addresses are allocated from the pools.
	// "sequential" (default) allocates the first available IP.
	// "random" allocates a random available IP.
	AllocationStrategy AllocationStrategy `json:"allocationStrategy,omitempty"`

	// PreAllocations contains the preallocated IP addresses
	PreAllocations map[string]IPAddressStr `json:"preAllocations,omitempty"`

	// +kubebuilder:validation:Maximum=128
	// Prefix is the mask of the network as integer (max 128)
	Prefix int `json:"prefix,omitempty"`

	// Gateway is the gateway ip address
	Gateway *IPAddressStr `json:"gateway,omitempty"`

	// DNSServers is the list of dns servers
	DNSServers []IPAddressStr `json:"dnsServers,omitempty"`

	// +kubebuilder:validation:MinLength=1
	// namePrefix is the prefix used to generate the IPAddress object names
	NamePrefix string `json:"namePrefix"`
}

// IPPoolStatus defines the observed state of IPPool.
type IPPoolStatus struct {
	// LastUpdated identifies when this status was last observed.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Allocations contains the map of objects and IP addresses they have
	Allocations map[string]IPAddressStr `json:"indexes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=ippools,scope=Namespaced,categories=cluster-api,shortName=ipp;ippool;m3ipp;m3ippool;m3ippools;metal3ipp;metal3ippool;metal3ippools
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this template belongs"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Metal3IPPool"
// IPPool is the Schema for the ippools API.
type IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPPoolSpec   `json:"spec,omitempty"`
	Status IPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPPoolList contains a list of IPPool.
type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPPool `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &IPPool{}, &IPPoolList{})
}

// ValidateIPAddress validates that the given string is a valid IP address.
func ValidateIPAddress(s IPAddressStr) error {
	if s == "" {
		return nil
	}
	if net.ParseIP(string(s)) == nil {
		return fmt.Errorf("invalid IP address: %q", s)
	}
	return nil
}

// ValidatePool validates a single Pool entry: IP format, CIDR format, and start <= end.
func ValidatePool(pool Pool) []string {
	var errs []string

	if pool.Start != nil {
		if err := ValidateIPAddress(*pool.Start); err != nil {
			errs = append(errs, fmt.Sprintf("start: %v", err))
		}
	}
	if pool.End != nil {
		if err := ValidateIPAddress(*pool.End); err != nil {
			errs = append(errs, fmt.Sprintf("end: %v", err))
		}
	}
	if pool.Subnet != nil {
		if _, _, err := net.ParseCIDR(string(*pool.Subnet)); err != nil {
			errs = append(errs, fmt.Sprintf("subnet: %v", err))
		}
	}
	if pool.Gateway != nil {
		if err := ValidateIPAddress(*pool.Gateway); err != nil {
			errs = append(errs, fmt.Sprintf("gateway: %v", err))
		}
	}
	for _, dns := range pool.DNSServers {
		if err := ValidateIPAddress(dns); err != nil {
			errs = append(errs, fmt.Sprintf("dnsServers: %v", err))
		}
	}

	// Validate start <= end
	if pool.Start != nil && pool.End != nil {
		startIP := net.ParseIP(string(*pool.Start))
		endIP := net.ParseIP(string(*pool.End))
		if startIP != nil && endIP != nil {
			if bytes.Compare(startIP, endIP) > 0 {
				errs = append(errs, fmt.Sprintf("start %s is greater than end %s", *pool.Start, *pool.End))
			}
		}
	}

	return errs
}
