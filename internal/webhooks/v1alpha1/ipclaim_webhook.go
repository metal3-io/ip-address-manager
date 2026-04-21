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

func (webhook *IPClaim) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ipamv1.IPClaim{}).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ipam-metal3-io-v1alpha1-ipclaim,mutating=false,failurePolicy=fail,groups=ipam.metal3.io,resources=ipclaims,versions=v1alpha1,name=validation.ipclaim.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-ipam-metal3-io-v1alpha1-ipclaim,mutating=true,failurePolicy=fail,groups=ipam.metal3.io,resources=ipclaims,versions=v1alpha1,name=default.ipclaim.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// IPClaim implements a validation and defaulting webhook for IPClaim.
type IPClaim struct{}

var _ admission.Defaulter[*ipamv1.IPClaim] = &IPClaim{}
var _ admission.Validator[*ipamv1.IPClaim] = &IPClaim{}

func (webhook *IPClaim) Default(_ context.Context, _ *ipamv1.IPClaim) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPClaim) ValidateCreate(_ context.Context, ipClaim *ipamv1.IPClaim) (admission.Warnings, error) {
	if ipClaim == nil {
		return nil, apierrors.NewBadRequest("expected a IPClaim but got nil")
	}

	allErrs := field.ErrorList{}
	if ipClaim.Spec.Pool.Name == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool", "name"),
				ipClaim.Spec.Pool.Name,
				"cannot be empty",
			),
		)
	}

	// Validate requested IP address if present in annotations
	if requestedIP, ok := ipClaim.ObjectMeta.Annotations["ipAddress"]; ok && requestedIP != "" {
		if err := ipamv1.ValidateIPAddress(ipamv1.IPAddressStr(requestedIP)); err != nil {
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
	return nil, apierrors.NewInvalid(ipamv1.GroupVersion.WithKind("IPClaim").GroupKind(), ipClaim.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPClaim) ValidateUpdate(_ context.Context, oldIPClaim, newIPClaim *ipamv1.IPClaim) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	if oldIPClaim == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if newIPClaim == nil {
		return nil, apierrors.NewBadRequest("expected a IPClaim but got nil")
	}

	if newIPClaim.Spec.Pool.Name != oldIPClaim.Spec.Pool.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool"),
				newIPClaim.Spec.Pool,
				"cannot be modified",
			),
		)
	} else if newIPClaim.Spec.Pool.Namespace != oldIPClaim.Spec.Pool.Namespace {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool"),
				newIPClaim.Spec.Pool,
				"cannot be modified",
			),
		)
	} else if newIPClaim.Spec.Pool.Kind != oldIPClaim.Spec.Pool.Kind {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool"),
				newIPClaim.Spec.Pool,
				"cannot be modified",
			),
		)
	}

	// Validate requested IP address if present in annotations
	if requestedIP, ok := newIPClaim.ObjectMeta.Annotations["ipAddress"]; ok && requestedIP != "" {
		if err := ipamv1.ValidateIPAddress(ipamv1.IPAddressStr(requestedIP)); err != nil {
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
	return nil, apierrors.NewInvalid(ipamv1.GroupVersion.WithKind("IPClaim").GroupKind(), newIPClaim.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPClaim) ValidateDelete(_ context.Context, _ *ipamv1.IPClaim) (admission.Warnings, error) {
	return nil, nil
}
