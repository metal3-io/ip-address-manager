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

import "testing"

// TestFuzzFilterAndContains verifies the fuzzer runs without panicking
// with basic test data.
func TestFuzzFilterAndContains(t *testing.T) {
	testCases := [][]byte{
		{},
		{0x01},
		{0x01, 0x02, 0x03, 0x04, 0x05},
		{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
			0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14},
	}

	for i, tc := range testCases {
		result := FuzzFilterAndContains(tc)
		t.Logf("Test case %d returned: %d", i, result)
	}
}

// TestFuzzOwnerReferences verifies the fuzzer runs without panicking
// with basic test data.
func TestFuzzOwnerReferences(t *testing.T) {
	testCases := [][]byte{
		{},
		{0x01},
		{0x01, 0x02, 0x03, 0x04, 0x05},
	}

	for i, tc := range testCases {
		result := FuzzOwnerReferences(tc)
		t.Logf("Test case %d returned: %d", i, result)
	}
}

// TestFuzzStringSliceOperations verifies the fuzzer runs without panicking
// with basic test data.
func TestFuzzStringSliceOperations(t *testing.T) {
	testCases := [][]byte{
		{},
		{0x01},
		{0x01, 0x02, 0x03, 0x04, 0x05},
		{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa, 0xf9, 0xf8},
	}

	for i, tc := range testCases {
		result := FuzzStringSliceOperations(tc)
		t.Logf("Test case %d returned: %d", i, result)
	}
}
