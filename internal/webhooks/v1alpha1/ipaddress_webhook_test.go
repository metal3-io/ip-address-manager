/*
Copyright 2019 The Kubernetes Authors.
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
	"testing"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIPAddressDefault(t *testing.T) {
	g := NewWithT(t)
	webhook := &IPAddress{}

	c := &ipamv1.IPAddress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: ipamv1.IPAddressSpec{},
	}

	g.Expect(webhook.Default(ctx, c)).To(Succeed())
	g.Expect(c.Spec).To(Equal(ipamv1.IPAddressSpec{}))
}

func TestIPAddressCreateValidation(t *testing.T) {
	tests := []struct {
		name        string
		addressName string
		expectErr   bool
		ipPool      corev1.ObjectReference
		address     ipamv1.IPAddressStr
		claim       corev1.ObjectReference
	}{
		{
			name:        "should succeed when values and ipPools correct",
			expectErr:   false,
			addressName: "abc-1",
			ipPool: corev1.ObjectReference{
				Name: "abc",
			},
			claim: corev1.ObjectReference{
				Name: "abc",
			},
			address: "abcd",
		},
		{
			name:        "should fail without address",
			expectErr:   true,
			addressName: "abc-1",
			ipPool: corev1.ObjectReference{
				Name: "abc",
			},
			claim: corev1.ObjectReference{
				Name: "abc",
			},
		},
		{
			name:        "should fail without ipPool name",
			expectErr:   true,
			addressName: "abc-1",
			ipPool: corev1.ObjectReference{
				Namespace: "abc",
			},
			claim: corev1.ObjectReference{
				Name: "abc",
			},
			address: "abcd",
		},
		{
			name:        "should fail without claim name",
			expectErr:   true,
			addressName: "abc-1",
			ipPool: corev1.ObjectReference{
				Name: "abc",
			},
			claim: corev1.ObjectReference{
				Namespace: "abc",
			},
			address: "abcd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &IPAddress{}

			obj := &ipamv1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      tt.addressName,
				},
				Spec: ipamv1.IPAddressSpec{
					Pool:    tt.ipPool,
					Address: tt.address,
					Claim:   tt.claim,
				},
			}

			if tt.expectErr {
				_, err := webhook.ValidateCreate(ctx, obj)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateCreate(ctx, obj)
				g.Expect(err).NotTo(HaveOccurred())
			}
			_, err := webhook.ValidateDelete(ctx, obj)
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestIPAddressUpdateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		newAdd    *ipamv1.IPAddressSpec
		old       *ipamv1.IPAddressSpec
	}{
		{
			name:      "should succeed when values are the same",
			expectErr: false,
			newAdd: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Claim: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Claim: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail with nil old",
			expectErr: true,
			newAdd: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: nil,
		},
		{
			name:      "should fail when index changes",
			expectErr: true,
			newAdd: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcde",
			},
		},
		{
			name:      "should fail when pool name changes",
			expectErr: true,
			newAdd: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Pool Namespace changes",
			expectErr: true,
			newAdd: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
				Address: "abcd",
			},
			old: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Pool kind changes",
			expectErr: true,
			newAdd: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
				Address: "abcd",
			},
			old: &ipamv1.IPAddressSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Claim name changes",
			expectErr: true,
			newAdd: &ipamv1.IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
				},
				Address: "abcd",
			},
			old: &ipamv1.IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Claim Namespace changes",
			expectErr: true,
			newAdd: &ipamv1.IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
				Address: "abcd",
			},
			old: &ipamv1.IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
				Address: "abcd",
			},
		},
		{
			name:      "should fail when Claim kind changes",
			expectErr: true,
			newAdd: &ipamv1.IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
				Address: "abcd",
			},
			old: &ipamv1.IPAddressSpec{
				Claim: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
				Address: "abcd",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newAdd, old *ipamv1.IPAddress
			g := NewWithT(t)
			webhook := &IPAddress{}

			newAdd = &ipamv1.IPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "abc-1",
				},
				Spec: *tt.newAdd,
			}

			if tt.old != nil {
				old = &ipamv1.IPAddress{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
						Name:      "abc-1",
					},
					Spec: *tt.old,
				}
			} else {
				old = nil
			}

			if tt.expectErr {
				_, err := webhook.ValidateUpdate(ctx, old, newAdd)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateUpdate(ctx, old, newAdd)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
