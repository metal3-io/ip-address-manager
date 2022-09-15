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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIPPoolDefault(t *testing.T) {
	g := NewWithT(t)

	c := &IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: IPPoolSpec{},
	}
	c.Default()

	g.Expect(c.Spec).To(Equal(IPPoolSpec{}))
	g.Expect(c.Status).To(Equal(IPPoolStatus{}))
}

func TestIPPoolValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		c         *IPPool
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			c: &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: IPPoolSpec{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if tt.expectErr {
				g.Expect(tt.c.ValidateCreate()).NotTo(Succeed())
			} else {
				g.Expect(tt.c.ValidateCreate()).To(Succeed())
			}

			g.Expect(tt.c.ValidateDelete()).To(Succeed())
		})
	}
}

func TestIPPoolUpdateValidation(t *testing.T) {
	startAddr := IPAddressStr("192.168.0.1")
	endAddr := IPAddressStr("192.168.0.10")

	tests := []struct {
		name          string
		expectErr     bool
		newPoolSpec   *IPPoolSpec
		oldPoolSpec   *IPPoolSpec
		oldPoolStatus IPPoolStatus
	}{
		{
			name:        "should succeed when values and templates correct",
			expectErr:   false,
			newPoolSpec: &IPPoolSpec{},
			oldPoolSpec: &IPPoolSpec{},
		},
		{
			name:        "should fail when oldPoolSpec is nil",
			expectErr:   true,
			newPoolSpec: &IPPoolSpec{},
			oldPoolSpec: nil,
		},
		{
			name:      "should fail when namePrefix value changes",
			expectErr: true,
			newPoolSpec: &IPPoolSpec{
				NamePrefix: "abcde",
			},
			oldPoolSpec: &IPPoolSpec{
				NamePrefix: "abcd",
			},
		},
		{
			name:      "should succeed when preAllocations are correct",
			expectErr: false,
			newPoolSpec: &IPPoolSpec{
				NamePrefix: "abcd",
				Pools: []Pool{
					{Start: &startAddr, End: &endAddr},
				},
				PreAllocations: map[string]IPAddressStr{
					"alloc": IPAddressStr("192.168.0.2"),
				},
			},
			oldPoolSpec: &IPPoolSpec{
				NamePrefix: "abcd",
			},
			oldPoolStatus: IPPoolStatus{
				Allocations: map[string]IPAddressStr{
					"inuse": IPAddressStr("192.168.0.3"),
				},
			},
		},
		{
			name:      "should fail when preAllocations are incorrect",
			expectErr: true,
			newPoolSpec: &IPPoolSpec{
				NamePrefix: "abcd",
				Pools: []Pool{
					{Start: &startAddr, End: &endAddr},
				},
				PreAllocations: map[string]IPAddressStr{
					"alloc": IPAddressStr("192.168.0.20"),
				},
			},
			oldPoolSpec: &IPPoolSpec{
				NamePrefix: "abcd",
			},
			oldPoolStatus: IPPoolStatus{
				Allocations: map[string]IPAddressStr{
					"inuse": IPAddressStr("192.168.0.3"),
				},
			},
		},
		{
			name:      "should fail when ip in use",
			expectErr: true,
			newPoolSpec: &IPPoolSpec{
				NamePrefix: "abcd",
				Pools: []Pool{
					{Start: &startAddr, End: &endAddr},
				},
				PreAllocations: map[string]IPAddressStr{
					"alloc": IPAddressStr("192.168.0.2"),
				},
			},
			oldPoolSpec: &IPPoolSpec{
				NamePrefix: "abcd",
			},
			oldPoolStatus: IPPoolStatus{
				Allocations: map[string]IPAddressStr{
					"inuse": IPAddressStr("192.168.0.30"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newPool, oldPool *IPPool
			g := NewWithT(t)
			newPool = &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: *tt.newPoolSpec,
			}

			if tt.oldPoolSpec != nil {
				oldPool = &IPPool{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
					},
					Spec:   *tt.oldPoolSpec,
					Status: tt.oldPoolStatus,
				}
			} else {
				oldPool = nil
			}

			if tt.expectErr {
				g.Expect(newPool.ValidateUpdate(oldPool)).NotTo(Succeed())
			} else {
				g.Expect(newPool.ValidateUpdate(oldPool)).To(Succeed())
			}
		})
	}
}

func TestIPPoolDeleteValidation(t *testing.T) {
	startAddr := IPAddressStr("192.168.0.1")
	endAddr := IPAddressStr("192.168.0.10")

	tests := []struct {
		name       string
		expectErr  bool
		poolStatus IPPoolStatus
	}{{
		name:      "should fail when ip in use",
		expectErr: true,
		poolStatus: IPPoolStatus{
			Allocations: map[string]IPAddressStr{
				"inuse": IPAddressStr("192.168.0.30"),
			},
		},
	}, {
		name:      "should succeed when no ip in use",
		expectErr: false,
		poolStatus: IPPoolStatus{
			Allocations: map[string]IPAddressStr{},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pool *IPPool
			g := NewWithT(t)
			pool = &IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: IPPoolSpec{
					Pools: []Pool{
						{Start: &startAddr, End: &endAddr},
					},
				},
				Status: tt.poolStatus,
			}

			if tt.expectErr {
				g.Expect(pool.ValidateDelete()).NotTo(Succeed())
			} else {
				g.Expect(pool.ValidateDelete()).To(Succeed())
			}
		})
	}
}
