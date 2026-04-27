/*
Copyright 2020 The Kubernetes Authors.
Copyright 2025 The Metal3 Authors.

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

package webhooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"reflect"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *IPPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ipamv1.IPPool{}).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ipam-metal3-io-v1alpha1-ippool,mutating=false,failurePolicy=fail,groups=ipam.metal3.io,resources=ippools,versions=v1alpha1,name=validation.ippool.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-ipam-metal3-io-v1alpha1-ippool,mutating=true,failurePolicy=fail,groups=ipam.metal3.io,resources=ippools,versions=v1alpha1,name=default.ippool.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// IPPool implements a validation and defaulting webhook for IPPool.
type IPPool struct{}

var _ admission.Defaulter[*ipamv1.IPPool] = &IPPool{}
var _ admission.Validator[*ipamv1.IPPool] = &IPPool{}

func (webhook *IPPool) Default(_ context.Context, ipPool *ipamv1.IPPool) error {
	if ipPool.Spec.AllocationStrategy == "" {
		ipPool.Spec.AllocationStrategy = ipamv1.AllocationStrategySequential
	}
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPPool) ValidateCreate(_ context.Context, ipPool *ipamv1.IPPool) (admission.Warnings, error) {
	if ipPool == nil {
		return nil, apierrors.NewBadRequest("expected an IPPool but got nil")
	}

	allErrs := webhook.validatePoolRanges(ipPool)
	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(ipamv1.GroupVersion.WithKind("IPPool").GroupKind(), ipPool.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPPool) ValidateUpdate(_ context.Context, oldIPPool, newIPPool *ipamv1.IPPool) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	if oldIPPool == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if newIPPool == nil {
		return nil, apierrors.NewBadRequest("expected an IPPool but got nil")
	}

	if !reflect.DeepEqual(newIPPool.Spec.NamePrefix, oldIPPool.Spec.NamePrefix) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "namePrefix"),
				newIPPool.Spec.NamePrefix,
				"cannot be modified",
			),
		)
	}

	if oldIPPool.Spec.AllocationStrategy != "" && newIPPool.Spec.AllocationStrategy != oldIPPool.Spec.AllocationStrategy {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "allocationStrategy"),
				newIPPool.Spec.AllocationStrategy,
				"cannot be modified after creation",
			),
		)
	}

	// Validate the new pool ranges
	allErrs = append(allErrs, webhook.validatePoolRanges(newIPPool)...)

	allocationOutOfBonds, inUseOutOfBonds := webhook.checkPoolBonds(oldIPPool, newIPPool)
	if len(allocationOutOfBonds) != 0 {
		for _, address := range allocationOutOfBonds {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "preAllocations"),
					address,
					"is out of bonds of the pools given",
				),
			)
		}
	}
	if len(inUseOutOfBonds) != 0 {
		for _, address := range inUseOutOfBonds {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "pools"),
					address,
					"is in use but out of bonds of the pools given",
				),
			)
		}
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(ipamv1.GroupVersion.WithKind("IPPool").GroupKind(), newIPPool.Name, allErrs)
}

func (webhook *IPPool) checkPoolBonds(oldPool, newPool *ipamv1.IPPool) ([]ipamv1.IPAddressStr, []ipamv1.IPAddressStr) {
	allocationOutOfBonds := []ipamv1.IPAddressStr{}
	inUseOutOfBonds := []ipamv1.IPAddressStr{}
	for _, address := range newPool.Spec.PreAllocations {
		inBonds := webhook.isAddressInBonds(newPool, address)

		if !inBonds {
			allocationOutOfBonds = append(allocationOutOfBonds, address)
		}
	}
	for _, address := range oldPool.Status.Allocations {
		inBonds := webhook.isAddressInBonds(newPool, address)

		if !inBonds {
			inUseOutOfBonds = append(inUseOutOfBonds, address)
		}
	}
	return allocationOutOfBonds, inUseOutOfBonds
}

func (webhook *IPPool) isAddressInBonds(newPool *ipamv1.IPPool, address ipamv1.IPAddressStr) bool {
	ip, err := netip.ParseAddr(string(address))
	if err != nil {
		return false
	}

	for _, pool := range newPool.Spec.Pools {
		if pool.Start != nil {
			startIP, err := netip.ParseAddr(string(*pool.Start))
			if err != nil {
				// skip this invalid pool, as the validation error should be caught somewhere else
				continue
			}
			if startIP.Compare(ip) > 0 {
				continue
			}
		}

		if pool.End != nil {
			endIP, err := netip.ParseAddr(string(*pool.End))
			if err != nil {
				// skip this invalid pool, as the validation error should be caught somewhere else
				continue
			}
			if endIP.Compare(ip) < 0 {
				continue
			}
		}

		if pool.Subnet != nil {
			_, subnet, err := net.ParseCIDR(string(*pool.Subnet))
			if err != nil {
				// skip this invalid pool, as the validation error should be caught somewhere else
				continue
			}
			if !subnet.Contains(net.ParseIP(ip.String())) {
				continue
			}
		}

		return true
	}

	return false
}

// validatePoolRanges validates that start <= end for each pool and validates all IP addresses.
// It delegates per-pool validation to the shared ipamv1.ValidatePool function.
func (webhook *IPPool) validatePoolRanges(pool *ipamv1.IPPool) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate spec-level gateway IP
	if pool.Spec.Gateway != nil {
		if err := ipamv1.ValidateIPAddress(*pool.Spec.Gateway); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "gateway"),
					*pool.Spec.Gateway,
					"is not a valid IP address",
				),
			)
		}
	}

	// Validate spec-level DNS server IPs
	for i, dnsServer := range pool.Spec.DNSServers {
		if err := ipamv1.ValidateIPAddress(dnsServer); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "dnsServers").Index(i),
					dnsServer,
					"is not a valid IP address",
				),
			)
		}
	}

	// Validate preAllocations IPs
	for name, ipAddr := range pool.Spec.PreAllocations {
		if err := ipamv1.ValidateIPAddress(ipAddr); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "preAllocations", name),
					ipAddr,
					"is not a valid IP address",
				),
			)
		}
	}

	// Validate each pool entry with per-field error paths
	for i, p := range pool.Spec.Pools {
		poolPath := field.NewPath("spec", "pools").Index(i)

		if p.Start != nil {
			if err := ipamv1.ValidateIPAddress(*p.Start); err != nil {
				allErrs = append(allErrs, field.Invalid(poolPath.Child("start"), *p.Start, "is not a valid IP address"))
			}
		}
		if p.End != nil {
			if err := ipamv1.ValidateIPAddress(*p.End); err != nil {
				allErrs = append(allErrs, field.Invalid(poolPath.Child("end"), *p.End, "is not a valid IP address"))
			}
		}
		if p.Subnet != nil {
			if _, _, err := net.ParseCIDR(string(*p.Subnet)); err != nil {
				allErrs = append(allErrs, field.Invalid(poolPath.Child("subnet"), *p.Subnet, "is not a valid CIDR"))
			}
		}
		if p.Gateway != nil {
			if err := ipamv1.ValidateIPAddress(*p.Gateway); err != nil {
				allErrs = append(allErrs, field.Invalid(poolPath.Child("gateway"), *p.Gateway, "is not a valid IP address"))
			}
		}
		for j, dnsServer := range p.DNSServers {
			if err := ipamv1.ValidateIPAddress(dnsServer); err != nil {
				allErrs = append(allErrs, field.Invalid(poolPath.Child("dnsServers").Index(j), dnsServer, "is not a valid IP address"))
			}
		}
		if p.Start != nil && p.End != nil {
			startIP := net.ParseIP(string(*p.Start))
			endIP := net.ParseIP(string(*p.End))
			if startIP != nil && endIP != nil {
				if bytes.Compare(startIP, endIP) > 0 {
					allErrs = append(allErrs, field.Invalid(poolPath, fmt.Sprintf("start: %s, end: %s", *p.Start, *p.End), "start address must be less than or equal to end address"))
				}
			}
		}
	}
	return allErrs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPPool) ValidateDelete(_ context.Context, _ *ipamv1.IPPool) (admission.Warnings, error) {
	return nil, nil
}
