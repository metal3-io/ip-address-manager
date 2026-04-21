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
	"context"
	"errors"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *IPAddress) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ipamv1.IPAddress{}).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ipam-metal3-io-v1alpha1-ipaddress,mutating=false,failurePolicy=fail,groups=ipam.metal3.io,resources=ipaddresses,versions=v1alpha1,name=validation.ipaddress.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-ipam-metal3-io-v1alpha1-ipaddress,mutating=true,failurePolicy=fail,groups=ipam.metal3.io,resources=ipaddresses,versions=v1alpha1,name=default.ipaddress.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// IPAddress implements a validation and defaulting webhook for IPAddress.
type IPAddress struct{}

var _ admission.Defaulter[*ipamv1.IPAddress] = &IPAddress{}
var _ admission.Validator[*ipamv1.IPAddress] = &IPAddress{}

func (webhook *IPAddress) Default(_ context.Context, _ *ipamv1.IPAddress) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPAddress) ValidateCreate(_ context.Context, ipAddress *ipamv1.IPAddress) (admission.Warnings, error) {
	if ipAddress == nil {
		return nil, apierrors.NewBadRequest("expected a IPAddress but got nil")
	}

	allErrs := field.ErrorList{}
	if ipAddress.Spec.Pool.Name == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool", "name"),
				ipAddress.Spec.Pool.Name,
				"cannot be empty",
			),
		)
	}

	if ipAddress.Spec.Claim.Name == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim", "name"),
				ipAddress.Spec.Claim.Name,
				"cannot be empty",
			),
		)
	}

	if ipAddress.Spec.Address == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "address"),
				ipAddress.Spec.Address,
				"cannot be empty",
			),
		)
	} else if ipamv1.ValidateIPAddress(ipAddress.Spec.Address) != nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "address"),
				ipAddress.Spec.Address,
				"is not a valid IP address",
			),
		)
	}

	// Validate requested IP address if present in annotations (for CAPI claims)
	if requestedIP, ok := ipAddress.ObjectMeta.Annotations["ipAddress"]; ok && requestedIP != "" {
		if ipamv1.ValidateIPAddress(ipamv1.IPAddressStr(requestedIP)) != nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("metadata", "annotations", "ipAddress"),
					requestedIP,
					"is not a valid IP address",
				),
			)
		}
	}
	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(ipamv1.GroupVersion.WithKind("IPAddress").GroupKind(), ipAddress.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPAddress) ValidateUpdate(_ context.Context, oldIPAddress, newIPAddress *ipamv1.IPAddress) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	if oldIPAddress == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if newIPAddress == nil {
		return nil, apierrors.NewBadRequest("expected a IPAddress but got nil")
	}

	if newIPAddress.Spec.Address != oldIPAddress.Spec.Address {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "address"),
				newIPAddress.Spec.Address,
				"cannot be modified",
			),
		)
	}

	if newIPAddress.Spec.Pool.Name != oldIPAddress.Spec.Pool.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool"),
				newIPAddress.Spec.Pool,
				"cannot be modified",
			),
		)
	} else if newIPAddress.Spec.Pool.Namespace != oldIPAddress.Spec.Pool.Namespace {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool"),
				newIPAddress.Spec.Pool,
				"cannot be modified",
			),
		)
	} else if newIPAddress.Spec.Pool.Kind != oldIPAddress.Spec.Pool.Kind {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool"),
				newIPAddress.Spec.Pool,
				"cannot be modified",
			),
		)
	}

	if newIPAddress.Spec.Claim.Name != oldIPAddress.Spec.Claim.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				newIPAddress.Spec.Claim,
				"cannot be modified",
			),
		)
	} else if newIPAddress.Spec.Claim.Namespace != oldIPAddress.Spec.Claim.Namespace {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				newIPAddress.Spec.Claim,
				"cannot be modified",
			),
		)
	} else if newIPAddress.Spec.Claim.Kind != oldIPAddress.Spec.Claim.Kind {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				newIPAddress.Spec.Claim,
				"cannot be modified",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(ipamv1.GroupVersion.WithKind("IPAddress").GroupKind(), newIPAddress.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPAddress) ValidateDelete(_ context.Context, _ *ipamv1.IPAddress) (admission.Warnings, error) {
	return nil, nil
}
