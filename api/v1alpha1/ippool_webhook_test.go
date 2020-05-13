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

	tests := []struct {
		name      string
		expectErr bool
		newPool   *IPPoolSpec
		oldPool   *IPPoolSpec
	}{
		{
			name:      "should succeed when values and templates correct",
			expectErr: false,
			newPool:   &IPPoolSpec{},
			oldPool:   &IPPoolSpec{},
		},
		{
			name:      "should fail when oldPool is nil",
			expectErr: true,
			newPool:   &IPPoolSpec{},
			oldPool:   nil,
		},
		{
			name:      "should fail when namePrefix value changes",
			expectErr: true,
			newPool: &IPPoolSpec{
				NamePrefix: "abcde",
			},
			oldPool: &IPPoolSpec{
				NamePrefix: "abcd",
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
				Spec: *tt.newPool,
			}

			if tt.oldPool != nil {
				oldPool = &IPPool{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
					},
					Spec: *tt.oldPool,
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
