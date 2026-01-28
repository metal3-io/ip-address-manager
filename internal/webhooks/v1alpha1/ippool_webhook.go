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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func validateIP(s ipamv1.IPAddressStr) error {
	if s == "" {
		return nil
	}
	if net.ParseIP(string(s)) == nil {
		return fmt.Errorf("invalid IP address: %q", s)
	}
	return nil
}

func (webhook *IPPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ipamv1.IPPool{}).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ipam-metal3-io-v1alpha1-ippool,mutating=false,failurePolicy=fail,groups=ipam.metal3.io,resources=ippools,versions=v1alpha1,name=validation.ippool.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-ipam-metal3-io-v1alpha1-ippool,mutating=true,failurePolicy=fail,groups=ipam.metal3.io,resources=ippools,versions=v1alpha1,name=default.ippool.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// IPPool implements a validation and defaulting webhook for IPPool.
type IPPool struct{}

var _ webhook.CustomDefaulter = &IPPool{}
var _ webhook.CustomValidator = &IPPool{}

func (webhook *IPPool) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPPool) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m3ipp, ok := obj.(*ipamv1.IPPool)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IPPool but got a %T", obj))
	}

	allErrs := webhook.validatePoolRanges(m3ipp)
	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(ipamv1.GroupVersion.WithKind("IPPool").GroupKind(), m3ipp.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPPool) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	oldM3ipp, ok := oldObj.(*ipamv1.IPPool)
	if !ok || oldM3ipp == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	newM3ipp, ok := newObj.(*ipamv1.IPPool)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IPPool but got a %T", newObj))
	}

	if !reflect.DeepEqual(newM3ipp.Spec.NamePrefix, oldM3ipp.Spec.NamePrefix) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "NamePrefix"),
				newM3ipp.Spec.NamePrefix,
				"cannot be modified",
			),
		)
	}

	// Validate the new pool ranges
	allErrs = append(allErrs, webhook.validatePoolRanges(newM3ipp)...)

	allocationOutOfBonds, inUseOutOfBonds := webhook.checkPoolBonds(oldM3ipp, newM3ipp)
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
	return nil, apierrors.NewInvalid(ipamv1.GroupVersion.WithKind("Metal3Data").GroupKind(), newM3ipp.Name, allErrs)
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
func (webhook *IPPool) validatePoolRanges(pool *ipamv1.IPPool) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate gateway IP if present
	if pool.Spec.Gateway != nil {
		if err := validateIP(*pool.Spec.Gateway); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "gateway"),
					*pool.Spec.Gateway,
					"is not a valid IP address",
				),
			)
		}
	}

	// Validate DNS server IPs
	for i, dnsServer := range pool.Spec.DNSServers {
		if err := validateIP(dnsServer); err != nil {
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
		if err := validateIP(ipAddr); err != nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "preAllocations", name),
					ipAddr,
					"is not a valid IP address",
				),
			)
		}
	}

	for i, pool := range pool.Spec.Pools {
		// Validate Start IP
		if pool.Start != nil {
			if err := validateIP(*pool.Start); err != nil {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "pools").Index(i).Child("start"),
						*pool.Start,
						"is not a valid IP address",
					),
				)
			}
		}

		// Validate End IP
		if pool.End != nil {
			if err := validateIP(*pool.End); err != nil {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "pools").Index(i).Child("end"),
						*pool.End,
						"is not a valid IP address",
					),
				)
			}
		}

		// Validate Subnet CIDR
		if pool.Subnet != nil {
			if _, _, err := net.ParseCIDR(string(*pool.Subnet)); err != nil {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "pools").Index(i).Child("subnet"),
						*pool.Subnet,
						"is not a valid CIDR",
					),
				)
			}
		}

		// Validate pool-specific gateway IP
		if pool.Gateway != nil {
			if err := validateIP(*pool.Gateway); err != nil {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "pools").Index(i).Child("gateway"),
						*pool.Gateway,
						"is not a valid IP address",
					),
				)
			}
		}

		// Validate pool-specific DNS server IPs
		for j, dnsServer := range pool.DNSServers {
			if err := validateIP(dnsServer); err != nil {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "pools").Index(i).Child("dnsServers").Index(j),
						dnsServer,
						"is not a valid IP address",
					),
				)
			}
		}
		// Validate start <= end if both are present and valid
		if pool.Start != nil && pool.End != nil {
			startIP := net.ParseIP(string(*pool.Start))
			endIP := net.ParseIP(string(*pool.End))

			// Only compare if both IPs parsed successfully
			if startIP != nil && endIP != nil {
				if bytes.Compare(startIP, endIP) > 0 {
					allErrs = append(allErrs,
						field.Invalid(
							field.NewPath("spec", "pools").Index(i),
							fmt.Sprintf("start: %s, end: %s", *pool.Start, *pool.End),
							"start address must be less than or equal to end address",
						),
					)
				}
			}
		}
	}
	return allErrs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPPool) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
