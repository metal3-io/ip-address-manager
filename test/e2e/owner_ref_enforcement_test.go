package e2e

import (
	"context"
	"net"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test verifies that IPAM correctly sets ownerReferences when the
// OwnerReferencesPermissionEnforcement admission plugin is enabled.
// This plugin requires controllers to have delete permission to set ownerReferences.
// See: https://github.com/metal3-io/baremetal-operator/issues/3304
//
// The CAPI test framework enables the OwnerReferencesPermissionEnforcement admission
// plugin on the bootstrap kind cluster by default, so these tests run against the
// shared bootstrapClusterProxy without needing a dedicated cluster.
// See: https://github.com/kubernetes-sigs/cluster-api/pull/13805 and
// test/framework/bootstrap/kind_provider.go (createKindCluster).
//
// NOTE: This assumption only holds when the CAPI framework creates the bootstrap
// cluster. With -e2e.use-existing-cluster, SetupBootstrapCluster skips framework Kind
// creation and the supplied API server may not enable this admission plugin, so the
// spec could pass without actually exercising it. In that mode we skip these specs.

// skipIfPluginNotGuaranteed skips the spec when we cannot guarantee that the
// OwnerReferencesPermissionEnforcement admission plugin is enabled on the cluster.
func skipIfPluginNotGuaranteed() {
	if useExistingCluster {
		Skip("Skipping OwnerReferencesPermissionEnforcement specs: with -e2e.use-existing-cluster " +
			"the admission plugin is not guaranteed to be enabled on the provided cluster")
	}
}

var _ = Describe("IPAM with OwnerReferencesPermissionEnforcement", Label("ipam", "rbac"), func() {
	var (
		namespace       string
		ownerRefCluster *clusterv1.Cluster
	)

	BeforeEach(func() {
		skipIfPluginNotGuaranteed()
		namespace = testNamespace()
		ownerRefCluster = nil
		validateGlobals()
		cl := bootstrapClusterProxy.GetClient()
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		err := cl.Create(ctx, ns)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	AfterEach(func() {
		// The CAPI core controller adds the "cluster.cluster.x-k8s.io" finalizer to the
		// Cluster. Because this Cluster points at a non-existent infrastructure object and
		// is never fully reconciled, its deletion can stall, which would block namespace
		// teardown. Delete it and force-remove its finalizer BEFORE cleanupNamespace so the
		// namespace can actually be deleted.
		//
		// Only do this when cleanup is enabled: when SKIP_RESOURCE_CLEANUP=true we must
		// leave the Cluster (and the pool it can garbage-collect) intact so failed runs
		// remain inspectable, matching cleanupNamespace's behavior.
		if ownerRefCluster != nil && !skipCleanup {
			By("Deleting the CAPI Cluster and clearing its finalizers")
			deleteClusterAndRemoveFinalizer(ctx, bootstrapClusterProxy.GetClient(), ownerRefCluster)
		}
		cleanupNamespace(ctx, bootstrapClusterProxy, namespace, artifactFolder, clusterctlConfigPath)
	})

	It("Should allocate IPs with valid ownerReferences", func() {
		cl := bootstrapClusterProxy.GetClient()

		By("Creating a CAPI Cluster as ownerReference target")
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ownerref-test-cluster",
				Namespace: namespace,
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "GenericInfrastructureCluster",
					Name:     "ownerref-test-infra",
				},
			},
		}
		Expect(cl.Create(ctx, cluster)).To(Succeed())
		// Record it so AfterEach tears it down before the namespace is deleted.
		ownerRefCluster = cluster

		By("Creating an IPPool with ClusterName set")
		ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
			Name:        "test-pool",
			Namespace:   namespace,
			Start:       "192.168.10.10",
			End:         "192.168.10.100",
			Subnet:      "192.168.10.0/24",
			Prefix:      24,
			Gateway:     "192.168.10.1",
			DNSServers:  []string{"8.8.8.8"},
			NamePrefix:  "ownerref-ip",
			ClusterName: cluster.Name,
		})

		By("Creating an IPClaim")
		ipClaim := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "test-claim", namespace)

		By("Waiting for IPClaim to get allocated address")
		Eventually(func(g Gomega) {
			retrieved := &ipamv1.IPClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.Address).ToNot(BeNil())
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying IPPool has Cluster ownerReference")
		Eventually(func(g Gomega) {
			pool := &ipamv1.IPPool{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipPool), pool)).To(Succeed())
			g.Expect(pool.OwnerReferences).ToNot(BeEmpty(), "IPPool should have ownerReference")
			g.Expect(pool.OwnerReferences).To(ContainElement(SatisfyAll(
				HaveField("Kind", Equal("Cluster")),
				HaveField("Name", Equal(cluster.Name)),
			)), "IPPool should have Cluster ownerReference")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying IPAddress has ownerReferences")
		updatedClaim := &ipamv1.IPClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), updatedClaim)).To(Succeed())

		ipAddr := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: updatedClaim.Status.Address.Namespace,
			Name:      updatedClaim.Status.Address.Name,
		}, ipAddr)).To(Succeed())

		Expect(ipAddr.OwnerReferences).ToNot(BeEmpty(), "IPAddress should have ownerReferences")
		Expect(net.ParseIP(string(ipAddr.Spec.Address))).ToNot(BeNil(), "Should be valid IP")
	})
})

// deleteClusterAndRemoveFinalizer deletes the given CAPI Cluster and force-removes
// its finalizers so the object is garbage-collected promptly.
func deleteClusterAndRemoveFinalizer(ctx context.Context, cl client.Client, cluster *clusterv1.Cluster) {
	// Best-effort delete; ignore if already gone.
	if err := cl.Delete(ctx, cluster); err != nil && !apierrors.IsNotFound(err) {
		Logf("Warning: failed to delete Cluster %s/%s: %v", cluster.Namespace, cluster.Name, err)
	}

	// Clear finalizers so the Cluster can be removed even if its controller never
	// completes deletion.
	Eventually(func(g Gomega) {
		fresh := &clusterv1.Cluster{}
		err := cl.Get(ctx, client.ObjectKeyFromObject(cluster), fresh)
		if apierrors.IsNotFound(err) {
			return
		}
		g.Expect(err).NotTo(HaveOccurred())
		if len(fresh.Finalizers) > 0 {
			fresh.Finalizers = nil
			g.Expect(cl.Update(ctx, fresh)).To(Succeed())
		}
	}, "60s", "2s").Should(Succeed())

	// Wait for the Cluster to be fully removed before namespace cleanup proceeds.
	Eventually(func() bool {
		return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(cluster), &clusterv1.Cluster{}))
	}, "60s", "2s").Should(BeTrue(), "Cluster %s/%s should be deleted", cluster.Namespace, cluster.Name)
}

var _ = Describe("IPClaim with pre-existing ownerReference", Label("ipam", "rbac"), func() {
	var namespace string

	BeforeEach(func() {
		skipIfPluginNotGuaranteed()
		namespace = testNamespace()
		validateGlobals()
		cl := bootstrapClusterProxy.GetClient()
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		err := cl.Create(ctx, ns)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	AfterEach(func() {
		cleanupNamespace(ctx, bootstrapClusterProxy, namespace, artifactFolder, clusterctlConfigPath)
	})

	It("Should allocate an IP when IPClaim has an ownerReference", func() {
		cl := bootstrapClusterProxy.GetClient()

		By("Creating an IPPool")
		ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
			Name:       "test-pool-ownerref",
			Namespace:  namespace,
			Start:      "192.168.5.10",
			End:        "192.168.5.100",
			Subnet:     "192.168.5.0/24",
			Prefix:     24,
			Gateway:    "192.168.5.1",
			DNSServers: []string{"8.8.8.8"},
			NamePrefix: "ownerref-test",
		})

		By("Creating a ConfigMap to serve as owner")
		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-cm",
				Namespace: namespace,
			},
			Data: map[string]string{"key": "value"},
		}
		Expect(cl.Create(ctx, owner)).To(Succeed())

		// Re-fetch to get the UID
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(owner), owner)).To(Succeed())

		By("Creating an IPClaim with an ownerReference to the ConfigMap")
		ipClaim := &ipamv1.IPClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "claim-with-ownerref",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Spec: ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name:      ipPool.Name,
					Namespace: namespace,
				},
			},
		}
		Expect(cl.Create(ctx, ipClaim)).To(Succeed())

		By("Waiting for IPClaim to get an allocated address")
		Eventually(func(g Gomega) {
			retrieved := &ipamv1.IPClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.Address).ToNot(BeNil(), "IPClaim with ownerReference should get an address allocated")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying the IPAddress was created")
		updatedClaim := &ipamv1.IPClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), updatedClaim)).To(Succeed())

		ipAddr := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: updatedClaim.Status.Address.Namespace,
			Name:      updatedClaim.Status.Address.Name,
		}, ipAddr)).To(Succeed())
		Expect(net.ParseIP(string(ipAddr.Spec.Address))).ToNot(BeNil(), "Should be valid IP")
	})

	It("Should allocate an IP when IPClaim has a controller ownerReference", func() {
		cl := bootstrapClusterProxy.GetClient()

		By("Creating an IPPool")
		ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
			Name:       "test-pool-ctrl-ownerref",
			Namespace:  namespace,
			Start:      "192.168.6.10",
			End:        "192.168.6.100",
			Subnet:     "192.168.6.0/24",
			Prefix:     24,
			Gateway:    "192.168.6.1",
			DNSServers: []string{"8.8.8.8"},
			NamePrefix: "ctrl-ownerref",
		})

		By("Creating a ConfigMap to serve as controller owner")
		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ctrl-owner-cm",
				Namespace: namespace,
			},
			Data: map[string]string{"key": "value"},
		}
		Expect(cl.Create(ctx, owner)).To(Succeed())
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(owner), owner)).To(Succeed())

		By("Creating an IPClaim with controller: true ownerReference")
		trueVal := true
		ipClaim := &ipamv1.IPClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "claim-ctrl-ownerref",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "v1",
						Kind:               "ConfigMap",
						Name:               owner.Name,
						UID:                owner.UID,
						Controller:         &trueVal,
						BlockOwnerDeletion: &trueVal,
					},
				},
			},
			Spec: ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name:      ipPool.Name,
					Namespace: namespace,
				},
			},
		}
		Expect(cl.Create(ctx, ipClaim)).To(Succeed())

		By("Waiting for IPClaim to get an allocated address")
		Eventually(func(g Gomega) {
			retrieved := &ipamv1.IPClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.Address).ToNot(BeNil(), "IPClaim with controller ownerReference should get an address allocated")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying the IPAddress was created")
		updatedClaim := &ipamv1.IPClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), updatedClaim)).To(Succeed())

		ipAddr := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: updatedClaim.Status.Address.Namespace,
			Name:      updatedClaim.Status.Address.Name,
		}, ipAddr)).To(Succeed())
		Expect(net.ParseIP(string(ipAddr.Spec.Address))).ToNot(BeNil(), "Should be valid IP")
	})
})
