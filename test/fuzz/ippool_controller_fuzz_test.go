package fuzz

import (
	"encoding/json"
	"testing"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	controller "github.com/metal3-io/ip-address-manager/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FuzzIPClaimToIPPool tests IPClaim to IPPool reconciliation mapping
// with various IPClaim configurations to ensure correct request handling.
func FuzzIPClaimToIPPool(f *testing.F) {
	// Add seed corpus with various IPClaim configurations
	f.Add([]byte(`{"metadata":{"name":"claim1","namespace":"default"},"spec":{"pool":{"name":"pool1"}}}`))
	f.Add([]byte(`{"metadata":{"name":"claim2"},"spec":{"pool":{"name":"pool2","namespace":"ns1"}}}`))
	f.Add([]byte(`{"spec":{"pool":{"name":"pool3"}}}`))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		ipClaim := &ipamv1.IPClaim{}

		// Attempt to unmarshal the fuzzed data
		if err := json.Unmarshal(data, ipClaim); err != nil {
			t.Skip("invalid JSON input")
		}

		// Create reconciler
		r := controller.IPPoolReconciler{}

		// Test IPClaimToIPPool mapping
		obj := client.Object(ipClaim)
		reqs := r.IPClaimToIPPool(t.Context(), obj)

		// Validate the mapping logic
		if ipClaim.Spec.Pool.Name == "" {
			// If no pool name specified, should return no requests
			if len(reqs) != 0 {
				t.Errorf("expected no requests for empty pool name, got %d", len(reqs))
			}
			return
		}

		// Should return exactly one request when pool name is specified
		if len(reqs) != 1 {
			t.Errorf("expected 1 request, got %d", len(reqs))
			return
		}

		req := reqs[0]

		// Verify the pool name matches
		if req.NamespacedName.Name != ipClaim.Spec.Pool.Name {
			t.Errorf("pool name mismatch: expected %s, got %s",
				ipClaim.Spec.Pool.Name, req.NamespacedName.Name)
		}

		// Verify namespace logic
		expectedNamespace := ipClaim.Spec.Pool.Namespace
		if expectedNamespace == "" {
			expectedNamespace = ipClaim.Namespace
		}
		if req.NamespacedName.Namespace != expectedNamespace {
			t.Errorf("namespace mismatch: expected %s, got %s",
				expectedNamespace, req.NamespacedName.Namespace)
		}
	})
}
