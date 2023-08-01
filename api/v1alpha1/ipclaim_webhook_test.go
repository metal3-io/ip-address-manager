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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIPClaimDefault(t *testing.T) {
	g := NewWithT(t)

	c := &IPClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
	}
	c.Default()

	g.Expect(c.Spec).To(Equal(IPClaimSpec{}))
	g.Expect(c.Status).To(Equal(IPClaimStatus{}))
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

			obj := &IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      tt.claimName,
				},
				Spec: IPClaimSpec{
					Pool: tt.ipPool,
				},
			}

			if tt.expectErr {
				_, err := obj.ValidateCreate()
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := obj.ValidateCreate()
				g.Expect(err).NotTo(HaveOccurred())
			}
			_, err := obj.ValidateDelete()
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestIPClaimUpdateValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		newClm    *IPClaimSpec
		old       *IPClaimSpec
	}{
		{
			name:      "should succeed when values are the same",
			expectErr: false,
			newClm: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
		},
		{
			name:      "should fail with nil old",
			expectErr: true,
			newClm: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: nil,
		},
		{
			name:      "should fail when pool is unset",
			expectErr: true,
			newClm: &IPClaimSpec{
				Pool: corev1.ObjectReference{},
			},
			old: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
		},
		{
			name:      "should fail when pool name changes",
			expectErr: true,
			newClm: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
				},
			},
			old: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abcd",
				},
			},
		},
		{
			name:      "should fail when Pool Namespace changes",
			expectErr: true,
			newClm: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abc",
				},
			},
			old: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name:      "abc",
					Namespace: "abcd",
				},
			},
		},
		{
			name:      "should fail when Pool kind changes",
			expectErr: true,
			newClm: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
					Kind: "abc",
				},
			},
			old: &IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name: "abc",
					Kind: "abcd",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newClm, old *IPClaim
			g := NewWithT(t)
			newClm = &IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "abc-1",
				},
				Spec: *tt.newClm,
			}

			if tt.old != nil {
				old = &IPClaim{
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
				_, err := newClm.ValidateUpdate(old)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := newClm.ValidateUpdate(old)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
