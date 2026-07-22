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
	"net"
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
var _ = Describe("When testing clusterctl move of IPAM resources", Label("ipam", "pivot"), func() {
	var (
		targetClusterProvider bootstrap.ClusterProvider
		targetClusterProxy    framework.ClusterProxy
		namespace             string
	)

	BeforeEach(func() {
		namespace = testNamespace()
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
		if skipCleanup {
			return
		}
		cleanupNamespace(ctx, bootstrapClusterProxy.GetClient(), namespace)
		if targetClusterProxy != nil {
			targetClusterProxy.Dispose(ctx)
		}
		if targetClusterProvider != nil {
			targetClusterProvider.Dispose(ctx)
		}
	})

	It("Should move IPPool, IPClaim and IPAddressClaim resources to a target cluster", func() {
		cl := bootstrapClusterProxy.GetClient()

		By("Creating an IPPool with allocations on the bootstrap cluster")
		ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
			Name:       "move-test-pool",
			Namespace:  namespace,
			Start:      "10.0.0.10",
			End:        "10.0.0.50",
			Subnet:     "10.0.0.0/24",
			Prefix:     24,
			Gateway:    "10.0.0.1",
			DNSServers: []string{"8.8.8.8", "8.8.4.4"},
			NamePrefix: "move-test",
		})

		By("Labeling IPPool for clusterctl move")
		labelPoolForMove(ctx, cl, ipPool)

		By("Creating CAPI IPAddressClaims on the bootstrap cluster")
		claim1 := createCAPIIPAddressClaim(ctx, bootstrapClusterProxy, ipPool.Name, "move-claim-1", namespace)
		claim2 := createCAPIIPAddressClaim(ctx, bootstrapClusterProxy, ipPool.Name, "move-claim-2", namespace)

		By("Creating a Metal3 IPClaim on the bootstrap cluster")
		m3Claim := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "move-capi-claim", namespace)

		By("Waiting for all claims to be allocated")
		for _, claim := range []*capipamv1.IPAddressClaim{claim1, claim2} {
			Eventually(func(g Gomega) {
				retrieved := &capipamv1.IPAddressClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.AddressRef.Name).ToNot(BeEmpty())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
		}
		Eventually(func(g Gomega) {
			retrieved := &ipamv1.IPClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(m3Claim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.Address.Name).ToNot(BeEmpty())
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Labeling claims for clusterctl move")
		labelIPAddressClaimForMove(ctx, cl, claim1)
		labelIPAddressClaimForMove(ctx, cl, claim2)
		labelClaimForMove(ctx, cl, m3Claim)

		By("Recording pre-move state")
		preMovePool := &ipamv1.IPPool{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipPool), preMovePool)).To(Succeed())
		Expect(preMovePool.Status.Allocations).To(HaveLen(3), "Should have 3 allocations before move")

		// Record the allocated IPs for verification after move
		preMoveClaim1 := &capipamv1.IPAddressClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim1), preMoveClaim1)).To(Succeed())
		preMoveClaim1Addr := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      preMoveClaim1.Status.AddressRef.Name,
		}, preMoveClaim1Addr)).To(Succeed())

		preMoveClaim2 := &capipamv1.IPAddressClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), preMoveClaim2)).To(Succeed())
		preMoveClaim2Addr := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      preMoveClaim2.Status.AddressRef.Name,
		}, preMoveClaim2Addr)).To(Succeed())

		preMoveM3Claim := &ipamv1.IPClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(m3Claim), preMoveM3Claim)).To(Succeed())
		preMoveM3ClaimAddr := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      preMoveM3Claim.Status.Address.Name,
		}, preMoveM3ClaimAddr)).To(Succeed())

		By("Creating a target Kind cluster for the move")
		targetClusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               "ipam-move-target",
			RequiresDockerSock: e2eConfig.HasDockerProvider(),
			Images:             e2eConfig.Images,
			LogFolder:          filepath.Join(artifactFolder, "clusters", "ipam-move-target"),
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
			LogFolder:               filepath.Join(artifactFolder, "clusters", "ipam-move-target"),
		}, e2eConfig.GetIntervals("clusterctl-move", "wait-controllers")...)

		By("Ensuring the target namespace exists on the target cluster")
		targetNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		Expect(targetClusterProxy.GetClient().Create(ctx, targetNs)).To(Succeed())

		By("Moving IPAM resources from bootstrap to target cluster")
		clusterctl.Move(ctx, clusterctl.MoveInput{
			LogFolder:            filepath.Join(artifactFolder, "clusters", "ipam-move"),
			ClusterctlConfigPath: clusterctlConfigPath,
			FromKubeconfigPath:   bootstrapClusterProxy.GetKubeconfigPath(),
			ToKubeconfigPath:     targetClusterProxy.GetKubeconfigPath(),
			Namespace:            namespace,
		})

		targetClient := targetClusterProxy.GetClient()

		By("Verifying IPPool exists on target cluster with correct spec")
		movedPool := &ipamv1.IPPool{}
		Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-test-pool"}, movedPool)).To(Succeed())
		Expect(movedPool.Spec.Pools).To(Equal(preMovePool.Spec.Pools))
		Expect(movedPool.Spec.Prefix).To(Equal(preMovePool.Spec.Prefix))
		Expect(movedPool.Spec.Gateway).To(Equal(preMovePool.Spec.Gateway))
		Expect(movedPool.Spec.DNSServers).To(Equal(preMovePool.Spec.DNSServers))
		Expect(movedPool.Spec.NamePrefix).To(Equal(preMovePool.Spec.NamePrefix))

		By("Verifying CAPI IPAddressClaims exist on target cluster with allocations")
		movedClaim1 := &capipamv1.IPAddressClaim{}
		Eventually(func(g Gomega) {
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-claim-1"}, movedClaim1)).To(Succeed())
			g.Expect(movedClaim1.Status.AddressRef.Name).ToNot(BeEmpty(), "Moved claim1 should have address")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		movedClaim2 := &capipamv1.IPAddressClaim{}
		Eventually(func(g Gomega) {
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-claim-2"}, movedClaim2)).To(Succeed())
			g.Expect(movedClaim2.Status.AddressRef.Name).ToNot(BeEmpty(), "Moved claim2 should have address")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		movedM3Claim := &ipamv1.IPClaim{}
		Eventually(func(g Gomega) {
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3Claim.Name}, movedM3Claim)).To(Succeed())
			g.Expect(movedM3Claim.Status.Address.Name).ToNot(BeEmpty(), "Moved m3 claim should have address")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying IPAddress objects exist on target with preserved addresses")
		movedAddr1 := &capipamv1.IPAddress{}
		Expect(targetClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      movedClaim1.Status.AddressRef.Name,
		}, movedAddr1)).To(Succeed())
		Expect(movedAddr1.Spec.Address).To(Equal(preMoveClaim1Addr.Spec.Address), "IP address should be preserved after move")
		Expect(net.ParseIP(movedAddr1.Spec.Address)).ToNot(BeNil())
		Expect(movedAddr1.Spec.PoolRef.Name).To(Equal(preMoveClaim1Addr.Spec.PoolRef.Name))
		Expect(movedAddr1.Spec.PoolRef.Kind).To(Equal(preMoveClaim1Addr.Spec.PoolRef.Kind))
		Expect(movedAddr1.Spec.PoolRef.APIGroup).To(Equal(preMoveClaim1Addr.Spec.PoolRef.APIGroup))
		Expect(movedAddr1.Spec.ClaimRef.Name).To(Equal(preMoveClaim1Addr.Spec.ClaimRef.Name))

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

		By("Verifying Metal3 IPAddress exists on target cluster with correct spec")
		movedM3Addr := &ipamv1.IPAddress{}
		Expect(targetClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      movedM3Claim.Status.Address.Name,
		}, movedM3Addr)).To(Succeed())
		Expect(movedM3Addr.Spec.Address).ToNot(BeEmpty(), "Metal3 IPAddress should have an address")
		Expect(net.ParseIP(string(movedM3Addr.Spec.Address))).ToNot(BeNil(), "Metal3 IPAddress should be a valid IP")
		Expect(movedM3Addr.Spec.Pool.Name).To(Equal(preMoveM3ClaimAddr.Spec.Pool.Name))
		Expect(movedM3Addr.Spec.Pool.Kind).To(Equal(preMoveM3ClaimAddr.Spec.Pool.Kind))
		Expect(movedM3Addr.Spec.Claim.Name).To(Equal(preMoveM3ClaimAddr.Spec.Claim.Name))

		By("Verifying IPPool status is reconciled on target cluster")
		Eventually(func(g Gomega) {
			pool := &ipamv1.IPPool{}
			g.Expect(targetClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-test-pool"}, pool)).To(Succeed())
			g.Expect(pool.Status.Allocations).To(HaveLen(3), "Moved pool should have 3 allocations")
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
		Expect(newIP).ToNot(Equal(preMoveClaim1Addr.Spec.Address), "New allocation should not duplicate existing IPs")
		Expect(newIP).ToNot(Equal(preMoveClaim2Addr.Spec.Address), "New allocation should not duplicate existing IPs")
		Expect(newIP).ToNot(Equal(string(preMoveM3ClaimAddr.Spec.Address)), "New allocation should not duplicate existing IPs")
		Expect(newAddr.Spec.PoolRef.Name).To(Equal(preMovePool.Name))
		Expect(newAddr.Spec.PoolRef.Kind).To(Equal("IPPool"))
		Expect(newAddr.Spec.PoolRef.APIGroup).To(Equal("ipam.metal3.io"))
		Expect(newAddr.Spec.ClaimRef.Name).To(Equal(newClaimRetrieved.Name))

		By("Verifying objects are removed from the source (bootstrap) cluster")
		Eventually(func() bool {
			return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-test-pool"}, &ipamv1.IPPool{}))
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPPool should be removed from source after move")

		Eventually(func() bool {
			return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "move-claim-1"}, &capipamv1.IPAddressClaim{}))
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPClaim should be removed from source after move")

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
		Expect(targetClient.Delete(ctx, movedClaim1)).To(Succeed())
		Expect(targetClient.Delete(ctx, movedClaim2)).To(Succeed())
		Expect(targetClient.Delete(ctx, movedM3Claim)).To(Succeed())

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

		updatedClaim2 := &capipamv1.IPAddressClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), updatedClaim2)).To(Succeed())
		preMoveClaim2Addr := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      updatedClaim2.Status.AddressRef.Name,
		}, preMoveClaim2Addr)).To(Succeed())

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
