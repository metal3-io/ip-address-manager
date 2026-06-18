package e2e

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool
	nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)
)

func Logf(format string, a ...any) {
	fmt.Fprintf(GinkgoWriter, "INFO: "+format+"\n", a...)
}

// testNamespace generates a valid Kubernetes namespace name based on the current It spec name.
// Uses a prefix + sanitized spec text + hash suffix to stay within the 63-char limit.
func testNamespace() string {
	specText := strings.TrimPrefix(strings.ToLower(CurrentSpecReport().LeafNodeText), "should ")
	cleaned := strings.Trim(nonAlphaNum.ReplaceAllString(specText, "-"), "-")
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(specText)))[:6]

	prefix := "e2e-"
	maxNameLen := 63 - len(prefix) - 1 - len(hash)
	if len(cleaned) > maxNameLen {
		cleaned = strings.TrimRight(cleaned[:maxNameLen], "-")
	}
	return fmt.Sprintf("%s%s-%s", prefix, cleaned, hash)
}

// cleanupNamespace deletes the test namespace, which cascade-deletes all resources within it.
// It honors the skipCleanup flag to preserve resources for debugging.
func cleanupNamespace(ctx context.Context, cl client.Client, namespace string) {
	if skipCleanup {
		Logf("Skipping cleanup of namespace %s (SKIP_RESOURCE_CLEANUP=true)", namespace)
		return
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if err := cl.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
		Logf("Warning: failed to delete namespace %s: %v", namespace, err)
		return
	}

	// Wait for namespace to be fully removed
	Eventually(func() bool {
		err := cl.Get(ctx, client.ObjectKeyFromObject(ns), &corev1.Namespace{})
		return apierrors.IsNotFound(err)
	}, "2m", "2s").Should(BeTrue(), "Namespace %s should be deleted", namespace)
}

// CreateIPPoolInput holds parameters for creating an IPPool in tests.
type CreateIPPoolInput struct {
	Name           string
	Namespace      string
	Start          string
	End            string
	Subnet         string
	Prefix         int
	Gateway        string
	DNSServers     []string
	NamePrefix     string
	PreAllocations map[string]ipamv1.IPAddressStr
	ClusterName    string
}

// createIPPool creates an IPPool resource from the given config.
func createIPPool(ctx context.Context, clusterProxy framework.ClusterProxy, cfg CreateIPPoolInput) *ipamv1.IPPool {
	Logf("Creating IPPool %s in namespace %s (range %s–%s)", cfg.Name, cfg.Namespace, cfg.Start, cfg.End)

	startAddr := ipamv1.IPAddressStr(cfg.Start)
	endAddr := ipamv1.IPAddressStr(cfg.End)
	subnet := ipamv1.IPSubnetStr(cfg.Subnet)

	pool := ipamv1.Pool{
		Start:  &startAddr,
		End:    &endAddr,
		Subnet: &subnet,
		Prefix: cfg.Prefix,
	}

	if cfg.Gateway != "" {
		gw := ipamv1.IPAddressStr(cfg.Gateway)
		pool.Gateway = &gw
	}

	for _, dns := range cfg.DNSServers {
		pool.DNSServers = append(pool.DNSServers, ipamv1.IPAddressStr(dns))
	}

	spec := ipamv1.IPPoolSpec{
		Pools:      []ipamv1.Pool{pool},
		Prefix:     cfg.Prefix,
		NamePrefix: cfg.NamePrefix,
	}

	if cfg.Gateway != "" {
		gw := ipamv1.IPAddressStr(cfg.Gateway)
		spec.Gateway = &gw
	}

	for _, dns := range cfg.DNSServers {
		spec.DNSServers = append(spec.DNSServers, ipamv1.IPAddressStr(dns))
	}

	if cfg.PreAllocations != nil {
		spec.PreAllocations = cfg.PreAllocations
	}

	if cfg.ClusterName != "" {
		spec.ClusterName = &cfg.ClusterName
	}

	ipPool := &ipamv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
		},
		Spec: spec,
	}

	Expect(clusterProxy.GetClient().Create(ctx, ipPool)).To(Succeed())
	Logf("Successfully created IPPool %s/%s", ipPool.Namespace, ipPool.Name)

	return ipPool
}

// createIPClaim creates a Metal3 IPClaim referencing the given pool name.
func createIPClaim(ctx context.Context, clusterProxy framework.ClusterProxy, poolName, claimName, namespace string) *ipamv1.IPClaim {
	Logf("Creating IPClaim in namespace %s for pool %s", namespace, poolName)

	ipClaim := &ipamv1.IPClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: namespace,
		},
		Spec: ipamv1.IPClaimSpec{
			Pool: corev1.ObjectReference{
				Name:      poolName,
				Namespace: namespace,
			},
		},
	}

	Expect(clusterProxy.GetClient().Create(ctx, ipClaim)).To(Succeed())
	Logf("Successfully created IPClaim %s/%s", ipClaim.Namespace, ipClaim.Name)

	return ipClaim
}

// createCAPIIPAddressClaim creates a CAPI IPAddressClaim (ipam.cluster.x-k8s.io/v1beta2)
// referencing a Metal3 IPPool.
func createCAPIIPAddressClaim(ctx context.Context, clusterProxy framework.ClusterProxy, poolName, claimName, namespace string) *capipamv1.IPAddressClaim {
	Logf("Creating CAPI IPAddressClaim in namespace %s for pool %s", namespace, poolName)

	claim := &capipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: namespace,
		},
		Spec: capipamv1.IPAddressClaimSpec{
			PoolRef: capipamv1.IPPoolReference{
				Name:     poolName,
				Kind:     "IPPool",
				APIGroup: "ipam.metal3.io",
			},
		},
	}

	Expect(clusterProxy.GetClient().Create(ctx, claim)).To(Succeed())
	Logf("Successfully created CAPI IPAddressClaim %s/%s", claim.Namespace, claim.Name)

	return claim
}
