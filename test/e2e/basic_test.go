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
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ctx = context.TODO()

var _ = Describe("Metal3 IPAM basic functionality", Label("ipam", "basic"), func() {
	var namespace string

	BeforeEach(func() {
		namespace = testNamespace()
		validateGlobals()
		cl := bootstrapClusterProxy.GetClient()
		// Create namespace for the test if it doesn't exist
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
		// Delete the namespace to clean up all test resources
		cleanupNamespace(ctx, bootstrapClusterProxy.GetClient(), namespace)
	})

	It("Should allocate an IPAddress via Metal3 IPClaim", func() {
		cl := bootstrapClusterProxy.GetClient()

		By("Creating an IPPool")
		ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
			Name:       "test-ippool-m3claim",
			Namespace:  namespace,
			Start:      "192.168.1.10",
			End:        "192.168.1.100",
			Subnet:     "192.168.1.0/24",
			Prefix:     24,
			Gateway:    "192.168.1.1",
			DNSServers: []string{"8.8.8.8", "8.8.4.4"},
			NamePrefix: "test-ip",
		})

		By("Verifying that the IPPool is created successfully")
		Eventually(func() error {
			return cl.Get(ctx, client.ObjectKeyFromObject(ipPool), &ipamv1.IPPool{})
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Creating a Metal3 IPClaim referencing the pool")
		claimName := "test-ipclaim"
		ipClaim := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, claimName, namespace)

		By("Waiting for the IPClaim to get an IPAddress allocated")
		Eventually(func(g Gomega) {
			retrieved := &ipamv1.IPClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.Address).ToNot(BeNil(), "IPClaim should have an address allocated")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying the Metal3 IPAddress object")
		updatedClaim := &ipamv1.IPClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), updatedClaim)).To(Succeed())

		ipAddress := &ipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: updatedClaim.Status.Address.Namespace,
			Name:      updatedClaim.Status.Address.Name,
		}, ipAddress)).To(Succeed())

		Expect(string(ipAddress.Spec.Address)).ToNot(BeEmpty())
		ip := net.ParseIP(string(ipAddress.Spec.Address))
		Expect(ip).ToNot(BeNil(), "Allocated address should be a valid IP")
		Expect(ipAddress.Spec.Prefix).To(Equal(24))
		Expect(ipAddress.Spec.Pool.Name).To(Equal(ipPool.Name))

		By("Cleaning up IPClaim and IPPool")
		Expect(cl.Delete(ctx, ipClaim)).To(Succeed())
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), &ipamv1.IPClaim{})
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPClaim should be deleted")

		By("Verifying the IPAddress is deleted after IPClaim removal")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKey{
				Namespace: ipAddress.Namespace,
				Name:      ipAddress.Name,
			}, &ipamv1.IPAddress{})
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPAddress should be cleaned up after IPClaim deletion")

		Expect(cl.Delete(ctx, ipPool)).To(Succeed())
	})

	It("Should allocate an IPAddress via CAPI IPAddressClaim", func() {
		cl := bootstrapClusterProxy.GetClient()

		By("Creating an IPPool")
		ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
			Name:       "test-ippool-capi",
			Namespace:  namespace,
			Start:      "192.168.1.10",
			End:        "192.168.1.100",
			Subnet:     "192.168.1.0/24",
			Prefix:     24,
			Gateway:    "192.168.1.1",
			DNSServers: []string{"8.8.8.8", "8.8.4.4"},
			NamePrefix: "test-ip",
		})

		By("Verifying that the IPPool is created successfully")
		Eventually(func() error {
			return cl.Get(ctx, client.ObjectKeyFromObject(ipPool), &ipamv1.IPPool{})
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Creating a CAPI IPAddressClaim referencing the Metal3 IPPool")
		claimName := "test-capi-ipclaim"
		ipAddressClaim := createCAPIIPAddressClaim(ctx, bootstrapClusterProxy, ipPool.Name, claimName, namespace)

		By("Waiting for the CAPI IPAddressClaim to get an address")
		Eventually(func(g Gomega) {
			retrieved := &capipamv1.IPAddressClaim{}
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipAddressClaim), retrieved)).To(Succeed())
			g.Expect(retrieved.Status.AddressRef.Name).ToNot(BeEmpty(), "IPAddressClaim should have addressRef set")
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

		By("Verifying the CAPI IPAddress object")
		updatedClaim := &capipamv1.IPAddressClaim{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipAddressClaim), updatedClaim)).To(Succeed())

		capiIPAddress := &capipamv1.IPAddress{}
		Expect(cl.Get(ctx, client.ObjectKey{
			Namespace: ipAddressClaim.Namespace,
			Name:      updatedClaim.Status.AddressRef.Name,
		}, capiIPAddress)).To(Succeed())

		Expect(capiIPAddress.Spec.Address).ToNot(BeEmpty())
		ip := net.ParseIP(capiIPAddress.Spec.Address)
		Expect(ip).ToNot(BeNil(), "Allocated address should be a valid IP")
		Expect(capiIPAddress.Spec.PoolRef.Name).To(Equal(ipPool.Name))
		Expect(capiIPAddress.Spec.PoolRef.Kind).To(Equal("IPPool"))
		Expect(capiIPAddress.Spec.PoolRef.APIGroup).To(Equal("ipam.metal3.io"))
		Expect(capiIPAddress.Spec.ClaimRef.Name).To(Equal(ipAddressClaim.Name))

		By("Cleaning up CAPI IPAddressClaim and IPPool")
		Expect(cl.Delete(ctx, ipAddressClaim)).To(Succeed())
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKeyFromObject(ipAddressClaim), &capipamv1.IPAddressClaim{})
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPAddressClaim should be deleted")

		By("Verifying the CAPI IPAddress is deleted after IPAddressClaim removal")
		Eventually(func() bool {
			err := cl.Get(ctx, client.ObjectKey{
				Namespace: capiIPAddress.Namespace,
				Name:      capiIPAddress.Name,
			}, &capipamv1.IPAddress{})
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "CAPI IPAddress should be cleaned up after IPAddressClaim deletion")

		Expect(cl.Delete(ctx, ipPool)).To(Succeed())
	})
})
