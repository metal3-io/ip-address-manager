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
	// Additional seed corpus for edge cases
	f.Add([]byte(`{"metadata":{"name":"ipv4-pool","namespace":"metal3-system"},"spec":{"clusterName":"test-cluster","namePrefix":"test"}}`))
	f.Add([]byte(`{"metadata":{"name":"ipv6-pool","namespace":"default"},"spec":{"clusterName":"prod-cluster","pools":[{"start":"2001:db8::1","end":"2001:db8::10"}]}}`))
	f.Add([]byte(`{"metadata":{"name":"pool-with-prefix","namespace":"system"},"spec":{"prefix":24,"clusterName":"cluster"}}`))
	f.Add([]byte(`{"metadata":{"name":"pool-with-gateway","namespace":"ipam"},"spec":{"gateway":"192.168.1.1","clusterName":"edge-cluster"}}`))
	f.Add([]byte(`{"metadata":{"name":"pool","namespace":""},"spec":{"clusterName":""}}`))
	f.Add([]byte(`{"metadata":{"name":"pool-with-labels","namespace":"default","labels":{"env":"prod","tier":"control-plane"}},"spec":{"clusterName":"main"}}`))
	f.Add([]byte(`{"metadata":{"name":"pool-dash-name-123","namespace":"kube-system"},"spec":{"clusterName":"cluster-prod-001","namePrefix":"ip"}}`))
	f.Add([]byte(`{"metadata":{"name":"minimal"},"spec":{}}`))
	f.Add([]byte(`{"metadata":{"name":"pool-annotations","annotations":{"description":"test pool"}},"spec":{"clusterName":"dev"}}`))
	f.Add([]byte(`{"metadata":{"name":"very-long-pool-name-with-many-characters-to-test-limits","namespace":"very-long-namespace-name"},"spec":{"clusterName":"very-long-cluster-name"}}`))
	f.Add([]byte(`{"metadata":{"name":"pool-special-chars","namespace":"default"},"spec":{"clusterName":"cluster-prod-123"}}`))
	f.Add([]byte(`{"metadata":{"name":"pool1"},"spec":{"pools":[{"start":"10.0.0.1","end":"10.0.0.255"}],"clusterName":"cluster1"}}`))
	f.Add([]byte(`{"metadata":{"name":"dual-stack-pool"},"spec":{"pools":[{"start":"192.168.1.1","end":"192.168.1.10"},{"start":"2001:db8::1","end":"2001:db8::10"}],"clusterName":"dual"}}`))
	f.Add([]byte(`{"metadata":{"name":"empty-namespace","namespace":""},"spec":{"clusterName":"test","namePrefix":"claim-"}}`))
	f.Add([]byte(`{"metadata":{"name":"pool.with.dots","namespace":"ns-with-dash"},"spec":{"clusterName":"cluster_underscore"}}`))

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
