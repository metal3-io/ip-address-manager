/*
Copyright 2024 The Metal3 Authors.

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

package e2e

import (
	"path/filepath"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Note: This test should be changed during "prepare main branch", it should test n-2 => current and n-1 => current.
var _ = Describe("When testing IPAM provider upgrade", Label("ipam", "upgrade"), func() {
	var (
		upgradeClusterProvider bootstrap.ClusterProvider
		upgradeClusterProxy    framework.ClusterProxy
		namespace              string
	)

	BeforeEach(func() {
		namespace = testNamespace()
		validateGlobals()
	})

	AfterEach(func() {
		if skipCleanup {
			return
		}
		if upgradeClusterProxy != nil {
			upgradeClusterProxy.Dispose(ctx)
		}
		if upgradeClusterProvider != nil {
			upgradeClusterProvider.Dispose(ctx)
		}
	})

	// setupUpgradeCluster creates a Kind cluster and initializes it with the given IPAM version.
	setupUpgradeCluster := func(clusterName, ipamVersion string) {
		By("Creating a Kind cluster for the upgrade test")
		upgradeClusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               clusterName,
			RequiresDockerSock: e2eConfig.HasDockerProvider(),
			Images:             e2eConfig.Images,
			LogFolder:          filepath.Join(artifactFolder, "clusters", clusterName),
		})
		Expect(upgradeClusterProvider).ToNot(BeNil(), "Failed to create Kind cluster")

		kubeconfigPath := upgradeClusterProvider.GetKubeconfigPath()
		Expect(kubeconfigPath).To(BeAnExistingFile())

		upgradeClusterProxy = framework.NewClusterProxy(clusterName, kubeconfigPath, initScheme())
		Expect(upgradeClusterProxy).ToNot(BeNil())

		By("Initializing IPAM provider with old version: " + ipamVersion)
		clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
			ClusterProxy:            upgradeClusterProxy,
			ClusterctlConfigPath:    clusterctlConfigPath,
			InfrastructureProviders: e2eConfig.InfrastructureProviders(),
			IPAMProviders:           []string{"metal3:" + ipamVersion},
			LogFolder:               filepath.Join(artifactFolder, "clusters", clusterName),
		}, e2eConfig.GetIntervals("default", "wait-controllers")...)
	}

	// runUpgradeTest creates IPAM resources with the old version, upgrades to current, and verifies.
	runUpgradeTest := func(clusterName, oldIPAMVersion string) {
		setupUpgradeCluster(clusterName, oldIPAMVersion)
		cl := upgradeClusterProxy.GetClient()

		By("Creating test namespace on upgrade cluster")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		err := cl.Create(ctx, ns)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		By("Creating IPPool with old IPAM version")
		ipPool := createIPPool(ctx, upgradeClusterProxy, CreateIPPoolInput{
			Name:       "upgrade-test-pool",
			Namespace:  namespace,
			Start:      "192.168.100.10",
			End:        "192.168.100.50",
			Subnet:     "192.168.100.0/24",
			Prefix:     24,
			Gateway:    "192.168.100.1",
			DNSServers: []string{"8.8.8.8"},
			NamePrefix: "upgrade-test",
		})

		By("Creating IPClaim with old IPAM version")
		ipClaim := createIPClaim(ctx, upgradeClusterProxy, ipPool.Name, "upgrade-test-claim", namespace)

		By("Creating CAPI IPAddressClaim with old IPAM version")
		capiClaim := createCAPIIPAddressClaim(ctx, upgradeClusterProxy, ipPool.Name, "upgrade-capi-claim", namespace)

		By("Waiting for IPClaim to be allocated")
		Eventually(func(g Gomega) {
			retrieved := &ipamv1.IPClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.Address).ToNot(BeNil(), "IPClaim should have an address allocated")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Waiting for CAPI IPAddressClaim to be allocated")
		Eventually(func(g Gomega) {
			retrieved := &capipamv1.IPAddressClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(capiClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.AddressRef.Name).ToNot(BeEmpty(), "CAPI IPAddressClaim should have addressRef")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Recording pre-upgrade allocations")
		preUpgradeClaim := &ipamv1.IPClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), preUpgradeClaim)).To(Succeed())
		preUpgradeAddr := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: preUpgradeClaim.Status.Address.Namespace,
			Name:      preUpgradeClaim.Status.Address.Name,
		}, preUpgradeAddr)).To(Succeed())
		preUpgradeIP := string(preUpgradeAddr.Spec.Address)

		preUpgradeCAPIClaim := &capipamv1.IPAddressClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(capiClaim), preUpgradeCAPIClaim)).To(Succeed())
		preUpgradeCAPIAddr := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      preUpgradeCAPIClaim.Status.AddressRef.Name,
		}, preUpgradeCAPIAddr)).To(Succeed())
		preUpgradeCAPIIP := preUpgradeCAPIAddr.Spec.Address

		Logf("Pre-upgrade: Metal3 IPClaim -> %s, CAPI IPAddressClaim -> %s", preUpgradeIP, preUpgradeCAPIIP)

		By("Upgrading the IPAM provider to current version")
		clusterctl.UpgradeManagementClusterAndWait(ctx, clusterctl.UpgradeManagementClusterAndWaitInput{
			ClusterProxy:         upgradeClusterProxy,
			ClusterctlConfigPath: clusterctlConfigPath,
			IPAMProviders:        e2eConfig.GetProviderLatestVersionsByContract("v1beta2", "metal3"),
			LogFolder:            filepath.Join(artifactFolder, "clusters", clusterName),
		}, e2eConfig.GetIntervals("default", "wait-controllers")...)

		By("Verifying IPPool still exists after upgrade")
		postPool := &ipamv1.IPPool{}
		Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "upgrade-test-pool"}, postPool)).To(Succeed())
		Expect(postPool.Spec.Pools).To(HaveLen(1))
		Expect(postPool.Spec.Prefix).To(Equal(24))

		By("Verifying IPClaim still has its allocation after upgrade")
		postClaim := &ipamv1.IPClaim{}
		Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "upgrade-test-claim"}, postClaim)).To(Succeed())
		Expect(postClaim.Status.Address).ToNot(BeNil(), "IPClaim should still have address after upgrade")

		By("Verifying IPAddress is preserved with same IP after upgrade")
		postAddr := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: postClaim.Status.Address.Namespace,
			Name:      postClaim.Status.Address.Name,
		}, postAddr)).To(Succeed())
		Expect(string(postAddr.Spec.Address)).To(Equal(preUpgradeIP), "IP address should be preserved after upgrade")
		Expect(postAddr.Spec.Pool.Name).To(Equal("upgrade-test-pool"))

		By("Verifying CAPI IPAddressClaim still has its allocation after upgrade")
		postCAPIClaim := &capipamv1.IPAddressClaim{}
		Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "upgrade-capi-claim"}, postCAPIClaim)).To(Succeed())
		Expect(postCAPIClaim.Status.AddressRef.Name).ToNot(BeEmpty(), "CAPI IPAddressClaim should still have addressRef after upgrade")

		By("Verifying CAPI IPAddress is preserved with same IP after upgrade")
		postCAPIAddr := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      postCAPIClaim.Status.AddressRef.Name,
		}, postCAPIAddr)).To(Succeed())
		Expect(postCAPIAddr.Spec.Address).To(Equal(preUpgradeCAPIIP), "CAPI IP address should be preserved after upgrade")
		Expect(postCAPIAddr.Spec.PoolRef.Name).To(Equal("upgrade-test-pool"))
		Expect(postCAPIAddr.Spec.PoolRef.APIGroup).To(Equal("ipam.metal3.io"))
		Expect(postCAPIAddr.Spec.ClaimRef.Name).To(Equal("upgrade-capi-claim"))

		By("Verifying new allocations work after upgrade")
		newClaim := &ipamv1.IPClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade-post-claim",
				Namespace: namespace,
			},
			Spec: ipamv1.IPClaimSpec{
				Pool: corev1.ObjectReference{
					Name:      "upgrade-test-pool",
					Namespace: namespace,
				},
			},
		}
		// Use Eventually for Create because the webhook may not be ready immediately after upgrade.
		Eventually(func() error {
			return cl.Create(ctx, newClaim)
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		Eventually(func(g Gomega) {
			retrieved := &ipamv1.IPClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(newClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.Address).ToNot(BeNil(), "New IPClaim should get allocated after upgrade")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying IPPool status shows all allocations")
		Eventually(func(g Gomega) {
			pool := &ipamv1.IPPool{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "upgrade-test-pool"}, pool)).To(Succeed())
			g.Expect(pool.Status.Allocations).To(HaveLen(3), "Should have 3 allocations: Metal3 claim + CAPI claim + post-upgrade claim")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying new allocation got a different IP")
		newClaimRetrieved := &ipamv1.IPClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(newClaim), newClaimRetrieved)).To(Succeed())
		newAddr := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: newClaimRetrieved.Status.Address.Namespace,
			Name:      newClaimRetrieved.Status.Address.Name,
		}, newAddr)).To(Succeed())
		Expect(string(newAddr.Spec.Address)).ToNot(Equal(preUpgradeIP), "New allocation should get a different IP")

		Logf("Upgrade test passed: pre-upgrade IPs preserved, new allocations working")
	}

	// Test upgrade from n-2 (v1.12) to current
	It("Should upgrade IPAM from v1.12 to current and preserve allocations", func() {
		// Use the .99 version available in the local artifact repository (built from
		// release branch kustomize overlay in e2e_conf.yaml).
		runUpgradeTest("ipam-upgrade-v1-12", "v1.12.99")
	})

	// Test upgrade from n-1 (v1.13) to current
	It("Should upgrade IPAM from v1.13 to current and preserve allocations", func() {
		// Use the .99 version available in the local artifact repository (built from
		// release branch kustomize overlay in e2e_conf.yaml).
		runUpgradeTest("ipam-upgrade-v1-13", "v1.13.99")
	})
})
