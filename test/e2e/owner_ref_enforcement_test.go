package e2e

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kindv1 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	kind "sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
)

// This test verifies that IPAM correctly sets ownerReferences when the
// OwnerReferencesPermissionEnforcement admission plugin is enabled.
// This plugin requires controllers to have delete permission to set ownerReferences.
// See: https://github.com/metal3-io/baremetal-operator/issues/3304

// The controller fetches the Cluster
// object via the controller-runtime cache. The cache requires list+watch RBAC on
// clusters.cluster.x-k8s.io. Without list+watch, the cache informer
// fails to sync and the controller cannot fetch the Cluster, causing it to return
// early without processing any IPClaims for that pool.

var _ = Describe("IPAM with OwnerReferencesPermissionEnforcement", Label("ipam", "rbac"), func() {
	var (
		clusterName    string
		kubeconfigPath string
		clusterProxy   framework.ClusterProxy
		kindProvider   *kind.Provider
	)

	BeforeEach(func() {
		validateGlobals()
		clusterName = "ipam-ownerref"

		By("Creating kind cluster with OwnerReferencesPermissionEnforcement enabled")
		kindProvider, kubeconfigPath = createKindClusterWithOwnerRefEnforcement(clusterName)

		By("Loading IPAM image into the kind cluster")
		err := bootstrap.LoadImagesToKindCluster(ctx, bootstrap.LoadImagesToKindClusterInput{
			Name:   clusterName,
			Images: e2eConfig.Images,
		})
		Expect(err).ToNot(HaveOccurred())

		By("Setting up cluster proxy")
		clusterProxy = framework.NewClusterProxy("ownerref-test", kubeconfigPath, initScheme())
		Expect(clusterProxy).ToNot(BeNil())

		By("Initializing IPAM on cluster")
		clusterctl.Init(ctx, clusterctl.InitInput{
			KubeconfigPath:        clusterProxy.GetKubeconfigPath(),
			ClusterctlConfigPath:  clusterctlConfigPath,
			CoreProvider:          "cluster-api",
			BootstrapProviders:    []string{"kubeadm"},
			ControlPlaneProviders: []string{"kubeadm"},
			IPAMProviders:         e2eConfig.IPAMProviders(),
			LogFolder:             filepath.Join(artifactFolder, "clusters", clusterName),
		})

		By("Waiting for IPAM controller")
		controllersDeployments := framework.GetControllerDeployments(ctx, framework.GetControllerDeploymentsInput{
			Lister: clusterProxy.GetClient(),
		})
		for _, deployment := range controllersDeployments {
			framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
				Getter:     clusterProxy.GetClient(),
				Deployment: deployment,
			}, e2eConfig.GetIntervals("default", "wait-controllers")...)
		}
	})

	AfterEach(func() {
		if skipCleanup {
			Logf("Skipping cleanup of kind cluster %s (SKIP_RESOURCE_CLEANUP=true)", clusterName)
			return
		}
		if clusterProxy != nil {
			clusterProxy.Dispose(ctx)
		}
		if kindProvider != nil {
			Expect(kindProvider.Delete(clusterName, kubeconfigPath)).To(Succeed())
		}
		if kubeconfigPath != "" {
			os.Remove(kubeconfigPath)
		}
	})

	It("Should allocate IPs with valid ownerReferences", func() {
		cl := clusterProxy.GetClient()
		namespace := testNamespace()

		By("Creating test namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		err := cl.Create(ctx, ns)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).ToNot(HaveOccurred())
		}

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

		By("Creating an IPPool with ClusterName set")
		ipPool := createIPPool(ctx, clusterProxy, CreateIPPoolInput{
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
		ipClaim := createIPClaim(ctx, clusterProxy, ipPool.Name, "test-claim", namespace)

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

var _ = Describe("IPClaim with pre-existing ownerReference", Label("ipam", "rbac"), func() {
	var namespace string

	BeforeEach(func() {
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
		cleanupNamespace(ctx, bootstrapClusterProxy.GetClient(), namespace)
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

// createKindClusterWithOwnerRefEnforcement creates a kind cluster with the
// OwnerReferencesPermissionEnforcement admission plugin enabled.
func createKindClusterWithOwnerRefEnforcement(name string) (*kind.Provider, string) {
	kubeconfigFile, err := os.CreateTemp("", "e2e-ownerref-kind-")
	Expect(err).ToNot(HaveOccurred())
	kubeconfigPath := kubeconfigFile.Name()
	kubeconfigFile.Close()

	cfg := &kindv1.Cluster{
		Nodes: []kindv1.Node{
			{
				Role: kindv1.ControlPlaneRole,
				KubeadmConfigPatches: []string{
					`kind: ClusterConfiguration
apiServer:
  extraArgs:
    enable-admission-plugins: "NodeRestriction,OwnerReferencesPermissionEnforcement"`,
				},
			},
		},
	}
	kindv1.SetDefaultsCluster(cfg)

	provider := kind.NewProvider(kind.ProviderWithLogger(cmd.NewLogger()))
	err = provider.Create(
		name,
		kind.CreateWithKubeconfigPath(kubeconfigPath),
		kind.CreateWithV1Alpha4Config(cfg),
		kind.CreateWithNodeImage(fmt.Sprintf("%s:%s", bootstrap.DefaultNodeImageRepository, bootstrap.DefaultNodeImageVersion)),
		kind.CreateWithRetain(true),
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(kubeconfigPath).To(BeAnExistingFile())

	return provider, kubeconfigPath
}
