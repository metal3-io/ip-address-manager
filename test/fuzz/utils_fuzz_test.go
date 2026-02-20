package fuzz

import (
	"encoding/json"
	"testing"

	ipam "github.com/metal3-io/ip-address-manager/ipam"
)

// FuzzFilterAndContains tests the Filter and Contains utility functions
// with arbitrary string lists and search terms to ensure they handle
// edge cases like empty strings, duplicates, and special characters.
func FuzzFilterAndContains(f *testing.F) {
	// Add seed corpus with various string list configurations
	f.Add([]byte(`{"list":["a","b","c"],"search":"b"}`))
	f.Add([]byte(`{"list":[],"search":"test"}`))
	f.Add([]byte(`{"list":["","test",""],"search":""}`))
	f.Add([]byte(`{"list":["foo","bar","foo"],"search":"foo"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		type TestData struct {
			List   []string `json:"list"`
			Search string   `json:"search"`
		}

		td := &TestData{}
		if err := json.Unmarshal(data, td); err != nil {
			t.Skip("invalid JSON input")
		}

		// Test Contains function
		containsResult := ipam.Contains(td.List, td.Search)

		// Test Filter function
		filteredList := ipam.Filter(td.List, td.Search)

		// Verify invariants:
		// 1. If Contains returns true, filtered list should be shorter
		if containsResult && len(filteredList) >= len(td.List) {
			t.Error("after filtering, list should be shorter when item is contained")
		}

		// 2. Filtered list should not contain the search string
		if ipam.Contains(filteredList, td.Search) {
			t.Error("filtered list should not contain the search string")
		}

		// 3. If Contains returns false, filtered list should be same length
		if !containsResult && len(filteredList) != len(td.List) {
			t.Error("filtered list should have same length when item not found")
		}
	})
}

// FuzzStringSliceOperations tests string slice operations with various
// edge cases including empty slices, single elements, and large slices.
func FuzzStringSliceOperations(f *testing.F) {
	// Add seed corpus
	f.Add([]byte(`{"list1":["a","b"],"list2":["c","d"],"filter":"a"}`))
	f.Add([]byte(`{"list1":[],"list2":[],"filter":"x"}`))
	f.Add([]byte(`{"list1":["test"],"list2":["test"],"filter":"test"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		type TestData struct {
			List1  []string `json:"list1"`
			List2  []string `json:"list2"`
			Filter string   `json:"filter"`
		}

		td := &TestData{}
		if err := json.Unmarshal(data, td); err != nil {
			t.Skip("invalid JSON input")
		}

		// Test multiple Filter operations in sequence
		result1 := ipam.Filter(td.List1, td.Filter)
		result2 := ipam.Filter(result1, td.Filter)

		// After filtering twice with the same string, result should not change
		if len(result2) != len(result1) {
			t.Error("double filtering with same string should produce idempotent result")
		}

		// Verify that filtering an empty list returns empty list
		emptyResult := ipam.Filter([]string{}, td.Filter)
		if len(emptyResult) != 0 {
			t.Error("filtering empty list should return empty list")
		}

		// Verify Contains on empty list returns false
		if ipam.Contains([]string{}, td.Filter) {
			t.Error("Contains on empty list should return false")
		}
	})
}
