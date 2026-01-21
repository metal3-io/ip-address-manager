package fuzz

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	ipam "github.com/metal3-io/ip-address-manager/ipam"
)

// FuzzNewIPPoolManager tests IPPool manager creation and finalizer operations
// with randomly generated IPPool objects to ensure robust handling of edge cases.
func FuzzNewIPPoolManager(f *testing.F) {
	// Add seed corpus with various IPPool configurations
	f.Add([]byte(`{"metadata":{"name":"test-pool"}}`))
	f.Add([]byte(`{"metadata":{"name":"pool","namespace":"default"},"spec":{"clusterName":"cluster1"}}`))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		ipPool := &ipamv1.IPPool{}

		// Attempt to unmarshal the fuzzed data
		if err := json.Unmarshal(data, ipPool); err != nil {
			t.Skip("invalid JSON input")
		}

		// Attempt to create IPPool manager
		ipPoolMgr, err := ipam.NewIPPoolManager(nil, ipPool, logr.Discard())
		if err != nil {
			// Some inputs may validly fail manager creation
			t.Skip("manager creation failed")
		}

		// Test SetFinalizer
		ipPoolMgr.SetFinalizer()
		if len(ipPool.ObjectMeta.Finalizers) == 0 {
			t.Error("SetFinalizer failed to add finalizer")
		}

		// Test UnsetFinalizer
		ipPoolMgr.UnsetFinalizer()
		if len(ipPool.ObjectMeta.Finalizers) != 0 {
			t.Error("UnsetFinalizer failed to remove finalizer")
		}
	})
}
