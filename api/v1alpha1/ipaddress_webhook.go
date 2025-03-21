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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *IPAddress) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(webhook).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ipam-metal3-io-v1alpha1-ipaddress,mutating=false,failurePolicy=fail,groups=ipam.metal3.io,resources=ipaddresses,versions=v1alpha1,name=validation.ipaddress.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-ipam-metal3-io-v1alpha1-ipaddress,mutating=true,failurePolicy=fail,groups=ipam.metal3.io,resources=ipaddresses,versions=v1alpha1,name=default.ipaddress.ipam.metal3.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.CustomDefaulter = &IPAddress{}
var _ webhook.CustomValidator = &IPAddress{}

func (webhook *IPAddress) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPAddress) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*IPAddress)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IPAddress but got a %T", obj))
	}

	allErrs := field.ErrorList{}
	if c.Spec.Pool.Name == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "pool", "name"),
				c.Spec.Pool.Name,
				"cannot be empty",
			),
		)
	}

	if c.Spec.Claim.Name == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim", "name"),
				c.Spec.Claim.Name,
				"cannot be empty",
			),
		)
	}

	if c.Spec.Address == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "address"),
				c.Spec.Address,
				"cannot be empty",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(GroupVersion.WithKind("IPAddress").GroupKind(), c.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPAddress) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	oldIPAddress, ok := oldObj.(*IPAddress)
	if !ok || oldIPAddress == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	newIPAddress, ok := newObj.(*IPAddress)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IPAddress but got a %T", newObj))
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
	return nil, apierrors.NewInvalid(GroupVersion.WithKind("IPAddress").GroupKind(), newIPAddress.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *IPAddress) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
