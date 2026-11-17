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
	"context"
	"fmt"
	"path/filepath"
	"sort"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This test verifies that IPAM objects (IPPool, Metal3-IPClaim, Metal3-IPAddress,
// CAPI-IPAddressClaim and CAPI-IPAddress) survive a
// `clusterctl move` operation between two clusters. This simulates the
// pivoting workflow used in production to move management resources from a
// bootstrap cluster to a target cluster.
//
// The test validates:
// - Metal3 IPPool/IPClaim/IPAddress and CAPI IPAddressClaim/IPAddress
// objects are correctly moved with all information preserved
// - Finalizers are preserved and functional after move.
// - Owner references remain valid in the target cluster.
// - Allocations in IPPool status are consistent after move.
// - The IPAM controller on the target cluster can reconcile moved objects.
// - New allocations work from moved pools on the target cluster.
// - Objects are removed from the source cluster after move.
//
// In the main move spec the IPPool is owned (spec.clusterName) by the Cluster,
// so the IPPool is also paused when Cluster is paused when pivoting. This stops
// target IPAM controller to not allocate new IPAddresses until unpaused.
// Random allocation is used instead of sequential to make the test more robust,
// because random allocation makes it very unlikely that a wrong reallocation
// lands on the same IPs, so the test can tell "moved and reused" apart from
// "allocated again". With sequential allocation, a broken pivot can still look
// correct.
// Source-UID marker is added on each IPAddress that ensures that re-allocation
// is detected, even if it happens to reuse the same IP.
var _ = Describe("When testing clusterctl move of IPAM resources", Label("ipam", "pivot"), func() {
	var (
		targetClusterProvider bootstrap.ClusterProvider
		targetClusterProxy    framework.ClusterProxy
		namespace             string
		targetCluster         *clusterv1.Cluster
	)

	BeforeEach(func() {
		namespace = testNamespace()
		targetCluster = nil
		targetClusterProvider = nil
		targetClusterProxy = nil
		validateGlobals()
		cl := bootstrapClusterProxy.GetClient()
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		err := cl.Create(ctx, ns)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	})

	AfterEach(func() {
		// The Cluster carries the CAPI core finalizer and is never fully provisioned,
		// so delete it and clear its finalizers before tearing down the namespace.
		// Depending on where the spec stopped, it lives on the bootstrap or the
		// target cluster, so try both (NotFound is tolerated).
		if targetCluster != nil && !skipCleanup {
			By("Deleting the target CAPI Cluster object and clearing its finalizers")
			if targetClusterProxy != nil {
				deleteClusterAndRemoveFinalizer(ctx, targetClusterProxy.GetClient(), targetCluster)
			}
			deleteClusterAndRemoveFinalizer(ctx, bootstrapClusterProxy.GetClient(), targetCluster)
		}
		// cleanupNamespace always dumps resources to artifacts first, then honors
		// skipCleanup for the namespace deletion.
		if targetClusterProxy != nil {
			cleanupNamespace(ctx, targetClusterProxy, namespace, artifactFolder, clusterctlConfigPath)
		}
		if bootstrapClusterProxy != nil {
			cleanupNamespace(ctx, bootstrapClusterProxy, namespace, artifactFolder, clusterctlConfigPath)
		}
		if skipCleanup {
			return
		}
		if targetClusterProxy != nil {
			targetClusterProxy.Dispose(ctx)
		}
		if targetClusterProvider != nil {
			targetClusterProvider.Dispose(ctx)
		}
	})

	It("Should move IPPool, IPClaim and IPAddressClaim resources to a target cluster", func() {
		cl := bootstrapClusterProxy.GetClient()

		By("Creating the CAPI Cluster representing the target cluster on the bootstrap cluster")
		targetCluster = createPivotCluster(ctx, cl, namespace, moveTargetClusterName)

		By("Marking the Cluster as provisioned so clusterctl move accepts it")
		markClusterReadyForMove(ctx, cl, targetCluster)

		By("Creating an IPPool with clusterName and random allocation on the bootstrap cluster")
		ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
			Name:               "move-test-pool",
			Namespace:          namespace,
			Start:              "10.0.0.10",
			End:                "10.0.0.50",
			Subnet:             "10.0.0.0/24",
			Prefix:             24,
			Gateway:            "10.0.0.1",
			DNSServers:         []string{"8.8.8.8", "8.8.4.4"},
			NamePrefix:         "move-test",
			ClusterName:        targetCluster.Name,
			AllocationStrategy: ipamv1.AllocationStrategyRandom,
		})

		By("Verifying the IPPool is owned by the target Cluster object on the bootstrap cluster")
		Eventually(func(g Gomega) {
			pool := &ipamv1.IPPool{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipPool), pool)).To(Succeed())
			g.Expect(pool.OwnerReferences).To(ContainElement(SatisfyAll(
				HaveField("Kind", Equal("Cluster")),
				HaveField("Name", Equal(targetCluster.Name)),
				HaveField("UID", Equal(targetCluster.UID)),
			)), "IPPool should be owned by target Cluster %s so it follows the Cluster pause state", targetCluster.Name)
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Labeling IPPool for clusterctl move")
		labelPoolForMove(ctx, cl, ipPool)

		By(fmt.Sprintf("Creating %d CAPI IPAddressClaims on the bootstrap cluster", pivotCAPIClaimCount))
		capiClaims := make([]*capipamv1.IPAddressClaim, 0, pivotCAPIClaimCount)
		for i := 1; i <= pivotCAPIClaimCount; i++ {
			capiClaims = append(capiClaims, createCAPIIPAddressClaim(ctx, bootstrapClusterProxy, ipPool.Name, fmt.Sprintf("move-capi-claim-%d", i), namespace))
		}

		By(fmt.Sprintf("Creating %d Metal3 IPClaims on the bootstrap cluster", pivotM3ClaimCount))
		m3Claims := make([]*ipamv1.IPClaim, 0, pivotM3ClaimCount)
		for i := 1; i <= pivotM3ClaimCount; i++ {
			m3Claims = append(m3Claims, createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, fmt.Sprintf("move-m3-claim-%d", i), namespace))
		}

		By("Waiting for all claims to be allocated")
		for _, c := range capiClaims {
			waitForCAPIClaimAllocated(ctx, cl, c)
		}
		for _, c := range m3Claims {
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(c), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil(), "IPClaim %s should be allocated", c.Name)
				g.Expect(retrieved.Status.Address.Name).ToNot(BeEmpty())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
		}

		By("Labeling claims for clusterctl move")
		for _, c := range capiClaims {
			labelIPAddressClaimForMove(ctx, cl, c)
		}
		for _, c := range m3Claims {
			labelClaimForMove(ctx, cl, c)
		}

		By("Recording pre-move state")
		preMovePool := &ipamv1.IPPool{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipPool), preMovePool)).To(Succeed())
		Expect(preMovePool.Status.Allocations).To(HaveLen(pivotClaimCount), "Should have %d allocations before move", pivotClaimCount)

		By("Recording the pre-move IPClaim -> IPAddress baseline and stamping IPAddresses with markers")
		baseline := recordPivotBaseline(ctx, cl, namespace, ipPool.Name)
		Expect(baseline.records).To(HaveLen(pivotClaimCount), "Baseline should contain one record per claim")

		By("Creating a target Kind cluster for the move")
		targetClusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               moveTargetClusterName,
			RequiresDockerSock: e2eConfig.HasDockerProvider(),
			Images:             e2eConfig.Images,
			LogFolder:          filepath.Join(artifactFolder, "clusters", moveTargetClusterName),
		})
		Expect(targetClusterProvider).ToNot(BeNil(), "Failed to create target Kind cluster")

		targetKubeconfigPath := targetClusterProvider.GetKubeconfigPath()
		Expect(targetKubeconfigPath).To(BeAnExistingFile())

		targetClusterProxy = framework.NewClusterProxy("move-target", targetKubeconfigPath, initScheme())
		Expect(targetClusterProxy).ToNot(BeNil())

		By("Initializing the IPAM provider on the target cluster")
		clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
			ClusterProxy:            targetClusterProxy,
			ClusterctlConfigPath:    clusterctlConfigPath,
			InfrastructureProviders: e2eConfig.InfrastructureProviders(),
			IPAMProviders:           e2eConfig.IPAMProviders(),
			LogFolder:               filepath.Join(artifactFolder, "clusters", moveTargetClusterName),
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-controllers")...)

		By("Ensuring the target namespace exists on the target cluster")
		targetNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		Expect(targetClusterProxy.GetClient().Create(ctx, targetNs)).To(Succeed())

		dumpPivotSnapshot(ctx, bootstrapClusterProxy, namespace, "beforePivot")

		By("Moving IPAM resources from bootstrap to target cluster")
		clusterctl.Move(ctx, clusterctl.MoveInput{
			LogFolder:            filepath.Join(artifactFolder, "clusters", "ipam-move"),
			ClusterctlConfigPath: clusterctlConfigPath,
			FromKubeconfigPath:   bootstrapClusterProxy.GetKubeconfigPath(),
			ToKubeconfigPath:     targetClusterProxy.GetKubeconfigPath(),
			Namespace:            namespace,
		})

		targetClient := targetClusterProxy.GetClient()

		By("Verifying the target Cluster object was moved onto the target cluster, unpaused and still owns the IPPool")
		Eventually(func(g Gomega) {
			movedCluster := &clusterv1.Cluster{}
			g.Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(targetCluster), movedCluster)).To(Succeed())
			g.Expect(movedCluster.Spec.Paused).To(Or(BeNil(), HaveValue(BeFalse())),
				"Cluster should be unpaused on the target after the move")
			pool := &ipamv1.IPPool{}
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ipPool.Name}, pool)).To(Succeed())
			g.Expect(pool.OwnerReferences).To(ContainElement(SatisfyAll(
				HaveField("Kind", Equal("Cluster")),
				HaveField("Name", Equal(movedCluster.Name)),
				HaveField("UID", Equal(movedCluster.UID)),
			)), "moved IPPool should be owned by the moved target Cluster (UID %s), got %+v", movedCluster.UID, pool.OwnerReferences)
			g.Expect(pool.OwnerReferences).ToNot(ContainElement(HaveField("UID", Equal(targetCluster.UID))),
				"moved IPPool still references the pre-move (bootstrap-side) UID %s of the target Cluster", targetCluster.UID)
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(Succeed())

		By("Verifying moved IPClaims and IPAddresses are reused rather than re-allocated")
		Eventually(func(g Gomega) {
			expectPivotBaselinePreserved(ctx, g, targetClient, namespace, baseline)
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(Succeed())
		// Guard against a late re-allocation by the now-unpaused IPPool.
		Consistently(func(g Gomega) {
			expectPivotBaselinePreserved(ctx, g, targetClient, namespace, baseline)
		}, "15s", "3s").Should(Succeed())

		// Snapshot after the reuse checks and before the test creates any new claim,
		// so it can be diffed against beforePivot.
		dumpPivotSnapshot(ctx, targetClusterProxy, namespace, "afterPivot")

		By("Verifying IPPool exists on target cluster with correct spec")
		movedPool := &ipamv1.IPPool{}
		Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-test-pool"}, movedPool)).To(Succeed())
		Expect(movedPool.Spec.Pools).To(Equal(preMovePool.Spec.Pools))
		Expect(movedPool.Spec.Prefix).To(Equal(preMovePool.Spec.Prefix))
		Expect(movedPool.Spec.Gateway).To(Equal(preMovePool.Spec.Gateway))
		Expect(movedPool.Spec.DNSServers).To(Equal(preMovePool.Spec.DNSServers))
		Expect(movedPool.Spec.NamePrefix).To(Equal(preMovePool.Spec.NamePrefix))
		Expect(movedPool.Spec.ClusterName).To(Equal(preMovePool.Spec.ClusterName))
		Expect(movedPool.Spec.AllocationStrategy).To(Equal(ipamv1.AllocationStrategyRandom))

		By("Verifying IPPool status is reconciled on target cluster")
		Eventually(func(g Gomega) {
			pool := &ipamv1.IPPool{}
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-test-pool"}, pool)).To(Succeed())
			g.Expect(pool.Status.Allocations).To(HaveLen(pivotClaimCount), "Moved pool should have %d allocations", pivotClaimCount)
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(Succeed())

		By("Verifying new CAPI allocations work on the target cluster from the moved pool")
		newClaim := createCAPIIPAddressClaim(ctx, targetClusterProxy, "move-test-pool", "move-new-claim", namespace)

		Eventually(func(g Gomega) {
			retrieved := &capipamv1.IPAddressClaim{}
			g.Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(newClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.AddressRef.Name).ToNot(BeEmpty(), "New CAPI claim on target should get allocated")
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(Succeed())

		// Verify the new allocation is a unique IP (not a duplicate of moved IPs)
		newClaimRetrieved := &capipamv1.IPAddressClaim{}
		Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(newClaim), newClaimRetrieved)).To(Succeed())
		newAddr := &capipamv1.IPAddress{}
		Expect(targetClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      newClaimRetrieved.Status.AddressRef.Name,
		}, newAddr)).To(Succeed())
		newIP := newAddr.Spec.Address
		for _, r := range baseline.records {
			Expect(newIP).ToNot(Equal(r.ip), "New allocation %s duplicates the moved IP of %s %s", newIP, r.claimKind, r.claimName)
		}
		Expect(newAddr.Spec.PoolRef.Name).To(Equal(preMovePool.Name))
		Expect(newAddr.Spec.PoolRef.Kind).To(Equal("IPPool"))
		Expect(newAddr.Spec.PoolRef.APIGroup).To(Equal("ipam.metal3.io"))
		Expect(newAddr.Spec.ClaimRef.Name).To(Equal(newClaimRetrieved.Name))

		By("Verifying objects are removed from the source (bootstrap) cluster")
		Eventually(func() bool {
			return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-test-pool"}, &ipamv1.IPPool{}))
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPPool should be removed from source after move")

		Eventually(func(g Gomega) {
			m3Names, capiNames := listPoolClaimNames(ctx, g, cl, namespace, ipPool.Name)
			g.Expect(m3Names).To(BeEmpty(), "IPClaims should be removed from source after move")
			g.Expect(capiNames).To(BeEmpty(), "IPAddressClaims should be removed from source after move")
			m3Addrs, capiAddrs := listPoolAddressNames(ctx, g, cl, namespace, ipPool.Name)
			g.Expect(m3Addrs).To(BeEmpty(), "Metal3 IPAddresses should be removed from source after move")
			g.Expect(capiAddrs).To(BeEmpty(), "CAPI IPAddresses should be removed from source after move")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying IPPool finalizer is functional on target (deletion blocked while claims exist)")
		Expect(targetClient.Delete(ctx, movedPool)).To(Succeed())
		Consistently(func() bool {
			pool := &ipamv1.IPPool{}
			err := targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-test-pool"}, pool)
			if err != nil {
				return false
			}
			return pool.DeletionTimestamp != nil
		}, "5s", "1s").Should(BeTrue(), "IPPool should have DeletionTimestamp but still exist due to finalizer")

		By("Cleaning up claims on target to release pool finalizer")
		// Delete all claims so the pool can be cleaned up
		Expect(targetClient.Delete(ctx, newClaim)).To(Succeed())
		for _, c := range capiClaims {
			Expect(targetClient.Delete(ctx, &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{Name: c.Name, Namespace: namespace},
			})).To(Succeed())
		}
		for _, c := range m3Claims {
			Expect(targetClient.Delete(ctx, &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{Name: c.Name, Namespace: namespace},
			})).To(Succeed())
		}

		By("Verifying IPPool is eventually deleted on target after all claims removed")
		Eventually(func() bool {
			return apierrors.IsNotFound(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-test-pool"}, &ipamv1.IPPool{}))
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(BeTrue(), "IPPool should be deleted after all claims removed")

	})

	It("Should move an IPPool with preallocations to a target cluster", func() {
		cl := bootstrapClusterProxy.GetClient()

		By("Creating an IPPool with preallocations on the bootstrap cluster")
		prealloc1 := ipamv1.IPAddressStr("10.0.1.100")
		prealloc2 := ipamv1.IPAddressStr("10.0.1.200")
		ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
			Name:       "move-prealloc-pool",
			Namespace:  namespace,
			Start:      "10.0.1.10",
			End:        "10.0.1.250",
			Subnet:     "10.0.1.0/24",
			Prefix:     24,
			Gateway:    "10.0.1.1",
			NamePrefix: "move-prealloc",
			PreAllocations: map[string]ipamv1.IPAddressStr{
				"prealloc-claim-1": prealloc1,
				"prealloc-claim-2": prealloc2,
			},
		})

		By("Labeling IPPool for clusterctl move")
		labelPoolForMove(ctx, cl, ipPool)

		By("Creating CAPI IPAddressClaims that match the preallocations")
		claim1 := createCAPIIPAddressClaim(ctx, bootstrapClusterProxy, ipPool.Name, "prealloc-claim-1", namespace)
		claim2 := createCAPIIPAddressClaim(ctx, bootstrapClusterProxy, ipPool.Name, "prealloc-claim-2", namespace)

		By("Waiting for preallocated IPs to be assigned")
		for _, claim := range []*capipamv1.IPAddressClaim{claim1, claim2} {
			Eventually(func(g Gomega) {
				retrieved := &capipamv1.IPAddressClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.AddressRef.Name).ToNot(BeEmpty())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
		}

		By("Labeling CAPI IPAddressClaims for clusterctl move")
		labelIPAddressClaimForMove(ctx, cl, claim1)
		labelIPAddressClaimForMove(ctx, cl, claim2)

		By("Verifying preallocated IPs were assigned correctly")
		updatedClaim1 := &capipamv1.IPAddressClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim1), updatedClaim1)).To(Succeed())
		preMoveClaim1Addr := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      updatedClaim1.Status.AddressRef.Name,
		}, preMoveClaim1Addr)).To(Succeed())
		Expect(preMoveClaim1Addr.Spec.Address).To(Equal(string(prealloc1)), "claim1 should be assigned its configured preallocated IP before move")

		updatedClaim2 := &capipamv1.IPAddressClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), updatedClaim2)).To(Succeed())
		preMoveClaim2Addr := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      updatedClaim2.Status.AddressRef.Name,
		}, preMoveClaim2Addr)).To(Succeed())
		Expect(preMoveClaim2Addr.Spec.Address).To(Equal(string(prealloc2)), "claim2 should be assigned its configured preallocated IP before move")

		By("Recording pre-move state")
		preMovePool := &ipamv1.IPPool{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipPool), preMovePool)).To(Succeed())
		Expect(preMovePool.Status.Allocations).To(HaveLen(2), "Should have 2 allocations before move")

		By("Creating target Kind cluster")
		targetClusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               "ipam-move-prealloc-target",
			RequiresDockerSock: e2eConfig.HasDockerProvider(),
			Images:             e2eConfig.Images,
			LogFolder:          filepath.Join(artifactFolder, "clusters", "ipam-move-prealloc-target"),
		})
		Expect(targetClusterProvider).ToNot(BeNil())

		targetKubeconfigPath := targetClusterProvider.GetKubeconfigPath()
		targetClusterProxy = framework.NewClusterProxy("move-prealloc-target", targetKubeconfigPath, initScheme())

		By("Initializing IPAM provider on target cluster")
		clusterctl.InitManagementClusterAndWatchControllerLogs(ctx, clusterctl.InitManagementClusterAndWatchControllerLogsInput{
			ClusterProxy:            targetClusterProxy,
			ClusterctlConfigPath:    clusterctlConfigPath,
			InfrastructureProviders: e2eConfig.InfrastructureProviders(),
			IPAMProviders:           e2eConfig.IPAMProviders(),
			LogFolder:               filepath.Join(artifactFolder, "clusters", "ipam-move-prealloc-target"),
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-controllers")...)

		By("Creating target namespace on target cluster")
		targetNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(targetClusterProxy.GetClient().Create(ctx, targetNs)).To(Succeed())

		By("Moving resources to target cluster")
		clusterctl.Move(ctx, clusterctl.MoveInput{
			LogFolder:            filepath.Join(artifactFolder, "clusters", "ipam-move-prealloc"),
			ClusterctlConfigPath: clusterctlConfigPath,
			FromKubeconfigPath:   bootstrapClusterProxy.GetKubeconfigPath(),
			ToKubeconfigPath:     targetClusterProxy.GetKubeconfigPath(),
			Namespace:            namespace,
		})

		targetClient := targetClusterProxy.GetClient()

		By("Verifying preallocations are preserved on target")
		movedPool := &ipamv1.IPPool{}
		Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-prealloc-pool"}, movedPool)).To(Succeed())
		Expect(movedPool.Spec.PreAllocations).To(HaveLen(2))
		Expect(movedPool.Spec.PreAllocations["prealloc-claim-1"]).To(Equal(prealloc1))
		Expect(movedPool.Spec.PreAllocations["prealloc-claim-2"]).To(Equal(prealloc2))

		By("Verifying the preallocated IPAddresses have correct values on target")
		movedClaim1 := &capipamv1.IPAddressClaim{}
		Eventually(func(g Gomega) {
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "prealloc-claim-1"}, movedClaim1)).To(Succeed())
			g.Expect(movedClaim1.Status.AddressRef.Name).ToNot(BeEmpty(), "Moved claim should have address status")
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(Succeed())
		movedAddr1 := &capipamv1.IPAddress{}
		Expect(targetClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      movedClaim1.Status.AddressRef.Name,
		}, movedAddr1)).To(Succeed())
		Expect(movedAddr1.Spec.Address).To(Equal(preMoveClaim1Addr.Spec.Address), "IP address should be preserved after move")
		Expect(movedAddr1.Spec.PoolRef.Name).To(Equal(preMoveClaim1Addr.Spec.PoolRef.Name))
		Expect(movedAddr1.Spec.PoolRef.Kind).To(Equal(preMoveClaim1Addr.Spec.PoolRef.Kind))
		Expect(movedAddr1.Spec.PoolRef.APIGroup).To(Equal(preMoveClaim1Addr.Spec.PoolRef.APIGroup))
		Expect(movedAddr1.Spec.ClaimRef.Name).To(Equal(preMoveClaim1Addr.Spec.ClaimRef.Name))

		movedClaim2 := &capipamv1.IPAddressClaim{}
		Eventually(func(g Gomega) {
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "prealloc-claim-2"}, movedClaim2)).To(Succeed())
			g.Expect(movedClaim2.Status.AddressRef.Name).ToNot(BeEmpty(), "Moved claim should have address status")
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(Succeed())
		movedAddr2 := &capipamv1.IPAddress{}
		Expect(targetClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      movedClaim2.Status.AddressRef.Name,
		}, movedAddr2)).To(Succeed())
		Expect(movedAddr2.Spec.Address).To(Equal(preMoveClaim2Addr.Spec.Address), "IP address should be preserved after move")
		Expect(movedAddr2.Spec.PoolRef.Name).To(Equal(preMoveClaim2Addr.Spec.PoolRef.Name))
		Expect(movedAddr2.Spec.PoolRef.Kind).To(Equal(preMoveClaim2Addr.Spec.PoolRef.Kind))
		Expect(movedAddr2.Spec.PoolRef.APIGroup).To(Equal(preMoveClaim2Addr.Spec.PoolRef.APIGroup))
		Expect(movedAddr2.Spec.ClaimRef.Name).To(Equal(preMoveClaim2Addr.Spec.ClaimRef.Name))

		By("Verifying IPPool status is reconciled on target cluster")
		Eventually(func(g Gomega) {
			pool := &ipamv1.IPPool{}
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-prealloc-pool"}, pool)).To(Succeed())
			g.Expect(pool.Status.Allocations).To(HaveLen(2), "Moved pool should have 2 allocations")
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(Succeed())

		By("Verifying objects are removed from the source (bootstrap) cluster")
		Eventually(func() bool {
			return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-prealloc-pool"}, &ipamv1.IPPool{}))
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPPool should be removed from source after move")

		Eventually(func() bool {
			return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "prealloc-claim-1"}, &capipamv1.IPAddressClaim{}))
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPClaim should be removed from source after move")

		By("Verifying IPPool finalizer is functional on target (deletion blocked while claims exist)")
		Expect(targetClient.Delete(ctx, movedPool)).To(Succeed())
		Consistently(func() bool {
			pool := &ipamv1.IPPool{}
			err := targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-prealloc-pool"}, pool)
			if err != nil {
				return false
			}
			return pool.DeletionTimestamp != nil
		}, "5s", "1s").Should(BeTrue(), "IPPool should have DeletionTimestamp but still exist due to finalizer")

		By("Cleaning up claims on target to release pool finalizer")
		Expect(targetClient.Delete(ctx, movedClaim1)).To(Succeed())
		Expect(targetClient.Delete(ctx, movedClaim2)).To(Succeed())

		By("Verifying IPPool is eventually deleted on target after all claims removed")
		Eventually(func() bool {
			return apierrors.IsNotFound(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-prealloc-pool"}, &ipamv1.IPPool{}))
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-ippool")...).Should(BeTrue(), "IPPool should be deleted after all claims removed")

	})
})

// labelPoolForMove adds the clusterctl.cluster.x-k8s.io/move-hierarchy label to an IPPool
// so that `clusterctl move` includes it AND all objects it owns (IPAddresses)
// in the move operation. IPClaims don't have ownerReferences to IPPool, so they
// also need the move label directly.
func labelPoolForMove(ctx context.Context, cl client.Client, pool *ipamv1.IPPool) {
	Logf("Adding clusterctl move-hierarchy label to IPPool %s/%s", pool.Namespace, pool.Name)
	Eventually(func(g Gomega) {
		updatedPool := &ipamv1.IPPool{}
		g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(pool), updatedPool)).To(Succeed())
		labels := updatedPool.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["clusterctl.cluster.x-k8s.io/move-hierarchy"] = ""
		updatedPool.SetLabels(labels)
		g.Expect(cl.Update(ctx, updatedPool)).To(Succeed())
	}, "10s", "1s").Should(Succeed())
}

// labelClaimForMove adds the clusterctl.cluster.x-k8s.io/move-hierarchy label to an IPClaim.
// IPClaims reference the pool via spec.pool but don't have an ownerReference to the IPPool,
// so they must be explicitly labeled for clusterctl move to discover them.
func labelClaimForMove(ctx context.Context, cl client.Client, claim *ipamv1.IPClaim) {
	Logf("Adding clusterctl move-hierarchy label to IPClaim %s/%s", claim.Namespace, claim.Name)
	Eventually(func(g Gomega) {
		updated := &ipamv1.IPClaim{}
		g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), updated)).To(Succeed())
		labels := updated.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["clusterctl.cluster.x-k8s.io/move-hierarchy"] = ""
		updated.SetLabels(labels)
		g.Expect(cl.Update(ctx, updated)).To(Succeed())
	}, "10s", "1s").Should(Succeed())
}

// labelIPAddressClaimForMove adds the clusterctl.cluster.x-k8s.io/move-hierarchy label to a CAPI IPAddressClaim.
func labelIPAddressClaimForMove(ctx context.Context, cl client.Client, claim *capipamv1.IPAddressClaim) {
	Logf("Adding clusterctl move-hierarchy label to IPAddressClaim %s/%s", claim.Namespace, claim.Name)
	Eventually(func(g Gomega) {
		updated := &capipamv1.IPAddressClaim{}
		g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), updated)).To(Succeed())
		labels := updated.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["clusterctl.cluster.x-k8s.io/move-hierarchy"] = ""
		updated.SetLabels(labels)
		g.Expect(cl.Update(ctx, updated)).To(Succeed())
	}, "10s", "1s").Should(Succeed())
}

// prePivotMarkerAnnotation is stamped on every IPAddress before the move with the
// IPAddress's source-cluster UID. clusterctl move copies annotations, but an
// IPAddress newly allocated by the IPAM controller on the target will not have
// it, which lets the test tell "moved and reused" apart from "re-allocated",
// even if a re-allocation happened to produce the same IP and object name.
const prePivotMarkerAnnotation = "e2e.ipam.metal3.io/pre-pivot-uid"

// Number of claims of each kind allocated before the move in the main move spec.
const (
	pivotCAPIClaimCount = 7
	pivotM3ClaimCount   = 8
	pivotClaimCount     = pivotCAPIClaimCount + pivotM3ClaimCount
)

// moveTargetClusterName is the name of the target Kind cluster of the main move
// spec and of the CAPI Cluster object that represents it and owns the IPPool.
const moveTargetClusterName = "ipam-move-target"

const (
	kindIPClaim        = "IPClaim"
	kindIPAddressClaim = "IPAddressClaim"
)

// pivotAllocationRecord captures the allocation of a single claim before the move.
// In this standalone IPAM suite there are no Machines; each claim stands in for
// the consumer (e.g. a Metal3Machine) that would own it in a CAPM3 deployment.
type pivotAllocationRecord struct {
	claimKind   string
	claimName   string
	addressName string
	ip          string
	marker      string
}

// pivotBaseline is the pre-move IPAM state of a single IPPool.
type pivotBaseline struct {
	poolName         string
	records          []pivotAllocationRecord
	poolAllocations  map[string]ipamv1.IPAddressStr
	m3ClaimNames     []string
	capiClaimNames   []string
	m3AddressNames   []string
	capiAddressNames []string
}

// dumpPivotSnapshot writes every resource in the namespace as YAML to
// <artifactFolder>/<stage>/<cluster>/<namespace>/<Kind>/<name>.yaml, so the
// IPAM objects can be compared before and after the move.
func dumpPivotSnapshot(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, stage string) {
	logPath := filepath.Join(artifactFolder, stage, clusterProxy.GetName())
	By(fmt.Sprintf("Dumping %s snapshot of namespace %s from cluster %s to %s", stage, namespace, clusterProxy.GetName(), logPath))
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:               clusterProxy.GetClient(),
		KubeConfigPath:       clusterProxy.GetKubeconfigPath(),
		ClusterctlConfigPath: clusterctlConfigPath,
		Namespace:            namespace,
		LogPath:              logPath,
	})
}

// createPivotCluster creates a minimal CAPI Cluster object that represents the
// move target cluster and owns the IPPool. The infrastructureRef is required by the CAPI webhook; it points at a
// DockerCluster that is intentionally never created, so no infrastructure is
// provisioned for this Cluster.
func createPivotCluster(ctx context.Context, cl client.Client, namespace, name string) *clusterv1.Cluster {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: "infrastructure.cluster.x-k8s.io",
				Kind:     "DockerCluster",
				Name:     name + "-infra",
			},
		},
	}
	Expect(cl.Create(ctx, cluster)).To(Succeed())
	Expect(cl.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
	Logf("Created Cluster %s/%s (UID %s)", cluster.Namespace, cluster.Name, cluster.UID)
	return cluster
}

// markClusterReadyForMove sets the status fields clusterctl move checks before it
// starts (infrastructure provisioned, control plane initialized). Both are
// write-once from the CAPI core controller's point of view: it never resets
// status.initialization.infrastructureProvisioned and stops recomputing
// ControlPlaneInitialized once it is True. We wait for the controller to have
// computed the condition at least once so our value is not overwritten by its
// first reconcile.
func markClusterReadyForMove(ctx context.Context, cl client.Client, cluster *clusterv1.Cluster) {
	Eventually(func(g Gomega) {
		c := &clusterv1.Cluster{}
		g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(cluster), c)).To(Succeed())
		g.Expect(meta.FindStatusCondition(c.Status.Conditions, clusterv1.ClusterControlPlaneInitializedCondition)).ToNot(BeNil(),
			"waiting for the CAPI controller to reconcile Cluster %s", c.Name)
	}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

	Eventually(func(g Gomega) {
		c := &clusterv1.Cluster{}
		g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(cluster), c)).To(Succeed())
		infraProvisioned := c.Status.Initialization.InfrastructureProvisioned != nil && *c.Status.Initialization.InfrastructureProvisioned
		if !infraProvisioned || !meta.IsStatusConditionTrue(c.Status.Conditions, clusterv1.ClusterControlPlaneInitializedCondition) {
			patchBase := client.MergeFrom(c.DeepCopy())
			provisioned := true
			c.Status.Initialization.InfrastructureProvisioned = &provisioned
			meta.SetStatusCondition(&c.Status.Conditions, metav1.Condition{
				Type:   clusterv1.ClusterControlPlaneInitializedCondition,
				Status: metav1.ConditionTrue,
				Reason: clusterv1.ClusterControlPlaneInitializedReason,
			})
			g.Expect(cl.Status().Patch(ctx, c, patchBase)).To(Succeed())
		}
		fresh := &clusterv1.Cluster{}
		g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(cluster), fresh)).To(Succeed())
		g.Expect(fresh.Status.Initialization.InfrastructureProvisioned).To(HaveValue(BeTrue()))
		g.Expect(meta.IsStatusConditionTrue(fresh.Status.Conditions, clusterv1.ClusterControlPlaneInitializedCondition)).To(BeTrue())
	}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
}

// waitForCAPIClaimAllocated waits until the CAPI IPAddressClaim references an IPAddress.
func waitForCAPIClaimAllocated(ctx context.Context, cl client.Client, claim *capipamv1.IPAddressClaim) {
	Eventually(func(g Gomega) {
		retrieved := &capipamv1.IPAddressClaim{}
		g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), retrieved)).To(Succeed())
		g.Expect(retrieved.Status.AddressRef.Name).ToNot(BeEmpty(), "IPAddressClaim %s should be allocated", claim.Name)
	}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
}

// stampPrePivotMarker annotates obj with its own source-cluster UID.
func stampPrePivotMarker(ctx context.Context, cl client.Client, obj client.Object) string {
	marker := string(obj.GetUID())
	Expect(marker).ToNot(BeEmpty())
	Eventually(func(g Gomega) {
		fresh, ok := obj.DeepCopyObject().(client.Object)
		g.Expect(ok).To(BeTrue())
		g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(obj), fresh)).To(Succeed())
		base, ok := fresh.DeepCopyObject().(client.Object)
		g.Expect(ok).To(BeTrue())
		patchBase := client.MergeFrom(base)
		annotations := fresh.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[prePivotMarkerAnnotation] = marker
		fresh.SetAnnotations(annotations)
		g.Expect(cl.Patch(ctx, fresh, patchBase)).To(Succeed())
	}, "10s", "1s").Should(Succeed())
	return marker
}

// recordPivotBaseline records every claim of the pool, the IPAddress it references,
// the allocated IP and the pool's allocation map. It also stamps each IPAddress with
// prePivotMarkerAnnotation and asserts the pre-move state is self-consistent, so
// that post-move mismatches can only be caused by the move.
func recordPivotBaseline(ctx context.Context, cl client.Client, namespace, poolName string) pivotBaseline {
	baseline := pivotBaseline{poolName: poolName}

	pool := &ipamv1.IPPool{}
	Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: poolName}, pool)).To(Succeed())
	baseline.poolAllocations = make(map[string]ipamv1.IPAddressStr, len(pool.Status.Allocations))
	for claimName, ip := range pool.Status.Allocations {
		baseline.poolAllocations[claimName] = ip
	}

	m3Claims := &ipamv1.IPClaimList{}
	Expect(cl.List(ctx, m3Claims, client.InNamespace(namespace))).To(Succeed())
	for i := range m3Claims.Items {
		claim := &m3Claims.Items[i]
		if claim.Spec.Pool.Name != poolName {
			continue
		}
		Expect(claim.Status.Address).ToNot(BeNil(), "pre-move: IPClaim %s has no address", claim.Name)
		addr := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: claim.Status.Address.Name}, addr)).To(Succeed(),
			"pre-move: IPAddress %s referenced by IPClaim %s not found", claim.Status.Address.Name, claim.Name)
		Expect(addr.Spec.Claim.Name).To(Equal(claim.Name), "pre-move: IPAddress %s does not point back to IPClaim %s", addr.Name, claim.Name)
		Expect(addr.Spec.Pool.Name).To(Equal(poolName))
		baseline.m3ClaimNames = append(baseline.m3ClaimNames, claim.Name)
		baseline.records = append(baseline.records, pivotAllocationRecord{
			claimKind:   kindIPClaim,
			claimName:   claim.Name,
			addressName: addr.Name,
			ip:          string(addr.Spec.Address),
			marker:      stampPrePivotMarker(ctx, cl, addr),
		})
	}

	capiClaims := &capipamv1.IPAddressClaimList{}
	Expect(cl.List(ctx, capiClaims, client.InNamespace(namespace))).To(Succeed())
	for i := range capiClaims.Items {
		claim := &capiClaims.Items[i]
		if claim.Spec.PoolRef.Name != poolName {
			continue
		}
		Expect(claim.Status.AddressRef.Name).ToNot(BeEmpty(), "pre-move: IPAddressClaim %s has no address", claim.Name)
		addr := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: claim.Status.AddressRef.Name}, addr)).To(Succeed(),
			"pre-move: IPAddress %s referenced by IPAddressClaim %s not found", claim.Status.AddressRef.Name, claim.Name)
		Expect(addr.Spec.ClaimRef.Name).To(Equal(claim.Name), "pre-move: IPAddress %s does not point back to IPAddressClaim %s", addr.Name, claim.Name)
		Expect(addr.Spec.PoolRef.Name).To(Equal(poolName))
		baseline.capiClaimNames = append(baseline.capiClaimNames, claim.Name)
		baseline.records = append(baseline.records, pivotAllocationRecord{
			claimKind:   kindIPAddressClaim,
			claimName:   claim.Name,
			addressName: addr.Name,
			ip:          addr.Spec.Address,
			marker:      stampPrePivotMarker(ctx, cl, addr),
		})
	}

	baseline.m3AddressNames, baseline.capiAddressNames = listPoolAddressNames(ctx, Default, cl, namespace, poolName)

	Expect(baseline.poolAllocations).To(HaveLen(len(baseline.records)),
		"pre-move: IPPool allocations %v do not match the %d allocated claims", baseline.poolAllocations, len(baseline.records))
	Expect(baseline.m3AddressNames).To(HaveLen(len(baseline.m3ClaimNames)), "pre-move: unexpected number of Metal3 IPAddresses")
	Expect(baseline.capiAddressNames).To(HaveLen(len(baseline.capiClaimNames)), "pre-move: unexpected number of CAPI IPAddresses")
	expectUniquePoolIPs(ctx, Default, cl, namespace, poolName, "pre-move")
	for _, r := range baseline.records {
		Expect(baseline.poolAllocations).To(HaveKeyWithValue(r.claimName, ipamv1.IPAddressStr(r.ip)),
			"pre-move: IPPool allocation for %s %s does not match its IPAddress", r.claimKind, r.claimName)
		Logf("Pre-move baseline: %s %s -> IPAddress %s (%s), marker %s", r.claimKind, r.claimName, r.addressName, r.ip, r.marker)
	}
	return baseline
}

// listPoolAddressNames returns the sorted names of the Metal3 and CAPI IPAddresses
// in the namespace that belong to the given pool.
func listPoolAddressNames(ctx context.Context, g Gomega, cl client.Client, namespace, poolName string) ([]string, []string) {
	m3Addrs := &ipamv1.IPAddressList{}
	g.Expect(cl.List(ctx, m3Addrs, client.InNamespace(namespace))).To(Succeed())
	m3Names := []string{}
	for _, a := range m3Addrs.Items {
		if a.Spec.Pool.Name == poolName {
			m3Names = append(m3Names, a.Name)
		}
	}
	capiAddrs := &capipamv1.IPAddressList{}
	g.Expect(cl.List(ctx, capiAddrs, client.InNamespace(namespace))).To(Succeed())
	capiNames := []string{}
	for _, a := range capiAddrs.Items {
		if a.Spec.PoolRef.Name == poolName {
			capiNames = append(capiNames, a.Name)
		}
	}
	sort.Strings(m3Names)
	sort.Strings(capiNames)
	return m3Names, capiNames
}

// expectUniquePoolIPs asserts that no IP of the pool is held by more than one
// IPAddress, across Metal3 and CAPI IPAddresses together. A Metal3 and a CAPI
// IPAddress can have the same name and IP because their kinds are different, so
// the per-kind and per-claim checks do not catch a duplicate across kinds.
func expectUniquePoolIPs(ctx context.Context, g Gomega, cl client.Client, namespace, poolName, stage string) {
	holders := map[string][]string{}
	m3Addrs := &ipamv1.IPAddressList{}
	g.Expect(cl.List(ctx, m3Addrs, client.InNamespace(namespace))).To(Succeed())
	for _, a := range m3Addrs.Items {
		if a.Spec.Pool.Name == poolName {
			ip := string(a.Spec.Address)
			holders[ip] = append(holders[ip], kindIPClaim+"/"+a.Spec.Claim.Name)
		}
	}
	capiAddrs := &capipamv1.IPAddressList{}
	g.Expect(cl.List(ctx, capiAddrs, client.InNamespace(namespace))).To(Succeed())
	for _, a := range capiAddrs.Items {
		if a.Spec.PoolRef.Name == poolName {
			holders[a.Spec.Address] = append(holders[a.Spec.Address], kindIPAddressClaim+"/"+a.Spec.ClaimRef.Name)
		}
	}
	duplicates := map[string][]string{}
	for ip, claims := range holders {
		if len(claims) > 1 {
			duplicates[ip] = claims
		}
	}
	g.Expect(duplicates).To(BeEmpty(), "%s: IPs of pool %s are allocated more than once (IP -> claims): %v", stage, poolName, duplicates)
}

// listPoolClaimNames returns the names of the Metal3 IPClaims and CAPI
// IPAddressClaims in the namespace that reference the given pool.
func listPoolClaimNames(ctx context.Context, g Gomega, cl client.Client, namespace, poolName string) ([]string, []string) {
	m3Claims := &ipamv1.IPClaimList{}
	g.Expect(cl.List(ctx, m3Claims, client.InNamespace(namespace))).To(Succeed())
	m3Names := []string{}
	for _, c := range m3Claims.Items {
		if c.Spec.Pool.Name == poolName {
			m3Names = append(m3Names, c.Name)
		}
	}
	capiClaims := &capipamv1.IPAddressClaimList{}
	g.Expect(cl.List(ctx, capiClaims, client.InNamespace(namespace))).To(Succeed())
	capiNames := []string{}
	for _, c := range capiClaims.Items {
		if c.Spec.PoolRef.Name == poolName {
			capiNames = append(capiNames, c.Name)
		}
	}
	return m3Names, capiNames
}

// expectPivotBaselinePreserved asserts that the IPAM state on the target cluster
// matches the pre-move baseline: same claims, same IPAddress objects (by name and
// pre-pivot marker), same IPs, intact claim <-> address relationships, and no
// additional allocations.
func expectPivotBaselinePreserved(ctx context.Context, g Gomega, cl client.Client, namespace string, baseline pivotBaseline) {
	m3ClaimNames, capiClaimNames := listPoolClaimNames(ctx, g, cl, namespace, baseline.poolName)
	g.Expect(m3ClaimNames).To(ConsistOf(baseline.m3ClaimNames),
		"Metal3 IPClaims on target do not match the pre-move set: missing or unexpected IPClaims after move")
	g.Expect(capiClaimNames).To(ConsistOf(baseline.capiClaimNames),
		"CAPI IPAddressClaims on target do not match the pre-move set: missing or unexpected IPAddressClaims after move")

	m3AddrNames, capiAddrNames := listPoolAddressNames(ctx, g, cl, namespace, baseline.poolName)
	g.Expect(m3AddrNames).To(ConsistOf(baseline.m3AddressNames),
		"Metal3 IPAddresses on target do not match the pre-move set: an IPAddress is missing or a new one was allocated")
	g.Expect(capiAddrNames).To(ConsistOf(baseline.capiAddressNames),
		"CAPI IPAddresses on target do not match the pre-move set: an IPAddress is missing or a new one was allocated")

	pool := &ipamv1.IPPool{}
	g.Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: baseline.poolName}, pool)).To(Succeed())
	g.Expect(pool.Status.Allocations).To(Equal(baseline.poolAllocations),
		"IPPool %s allocations changed across the move (incorrect IPPool reconciliation or re-allocation)", pool.Name)
	expectUniquePoolIPs(ctx, g, cl, namespace, baseline.poolName, "post-move")

	for _, r := range baseline.records {
		var (
			referencedAddr string
			addrObj        client.Object
			addrIP         string
			addrClaimName  string
			addrPoolName   string
			claimUID       types.UID
		)
		switch r.claimKind {
		case kindIPClaim:
			claim := &ipamv1.IPClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: r.claimName}, claim)).To(Succeed(),
				"IPClaim %s was not moved to the target cluster", r.claimName)
			g.Expect(claim.Status.Address).ToNot(BeNil(), "moved IPClaim %s has no address reference yet", r.claimName)
			referencedAddr = claim.Status.Address.Name
			claimUID = claim.UID
			addr := &ipamv1.IPAddress{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: r.addressName}, addr)).To(Succeed(),
				"IPAddress %s (of IPClaim %s) was not moved to the target cluster", r.addressName, r.claimName)
			addrObj, addrIP, addrClaimName, addrPoolName = addr, string(addr.Spec.Address), addr.Spec.Claim.Name, addr.Spec.Pool.Name
		case kindIPAddressClaim:
			claim := &capipamv1.IPAddressClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: r.claimName}, claim)).To(Succeed(),
				"IPAddressClaim %s was not moved to the target cluster", r.claimName)
			referencedAddr = claim.Status.AddressRef.Name
			claimUID = claim.UID
			addr := &capipamv1.IPAddress{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: r.addressName}, addr)).To(Succeed(),
				"IPAddress %s (of IPAddressClaim %s) was not moved to the target cluster", r.addressName, r.claimName)
			addrObj, addrIP, addrClaimName, addrPoolName = addr, addr.Spec.Address, addr.Spec.ClaimRef.Name, addr.Spec.PoolRef.Name
			g.Expect(addr.Spec.PoolRef.Kind).To(Equal("IPPool"),
				"IPAddress %s poolRef.kind changed across the move", r.addressName)
			g.Expect(addr.Spec.PoolRef.APIGroup).To(Equal(ipamv1.GroupVersion.Group),
				"IPAddress %s poolRef.apiGroup changed across the move", r.addressName)
		}

		g.Expect(referencedAddr).To(Equal(r.addressName),
			"%s %s references IPAddress %q after move, expected pre-move IPAddress %q: claim -> address relationship broken",
			r.claimKind, r.claimName, referencedAddr, r.addressName)
		g.Expect(addrObj.GetAnnotations()).To(HaveKeyWithValue(prePivotMarkerAnnotation, r.marker),
			"IPAddress %s is missing the pre-move marker: it was re-created on the target instead of being moved and reused", r.addressName)
		g.Expect(addrIP).To(Equal(r.ip),
			"IP of %s %s changed across the move: was %s, now %s", r.claimKind, r.claimName, r.ip, addrIP)
		g.Expect(addrClaimName).To(Equal(r.claimName),
			"IPAddress %s points at claim %q after move, expected %q", r.addressName, addrClaimName, r.claimName)
		g.Expect(addrPoolName).To(Equal(baseline.poolName),
			"IPAddress %s points at pool %q after move, expected %q", r.addressName, addrPoolName, baseline.poolName)
		g.Expect(addrObj.GetOwnerReferences()).To(ContainElement(SatisfyAll(
			HaveField("Kind", Equal(r.claimKind)),
			HaveField("Name", Equal(r.claimName)),
			HaveField("UID", Equal(claimUID)),
		)), "IPAddress %s is not owned by the moved %s %s (UID %s)", r.addressName, r.claimKind, r.claimName, claimUID)
		g.Expect(addrObj.GetOwnerReferences()).To(ContainElement(SatisfyAll(
			HaveField("Kind", Equal("IPPool")),
			HaveField("Name", Equal(baseline.poolName)),
			HaveField("UID", Equal(pool.UID)),
		)), "IPAddress %s is not owned by the moved IPPool %s (UID %s)", r.addressName, baseline.poolName, pool.UID)
	}
}
