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

package ipam

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	ipamv1 "github.com/metal3-io/metal3-ipam/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Metal3 manager utils", func() {

	type testCaseFilter struct {
		TestList     []string
		TestString   string
		ExpectedList []string
	}

	DescribeTable("Test Filter",
		func(tc testCaseFilter) {
			resultList := Filter(tc.TestList, tc.TestString)
			Expect(resultList).To(Equal(tc.ExpectedList))
		},
		Entry("Absent", testCaseFilter{
			TestList:     []string{"abc", "bcd", "def"},
			TestString:   "efg",
			ExpectedList: []string{"abc", "bcd", "def"},
		}),
		Entry("Present in 1", testCaseFilter{
			TestList:     []string{"abc", "bcd", "def"},
			TestString:   "abc",
			ExpectedList: []string{"bcd", "def"},
		}),
		Entry("Present in 2", testCaseFilter{
			TestList:     []string{"abc", "bcd", "def"},
			TestString:   "bcd",
			ExpectedList: []string{"abc", "def"},
		}),
		Entry("Present in 3", testCaseFilter{
			TestList:     []string{"abc", "bcd", "def"},
			TestString:   "def",
			ExpectedList: []string{"abc", "bcd"},
		}),
	)

	type testCaseContains struct {
		TestList       []string
		TestString     string
		ExpectedOutput bool
	}

	DescribeTable("Test Filter",
		func(tc testCaseContains) {
			Expect(Contains(tc.TestList, tc.TestString)).To(Equal(tc.ExpectedOutput))
		},
		Entry("Absent", testCaseContains{
			TestList:       []string{"abc", "bcd", "def"},
			TestString:     "efg",
			ExpectedOutput: false,
		}),
		Entry("Present 1", testCaseContains{
			TestList:       []string{"abc", "bcd", "def"},
			TestString:     "abc",
			ExpectedOutput: true,
		}),
		Entry("Present 2", testCaseContains{
			TestList:       []string{"abc", "bcd", "def"},
			TestString:     "bcd",
			ExpectedOutput: true,
		}),
		Entry("Present 3", testCaseContains{
			TestList:       []string{"abc", "bcd", "def"},
			TestString:     "def",
			ExpectedOutput: true,
		}),
	)

	Describe("NotFoundError", func() {
		It("should return proper message", func() {
			err := &NotFoundError{}
			Expect(err.Error()).To(Equal("Object not found"))
		})
	})

	type testCaseUpdate struct {
		TestObject     *ipamv1.IPClaim
		ExistingObject *ipamv1.IPClaim
		ExpectedError  bool
	}

	DescribeTable("Test Update",
		func(tc testCaseUpdate) {
			c := k8sClient
			if tc.ExistingObject != nil {
				err := c.Create(context.TODO(), tc.ExistingObject)
				Expect(err).NotTo(HaveOccurred())
				ipPool := ipamv1.IPClaim{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.ExistingObject.Name,
						Namespace: tc.ExistingObject.Namespace,
					},
					&ipPool,
				)
				Expect(err).NotTo(HaveOccurred())
				tc.TestObject.ObjectMeta = ipPool.ObjectMeta
			}
			obj := tc.TestObject.DeepCopy()
			err := updateObject(c, context.TODO(), obj)
			if tc.ExpectedError {
				Expect(err).To(HaveOccurred())
				Expect(err).NotTo(BeAssignableToTypeOf(&RequeueAfterError{}))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.Spec).To(Equal(tc.TestObject.Spec))
				Expect(obj.Status).To(Equal(tc.TestObject.Status))
				savedObject := ipamv1.IPClaim{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.TestObject.Name,
						Namespace: tc.TestObject.Namespace,
					},
					&savedObject,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedObject.Spec).To(Equal(tc.TestObject.Spec))
				Expect(savedObject.ResourceVersion).NotTo(Equal(tc.TestObject.ResourceVersion))
				err := updateObject(c, context.TODO(), obj)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
			}
			err = c.Delete(context.TODO(), tc.TestObject)
			if err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Object does not exist", testCaseUpdate{
			TestObject: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
				},
				Spec: ipamv1.IPClaimSpec{
					Pool: corev1.ObjectReference{Name: "abc"},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{Name: "abc"},
				},
			},
			ExistingObject: nil,
			ExpectedError:  true,
		}),
		Entry("Object exists", testCaseUpdate{
			TestObject: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
				},
				Spec: ipamv1.IPClaimSpec{
					Pool: corev1.ObjectReference{Name: "abc"},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{Name: "abc"},
				},
			},
			ExistingObject: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
				},
				Spec: ipamv1.IPClaimSpec{
					Pool: corev1.ObjectReference{Name: "abcd"},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{Name: "abcd"},
				},
			},
			ExpectedError: false,
		}),
	)

	DescribeTable("Test Create",
		func(tc testCaseUpdate) {
			c := k8sClient
			if tc.ExistingObject != nil {
				err := c.Create(context.TODO(), tc.ExistingObject)
				Expect(err).NotTo(HaveOccurred())
			}
			obj := tc.TestObject.DeepCopy()
			err := createObject(c, context.TODO(), obj)
			if tc.ExpectedError {
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&RequeueAfterError{}))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(obj.Spec).To(Equal(tc.TestObject.Spec))
				Expect(obj.Status).To(Equal(tc.TestObject.Status))
				savedObject := ipamv1.IPClaim{}
				err = c.Get(context.TODO(),
					client.ObjectKey{
						Name:      tc.TestObject.Name,
						Namespace: tc.TestObject.Namespace,
					},
					&savedObject,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedObject.Spec).To(Equal(tc.TestObject.Spec))
			}
			err = c.Delete(context.TODO(), tc.TestObject)
			if err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		},
		Entry("Object does not exist", testCaseUpdate{
			TestObject: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
				},
				Spec: ipamv1.IPClaimSpec{
					Pool: corev1.ObjectReference{Name: "abc"},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{Name: "abc"},
				},
			},
			ExistingObject: nil,
			ExpectedError:  false,
		}),
		Entry("Object exists", testCaseUpdate{
			TestObject: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
				},
				Spec: ipamv1.IPClaimSpec{
					Pool: corev1.ObjectReference{Name: "abc"},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{Name: "abc"},
				},
			},
			ExistingObject: &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abc",
					Namespace: "myns",
				},
				Spec: ipamv1.IPClaimSpec{
					Pool: corev1.ObjectReference{Name: "abcd"},
				},
				Status: ipamv1.IPClaimStatus{
					Address: &corev1.ObjectReference{Name: "abcd"},
				},
			},
			ExpectedError: true,
		}),
	)

	type testCaseFindOwnerRef struct {
		OwnerRefs     []metav1.OwnerReference
		ExpectError   bool
		ExpectedIndex int
	}

	DescribeTable("Test FindOwnerRef",
		func(tc testCaseFindOwnerRef) {
			objType := metav1.TypeMeta{
				APIVersion: "abc.com/v1",
				Kind:       "def",
			}
			objMeta := metav1.ObjectMeta{
				Name: "ghi",
				UID:  "adfasdf",
			}
			index, err := findOwnerRefFromList(tc.OwnerRefs, objType, objMeta)
			if tc.ExpectError {
				Expect(err).NotTo(BeNil())
				Expect(err).To(BeAssignableToTypeOf(&NotFoundError{}))
			} else {
				Expect(err).To(BeNil())
				Expect(index).To(BeEquivalentTo(tc.ExpectedIndex))
			}
		},
		Entry("Empty list", testCaseFindOwnerRef{
			OwnerRefs:   []metav1.OwnerReference{},
			ExpectError: true,
		}),
		Entry("Absent", testCaseFindOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghij",
					UID:        "adfasdf",
				},
			},
			ExpectError: true,
		}),
		Entry("Present 0", testCaseFindOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 0,
		}),
		Entry("Present 1", testCaseFindOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghij",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 1,
		}),
		Entry("Present but different versions", testCaseFindOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v2",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
			ExpectError:   false,
			ExpectedIndex: 0,
		}),
		Entry("Wrong group", testCaseFindOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.co/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
			ExpectError: true,
		}),
	)

	type testCaseOwnerRef struct {
		OwnerRefs  []metav1.OwnerReference
		Controller bool
	}

	DescribeTable("Test DeleteOwnerRef",
		func(tc testCaseOwnerRef) {
			objType := metav1.TypeMeta{
				APIVersion: "abc.com/v1",
				Kind:       "def",
			}
			objMeta := metav1.ObjectMeta{
				Name: "ghi",
				UID:  "adfasdf",
			}
			refList, err := deleteOwnerRefFromList(tc.OwnerRefs, objType, objMeta)
			Expect(err).To(BeNil())
			_, err = findOwnerRefFromList(refList, objType, objMeta)
			Expect(err).NotTo(BeNil())
		},
		Entry("Empty list", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{},
		}),
		Entry("Absent", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghij",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 0", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 1", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghij",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghij",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
	)

	DescribeTable("Test SetOwnerRef",
		func(tc testCaseOwnerRef) {
			objType := metav1.TypeMeta{
				APIVersion: "abc.com/v1",
				Kind:       "def",
			}
			objMeta := metav1.ObjectMeta{
				Name: "ghi",
				UID:  "adfasdf",
			}
			refList, err := setOwnerRefInList(tc.OwnerRefs, tc.Controller, objType, objMeta)
			Expect(err).To(BeNil())
			index, err := findOwnerRefFromList(refList, objType, objMeta)
			Expect(err).To(BeNil())
			Expect(*refList[index].Controller).To(BeEquivalentTo(tc.Controller))
		},
		Entry("Empty list", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{},
		}),
		Entry("Absent", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghij",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 0", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
		Entry("Present 1", testCaseOwnerRef{
			OwnerRefs: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghij",
					UID:        "adfasdf",
				},
				metav1.OwnerReference{
					APIVersion: "abc.com/v1",
					Kind:       "def",
					Name:       "ghi",
					UID:        "adfasdf",
				},
			},
		}),
	)
})
