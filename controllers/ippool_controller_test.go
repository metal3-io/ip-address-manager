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

package controllers

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/metal3-io/ip-address-manager/ipam"
	ipam_mocks "github.com/metal3-io/ip-address-manager/ipam/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capipamv1beta1 "sigs.k8s.io/cluster-api/api/ipam/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	testObjectMeta = metav1.ObjectMeta{
		Name:      "abc",
		Namespace: "myns",
	}
)

var _ = Describe("IPPool controller", func() {

	type testCaseReconcile struct {
		expectError          bool
		expectRequeue        bool
		expectManager        bool
		m3ipp                *ipamv1.IPPool
		cluster              *clusterv1beta1.Cluster
		managerError         bool
		reconcileNormal      bool
		reconcileNormalError bool
		reconcileDeleteError bool
		setOwnerRefError     bool
	}

	DescribeTable("Test Reconcile",
		func(tc testCaseReconcile) {
			gomockCtrl := gomock.NewController(GinkgoT())
			f := ipam_mocks.NewMockManagerFactoryInterface(gomockCtrl)
			m := ipam_mocks.NewMockIPPoolManagerInterface(gomockCtrl)

			objects := []client.Object{}
			if tc.m3ipp != nil {
				objects = append(objects, tc.m3ipp)
			}
			if tc.cluster != nil {
				objects = append(objects, tc.cluster)
			}
			c := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(objects...).Build()

			if tc.managerError {
				f.EXPECT().NewIPPoolManager(gomock.Any(), gomock.Any()).Return(nil, errors.New(""))
			} else if tc.expectManager {
				f.EXPECT().NewIPPoolManager(gomock.Any(), gomock.Any()).Return(m, nil)
			}

			if tc.expectManager {
				if tc.setOwnerRefError {
					m.EXPECT().SetClusterOwnerRef(gomock.Any()).Return(errors.New(""))
				} else {
					if tc.cluster != nil {
						m.EXPECT().SetClusterOwnerRef(gomock.Any()).Return(nil)
					}
				}
			}

			if tc.m3ipp != nil && !tc.m3ipp.DeletionTimestamp.IsZero() && tc.reconcileDeleteError {
				m.EXPECT().UpdateAddresses(context.Background()).Return(0, errors.New(""))
			} else if tc.m3ipp != nil && !tc.m3ipp.DeletionTimestamp.IsZero() {
				m.EXPECT().UpdateAddresses(context.Background()).Return(0, nil)
				m.EXPECT().UnsetFinalizer()
			}

			if tc.m3ipp != nil && tc.m3ipp.DeletionTimestamp.IsZero() &&
				tc.reconcileNormal {
				m.EXPECT().SetFinalizer()
				if tc.reconcileNormalError {
					m.EXPECT().UpdateAddresses(context.Background()).Return(0, errors.New(""))
				} else {
					m.EXPECT().UpdateAddresses(context.Background()).Return(1, nil)
				}
			}

			ipPoolReconcile := &IPPoolReconciler{
				Client:         c,
				ManagerFactory: f,
				Log:            logr.Discard(),
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "abc",
					Namespace: "myns",
				},
			}

			result, err := ipPoolReconcile.Reconcile(context.Background(), req)

			if tc.expectError || tc.managerError || tc.reconcileNormalError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.expectRequeue {
				Expect(result).To(Equal(ctrl.Result{RequeueAfter: requeueAfter, Requeue: true}))
			} else {
				Expect(result).ToNot(Equal(ctrl.Result{RequeueAfter: requeueAfter}))
			}
			gomockCtrl.Finish()
		},
		Entry("IPPool not found", testCaseReconcile{}),
		Entry("Cluster not found", testCaseReconcile{
			m3ipp: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       ipamv1.IPPoolSpec{ClusterName: ptr.To("abc")},
			},
		}),
		Entry("Deletion, Cluster not found", testCaseReconcile{
			m3ipp: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "abc",
					Namespace:         "myns",
					DeletionTimestamp: &timestampNow,
					Finalizers: []string{
						ipamv1.IPClaimFinalizer,
					},
				},
				Spec: ipamv1.IPPoolSpec{ClusterName: ptr.To("abc")},
			},
			expectManager: true,
		}),
		Entry("Deletion, Cluster not found, error", testCaseReconcile{
			m3ipp: &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "abc",
					Namespace:         "myns",
					DeletionTimestamp: &timestampNow,
					Finalizers: []string{
						ipamv1.IPClaimFinalizer,
					},
				},
				Spec: ipamv1.IPPoolSpec{ClusterName: ptr.To("abc")},
			},
			expectManager:        true,
			reconcileDeleteError: true,
			expectError:          true,
		}),
		Entry("Paused cluster", testCaseReconcile{
			m3ipp: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       ipamv1.IPPoolSpec{ClusterName: ptr.To("abc")},
			},
			cluster: &clusterv1beta1.Cluster{
				ObjectMeta: testObjectMeta,
				Spec: clusterv1beta1.ClusterSpec{
					Paused: true,
				},
			},
			expectRequeue: true,
			expectManager: true,
		}),
		Entry("Error in manager", testCaseReconcile{
			m3ipp: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       ipamv1.IPPoolSpec{ClusterName: ptr.To("abc")},
			},
			cluster: &clusterv1beta1.Cluster{
				ObjectMeta: testObjectMeta,
			},
			managerError: true,
		}),
		Entry("Reconcile normal error", testCaseReconcile{
			m3ipp: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       ipamv1.IPPoolSpec{ClusterName: ptr.To("abc")},
			},
			cluster: &clusterv1beta1.Cluster{
				ObjectMeta: testObjectMeta,
			},
			reconcileNormal:      true,
			reconcileNormalError: true,
			expectManager:        true,
		}),
		Entry("Reconcile normal no cluster", testCaseReconcile{
			m3ipp: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       ipamv1.IPPoolSpec{ClusterName: ptr.To("abc")},
			},
			reconcileNormal: false,
			expectManager:   false,
		}),
		Entry("Reconcile normal no error", testCaseReconcile{
			m3ipp: &ipamv1.IPPool{
				ObjectMeta: testObjectMeta,
				Spec:       ipamv1.IPPoolSpec{ClusterName: ptr.To("abc")},
			},
			cluster: &clusterv1beta1.Cluster{
				ObjectMeta: testObjectMeta,
			},
			reconcileNormal: true,
			expectManager:   true,
		}),
	)

	type reconcileNormalTestCase struct {
		ExpectError   bool
		ExpectRequeue bool
		UpdateError   bool
	}

	DescribeTable("ReconcileNormal tests",
		func(tc reconcileNormalTestCase) {
			gomockCtrl := gomock.NewController(GinkgoT())

			c := fake.NewClientBuilder().WithScheme(setupScheme()).Build()

			ipPoolReconcile := &IPPoolReconciler{
				Client:         c,
				ManagerFactory: ipam.NewManagerFactory(c),
				Log:            logr.Discard(),
			}
			m := ipam_mocks.NewMockIPPoolManagerInterface(gomockCtrl)

			m.EXPECT().SetFinalizer()

			if !tc.UpdateError {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(1, nil)
			} else {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(0, errors.New(""))
			}

			result, err := ipPoolReconcile.reconcileNormal(context.TODO(), m)
			gomockCtrl.Finish()

			if tc.ExpectError {
				Expect(err).To(HaveOccurred(), "Expected an error did not got one %v", err)
			} else {
				Expect(err).NotTo(HaveOccurred(), "Expected no error but got one: %v", err)
			}
			if tc.ExpectRequeue {
				Expect(result).To(Equal(ctrl.Result{RequeueAfter: requeueAfter, Requeue: true}))
			} else {
				Expect(result).ToNot(Equal(ctrl.Result{RequeueAfter: requeueAfter}))
			}
		},
		Entry("No error", reconcileNormalTestCase{
			ExpectError:   false,
			ExpectRequeue: false,
		}),
		Entry("Update error", reconcileNormalTestCase{
			UpdateError:   true,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
	)

	type reconcileDeleteTestCase struct {
		ExpectError   bool
		ExpectRequeue bool
		DeleteReady   bool
		DeleteError   bool
	}

	DescribeTable("ReconcileDelete tests",
		func(tc reconcileDeleteTestCase) {
			gomockCtrl := gomock.NewController(GinkgoT())

			c := fake.NewClientBuilder().WithScheme(setupScheme()).Build()
			ipPoolReconcile := &IPPoolReconciler{
				Client:         c,
				ManagerFactory: ipam.NewManagerFactory(c),
				Log:            logr.Discard(),
			}
			m := ipam_mocks.NewMockIPPoolManagerInterface(gomockCtrl)

			if !tc.DeleteError && tc.DeleteReady {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(0, nil)
				m.EXPECT().UnsetFinalizer()
			} else if !tc.DeleteError {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(1, nil)
			} else {
				m.EXPECT().UpdateAddresses(context.TODO()).Return(0, errors.New(""))
			}

			result, err := ipPoolReconcile.reconcileDelete(context.TODO(), m)
			gomockCtrl.Finish()

			if tc.ExpectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.ExpectRequeue {
				Expect(result).To(Equal(ctrl.Result{RequeueAfter: requeueAfter, Requeue: true}))
			} else {
				Expect(result).ToNot(Equal(ctrl.Result{RequeueAfter: requeueAfter}))
			}
		},
		Entry("No error", reconcileDeleteTestCase{
			ExpectError:   false,
			ExpectRequeue: false,
		}),
		Entry("Delete error", reconcileDeleteTestCase{
			DeleteError:   true,
			ExpectError:   true,
			ExpectRequeue: false,
		}),
		Entry("Delete ready", reconcileDeleteTestCase{
			ExpectError:   false,
			ExpectRequeue: false,
			DeleteReady:   true,
		}),
	)

	type TestCaseM3IPCToM3IPP struct {
		IPClaim       *ipamv1.IPClaim
		ExpectRequest bool
	}

	DescribeTable("IPClaim To IPPool tests",
		func(tc TestCaseM3IPCToM3IPP) {
			r := IPPoolReconciler{}
			obj := client.Object(tc.IPClaim)
			reqs := r.IPClaimToIPPool(context.Background(), obj)

			if tc.ExpectRequest {
				Expect(reqs).To(HaveLen(1), "Expected 1 request, found %d", len(reqs))

				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.IPClaim.Spec.Pool.Name),
					"Expected name %s, found %s", tc.IPClaim.Spec.Pool.Name, req.NamespacedName.Name)
				if tc.IPClaim.Spec.Pool.Namespace == "" {
					Expect(req.NamespacedName.Namespace).To(Equal(tc.IPClaim.Namespace),
						"Expected namespace %s, found %s", tc.IPClaim.Namespace, req.NamespacedName.Namespace)
				} else {
					Expect(req.NamespacedName.Namespace).To(Equal(tc.IPClaim.Spec.Pool.Namespace),
						"Expected namespace %s, found %s", tc.IPClaim.Spec.Pool.Namespace, req.NamespacedName.Namespace)
				}

			} else {
				Expect(reqs).To(BeEmpty(), "Expected 0 request, found %d", len(reqs))

			}
		},
		Entry("No IPPool in Spec",
			TestCaseM3IPCToM3IPP{
				IPClaim: &ipamv1.IPClaim{
					ObjectMeta: testObjectMeta,
					Spec:       ipamv1.IPClaimSpec{},
				},
				ExpectRequest: false,
			},
		),
		Entry("IPPool in Spec, with namespace",
			TestCaseM3IPCToM3IPP{
				IPClaim: &ipamv1.IPClaim{
					ObjectMeta: testObjectMeta,
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name:      "abc",
							Namespace: "myns",
						},
					},
				},
				ExpectRequest: true,
			},
		),
		Entry("IPPool in Spec, no namespace",
			TestCaseM3IPCToM3IPP{
				IPClaim: &ipamv1.IPClaim{
					ObjectMeta: testObjectMeta,
					Spec: ipamv1.IPClaimSpec{
						Pool: corev1.ObjectReference{
							Name: "abc",
						},
					},
				},
				ExpectRequest: true,
			},
		),
	)

	type TestCaseK8SIPACToM3IPP struct {
		IPAddressClaim *capipamv1beta1.IPAddressClaim
		ExpectRequest  bool
	}

	DescribeTable("IPAddressClaim To IPPool tests",
		func(tc TestCaseK8SIPACToM3IPP) {
			r := IPPoolReconciler{}
			obj := client.Object(tc.IPAddressClaim)
			reqs := r.IPAddressClaimToIPPool(context.Background(), obj)

			if tc.ExpectRequest {
				Expect(reqs).To(HaveLen(1), "Expected 1 request, found %d", len(reqs))

				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.IPAddressClaim.Spec.PoolRef.Name),
					"Expected name %s, found %s", tc.IPAddressClaim.Spec.PoolRef.Name, req.NamespacedName.Name)
			} else {
				Expect(reqs).To(BeEmpty(), "Expected 0 request, found %d", len(reqs))

			}
		},
		Entry("No IPPool in Spec",
			TestCaseK8SIPACToM3IPP{
				IPAddressClaim: &capipamv1beta1.IPAddressClaim{
					ObjectMeta: testObjectMeta,
					Spec:       capipamv1beta1.IPAddressClaimSpec{},
				},
				ExpectRequest: false,
			},
		),
		Entry("IPPool in Spec, with namespace",
			TestCaseK8SIPACToM3IPP{
				IPAddressClaim: &capipamv1beta1.IPAddressClaim{
					ObjectMeta: testObjectMeta,
					Spec: capipamv1beta1.IPAddressClaimSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "abc",
						},
					},
				},
				ExpectRequest: true,
			},
		),
		Entry("IPPool in Spec, no namespace",
			TestCaseK8SIPACToM3IPP{
				IPAddressClaim: &capipamv1beta1.IPAddressClaim{
					ObjectMeta: testObjectMeta,
					Spec: capipamv1beta1.IPAddressClaimSpec{
						PoolRef: corev1.TypedLocalObjectReference{
							Name: "abc",
						},
					},
				},
				ExpectRequest: true,
			},
		),
	)
})
