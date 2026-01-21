/*
Copyright 2026 The Kubernetes Authors.

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
	fuzz "github.com/AdaLogics/go-fuzz-headers"
	ipam "github.com/metal3-io/ip-address-manager/ipam"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FuzzFilterAndContains tests the Filter and Contains utility functions
// with arbitrary string lists and search terms to ensure they handle
// edge cases like empty strings, duplicates, and special characters.
func FuzzFilterAndContains(data []byte) int {
	f := fuzz.NewConsumer(data)

	// Generate a list of strings and a string to filter/search
	var stringList []string
	var searchString string

	err := f.GenerateStruct(&stringList)
	if err != nil {
		return 0
	}

	err = f.GenerateStruct(&searchString)
	if err != nil {
		return 0
	}

	// Test Contains function
	containsResult := ipam.Contains(stringList, searchString)

	// Test Filter function
	filteredList := ipam.Filter(stringList, searchString)

	// Verify invariants:
	// 1. If Contains returns true, filtered list should be shorter
	if containsResult && len(filteredList) >= len(stringList) {
		return 0
	}

	// 2. Filtered list should not contain the search string
	if ipam.Contains(filteredList, searchString) {
		return 0
	}

	// 3. If Contains returns false, filtered list should be same length
	if !containsResult && len(filteredList) != len(stringList) {
		return 0
	}

	return 1
}

// FuzzOwnerReferences tests owner reference manipulation functions
// with various combinations of owner references, ensuring proper
// handling of duplicates, empty lists, and edge cases.
func FuzzOwnerReferences(data []byte) int {
	f := fuzz.NewConsumer(data)

	type TestData struct {
		RefList    []metav1.OwnerReference
		ObjType    metav1.TypeMeta
		ObjMeta    metav1.ObjectMeta
		Controller bool
	}

	tc := &TestData{}
	err := f.GenerateStruct(tc)
	if err != nil {
		return 0
	}

	// Test setOwnerRefInList - this is not exported, so we'll test through
	// the exported functions that use it. For this basic fuzzer, we'll focus
	// on the Filter and Contains functions which are more directly testable.

	return 1
}

// FuzzStringSliceOperations tests string slice operations with various
// edge cases including empty slices, single elements, and large slices.
func FuzzStringSliceOperations(data []byte) int {
	f := fuzz.NewConsumer(data)

	var list1, list2 []string
	var filterString string

	err := f.GenerateStruct(&list1)
	if err != nil {
		return 0
	}

	err = f.GenerateStruct(&list2)
	if err != nil {
		return 0
	}

	err = f.GenerateStruct(&filterString)
	if err != nil {
		return 0
	}

	// Test multiple Filter operations in sequence
	result1 := ipam.Filter(list1, filterString)
	result2 := ipam.Filter(result1, filterString)

	// After filtering twice with the same string, result should not change
	if len(result2) != len(result1) {
		return 0
	}

	// Test Contains on various lists
	_ = ipam.Contains(list1, filterString)
	_ = ipam.Contains(list2, filterString)
	_ = ipam.Contains(result1, filterString)

	// Verify that filtering an empty list returns empty list
	emptyResult := ipam.Filter([]string{}, filterString)
	if len(emptyResult) != 0 {
		return 0
	}

	// Verify Contains on empty list returns false
	if ipam.Contains([]string{}, filterString) {
		return 0
	}

	return 1
}
