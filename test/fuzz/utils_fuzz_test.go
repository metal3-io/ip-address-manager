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
	// Additional seed corpus for edge cases
	f.Add([]byte(`{"list":["192.168.1.1","10.0.0.1","172.16.0.1"],"search":"10.0.0.1"}`))
	f.Add([]byte(`{"list":["pool-1","pool-2","pool-3"],"search":"pool-1"}`))
	f.Add([]byte(`{"list":["test","Test","TEST"],"search":"test"}`))
	f.Add([]byte(`{"list":["item-with-dashes","item_with_underscores","item.with.dots"],"search":"item-with-dashes"}`))
	f.Add([]byte(`{"list":["special-chars-@#$","percent-%","ampersand-&"],"search":"special-chars-@#$"}`))
	f.Add([]byte(`{"list":[" leading-space","trailing-space ","  both  "],"search":" leading-space"}`))
	f.Add([]byte(`{"list":["very-long-string-that-exceeds-typical-length-limits-for-testing","short"],"search":"very-long-string-that-exceeds-typical-length-limits-for-testing"}`))
	f.Add([]byte(`{"list":["single"],"search":"single"}`))
	f.Add([]byte(`{"list":["single"],"search":"missing"}`))
	f.Add([]byte(`{"list":["newline\nchar","tab\tchar","return\rchar"],"search":"newline\nchar"}`))
	f.Add([]byte(`{"list":["a","a","a","a","a"],"search":"a"}`))
	f.Add([]byte(`{"list":["ippool.cluster.x-k8s.io","metal3.io/v1alpha1"],"search":"metal3.io/v1alpha1"}`))
	f.Add([]byte(`{"list":["claim-1","claim-2","claim-10","claim-20"],"search":"claim-10"}`))
	f.Add([]byte(`{"list":["0","1","2","3","4","5","6","7","8","9"],"search":"5"}`))

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
	// Additional seed corpus for edge cases
	f.Add([]byte(`{"list1":["192.168.1.1","192.168.1.2"],"list2":["192.168.1.3","192.168.1.4"],"filter":"192.168.1.1"}`))
	f.Add([]byte(`{"list1":["claim-a","claim-b","claim-c"],"list2":["claim-d","claim-e"],"filter":"claim-a"}`))
	f.Add([]byte(`{"list1":["","",""],"list2":["test"],"filter":""}`))
	f.Add([]byte(`{"list1":["duplicate","duplicate","unique"],"list2":["duplicate"],"filter":"duplicate"}`))
	f.Add([]byte(`{"list1":["very-long-item-name-with-many-characters"],"list2":["short"],"filter":"short"}`))
	f.Add([]byte(`{"list1":["path/to/resource","cluster/node"],"list2":["group/api"],"filter":"path/to/resource"}`))
	f.Add([]byte(`{"list1":["namespace/name","cluster/resource"],"list2":["group/version"],"filter":"namespace/name"}`))
	f.Add([]byte(`{"list1":["item1","item2","item3","item4","item5"],"list2":["item6","item7","item8"],"filter":"item3"}`))
	f.Add([]byte(`{"list1":["CamelCase","snake_case","kebab-case"],"list2":["PascalCase"],"filter":"snake_case"}`))
	f.Add([]byte(`{"list1":["with\ttab","with\nline"],"list2":["with\rreturn"],"filter":"with\ttab"}`))
	f.Add([]byte(`{"list1":["1","2","3"],"list2":["4","5","6"],"filter":"2"}`))
	f.Add([]byte(`{"list1":["ippool-default","ippool-prod","ippool-dev"],"list2":["ippool-test"],"filter":"ippool-prod"}`))
	f.Add([]byte(`{"list1":[" space "," space "],"list2":["no-space"],"filter":" space "}`))

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
