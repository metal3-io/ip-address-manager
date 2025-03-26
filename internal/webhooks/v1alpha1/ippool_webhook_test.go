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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var ctx = ctrl.SetupSignalHandler()

func TestIPPoolDefault(t *testing.T) {
	g := NewWithT(t)
	webhook := &IPPool{}

	c := &ipamv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: ipamv1.IPPoolSpec{},
	}
	g.Expect(webhook.Default(ctx, c)).To(Succeed())

	g.Expect(c.Spec).To(Equal(ipamv1.IPPoolSpec{}))
	g.Expect(c.Status).To(Equal(ipamv1.IPPoolStatus{}))
}

func TestIPPoolValidation(t *testing.T) {
	tests := []struct {
		name      string
		expectErr bool
		c         *ipamv1.IPPool
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			c: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: ipamv1.IPPoolSpec{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &IPPool{}

			if tt.expectErr {
				_, err := webhook.ValidateCreate(ctx, tt.c)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateCreate(ctx, tt.c)
				g.Expect(err).NotTo(HaveOccurred())
			}
			_, err := webhook.ValidateDelete(ctx, tt.c)
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestIPPoolUpdateValidation(t *testing.T) {
	startAddr := ipamv1.IPAddressStr("192.168.0.1")
	endAddr := ipamv1.IPAddressStr("192.168.0.10")
	subnet := ipamv1.IPSubnetStr("192.168.0.1/25")

	tests := []struct {
		name          string
		expectErr     bool
		newPoolSpec   *ipamv1.IPPoolSpec
		oldPoolSpec   *ipamv1.IPPoolSpec
		oldPoolStatus ipamv1.IPPoolStatus
	}{
		{
			name:        "should succeed when values and templates correct",
			expectErr:   false,
			newPoolSpec: &ipamv1.IPPoolSpec{},
			oldPoolSpec: &ipamv1.IPPoolSpec{},
		},
		{
			name:        "should fail when oldPoolSpec is nil",
			expectErr:   true,
			newPoolSpec: &ipamv1.IPPoolSpec{},
			oldPoolSpec: nil,
		},
		{
			name:      "should fail when namePrefix value changes",
			expectErr: true,
			newPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcde",
			},
			oldPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
			},
		},
		{
			name:      "should succeed when preAllocations are between start and end",
			expectErr: false,
			newPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
				Pools: []ipamv1.Pool{
					{Start: &startAddr, End: &endAddr},
				},
				PreAllocations: map[string]ipamv1.IPAddressStr{
					"alloc": ipamv1.IPAddressStr("192.168.0.2"),
				},
			},
			oldPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
			},
			oldPoolStatus: ipamv1.IPPoolStatus{
				Allocations: map[string]ipamv1.IPAddressStr{
					"inuse": ipamv1.IPAddressStr("192.168.0.3"),
				},
			},
		},
		{
			name:      "should fail when preAllocations are out of start and end",
			expectErr: true,
			newPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
				Pools: []ipamv1.Pool{
					{Start: &startAddr, End: &endAddr},
				},
				PreAllocations: map[string]ipamv1.IPAddressStr{
					"alloc": ipamv1.IPAddressStr("192.168.0.20"),
				},
			},
			oldPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
			},
			oldPoolStatus: ipamv1.IPPoolStatus{
				Allocations: map[string]ipamv1.IPAddressStr{
					"inuse": ipamv1.IPAddressStr("192.168.0.3"),
				},
			},
		},
		{
			name:      "should succeed when preAllocations are in the cidr",
			expectErr: false,
			newPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
				Pools: []ipamv1.Pool{
					{Subnet: &subnet},
				},
				PreAllocations: map[string]ipamv1.IPAddressStr{
					"alloc": ipamv1.IPAddressStr("192.168.0.2"),
				},
			},
			oldPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
			},
			oldPoolStatus: ipamv1.IPPoolStatus{
				Allocations: map[string]ipamv1.IPAddressStr{
					"inuse": ipamv1.IPAddressStr("192.168.0.3"),
				},
			},
		},
		{
			name:      "should fail when preAllocations are out of cidr",
			expectErr: true,
			newPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
				Pools: []ipamv1.Pool{
					{Subnet: &subnet},
				},
				PreAllocations: map[string]ipamv1.IPAddressStr{
					"alloc": ipamv1.IPAddressStr("192.168.0.250"),
				},
			},
			oldPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
			},
			oldPoolStatus: ipamv1.IPPoolStatus{
				Allocations: map[string]ipamv1.IPAddressStr{
					"inuse": ipamv1.IPAddressStr("192.168.0.3"),
				},
			},
		},
		{
			name:      "should fail when ip in use",
			expectErr: true,
			newPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
				Pools: []ipamv1.Pool{
					{Start: &startAddr, End: &endAddr},
				},
				PreAllocations: map[string]ipamv1.IPAddressStr{
					"alloc": ipamv1.IPAddressStr("192.168.0.2"),
				},
			},
			oldPoolSpec: &ipamv1.IPPoolSpec{
				NamePrefix: "abcd",
			},
			oldPoolStatus: ipamv1.IPPoolStatus{
				Allocations: map[string]ipamv1.IPAddressStr{
					"inuse": ipamv1.IPAddressStr("192.168.0.30"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var newPool, oldPool *ipamv1.IPPool
			g := NewWithT(t)
			webhook := &IPPool{}

			newPool = &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: *tt.newPoolSpec,
			}

			if tt.oldPoolSpec != nil {
				oldPool = &ipamv1.IPPool{
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
				_, err := webhook.ValidateUpdate(ctx, oldPool, newPool)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateUpdate(ctx, oldPool, newPool)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
