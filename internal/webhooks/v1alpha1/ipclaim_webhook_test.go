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

func TestIPClaimDefault(t *testing.T) {
	g := NewWithT(t)
	webhook := &IPClaim{}

	c := &ipamv1.IPClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
	}

	g.Expect(webhook.Default(ctx, c)).To(Succeed())

	g.Expect(c.Spec).To(Equal(ipamv1.IPClaimSpec{}))
	g.Expect(c.Status).To(Equal(ipamv1.IPClaimStatus{}))
}

func TestIPClaimCreateValidation(t *testing.T) {
	tests := []struct {
		name      string
		claimName string
		expectErr bool
		ipPool    corev1.ObjectReference
	}{
		{
			name:      "should succeed when ipPool is correct",
			expectErr: false,
			claimName: "abc-1",
			ipPool: corev1.ObjectReference{
				Name: "abc",
			},
		},
		{
			name:      "should fail without ipPool",
			expectErr: true,
			claimName: "abc-1",
			ipPool:    corev1.ObjectReference{},
		},
		{
			name:      "should fail without ipPool name",
			expectErr: true,
			claimName: "abc-1",
			ipPool: corev1.ObjectReference{
				Namespace: "abc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &IPClaim{}

			obj := &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      tt.claimName,
				},
				Spec: ipamv1.IPClaimSpec{
					Pool: tt.ipPool,
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

func TestIPClaimUpdateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		newClm    *ipamv1.IPClaimSpec
		old       *ipamv1.IPClaimSpec
	}{
		{
			name:      "should succeed when values are the same",
			expectErr: false,
			newClm: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
		},
		{
			name:      "should fail with nil old",
			expectErr: true,
			newClm: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: nil,
		},
		{
			name:      "should fail when pool is unset",
			expectErr: true,
			newClm: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{},
			},
			old: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
		},
		{
			name:      "should fail when pool name changes",
			expectErr: true,
			newClm: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abcd",
				},
			},
		},
		{
			name:      "should fail when Pool Namespace changes",
			expectErr: true,
			newClm: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
			},
			old: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
			},
		},
		{
			name:      "should fail when Pool kind changes",
			expectErr: true,
			newClm: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
			},
			old: &ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newClm, old *ipamv1.IPClaim
			g := NewWithT(t)
			webhook := &IPClaim{}

			newClm = &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "abc-1",
				},
				Spec: *tt.newClm,
			}

			if tt.old != nil {
				old = &ipamv1.IPClaim{
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
				_, err := webhook.ValidateUpdate(ctx, old, newClm)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateUpdate(ctx, old, newClm)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
