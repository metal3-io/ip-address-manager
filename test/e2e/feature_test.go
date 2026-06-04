package e2e

import (
	"net"
	"strings"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Metal3 IPAM advanced features", Label("ipam", "features"), func() {
	var namespace string

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
		cleanupNamespace(ctx, bootstrapClusterProxy.GetClient(), namespace)
	})

	Context("Preallocation", func() {
		It("Should allocate preallocated IPs to multiple matching claims", func() {
			cl := bootstrapClusterProxy.GetClient()

			claimName1 := "test-prealloc-multi-1"
			claimName2 := "test-prealloc-multi-2"
			prealloc1 := ipamv1.IPAddressStr("192.168.1.50")
			prealloc2 := ipamv1.IPAddressStr("192.168.1.60")

			By("Creating an IPPool with two preallocations")
			ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
				Name:       "test-ippool-prealloc-multi",
				Namespace:  namespace,
				Start:      "192.168.1.10",
				End:        "192.168.1.100",
				Subnet:     "192.168.1.0/24",
				Prefix:     24,
				Gateway:    "192.168.1.1",
				NamePrefix: "test-prealloc",
				PreAllocations: map[string]ipamv1.IPAddressStr{
					claimName1: prealloc1,
					claimName2: prealloc2,
				},
			})

			By("Creating both IPClaims")
			claim1 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, claimName1, namespace)
			claim2 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, claimName2, namespace)

			By("Verifying claim1 gets prealloc1")
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim1), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			updatedClaim1 := &ipamv1.IPClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim1), updatedClaim1)).To(Succeed())
			addr1 := &ipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: updatedClaim1.Status.Address.Namespace,
				Name:      updatedClaim1.Status.Address.Name,
			}, addr1)).To(Succeed())
			Expect(addr1.Spec.Address).To(Equal(prealloc1))

			By("Verifying claim2 gets prealloc2")
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			updatedClaim2 := &ipamv1.IPClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), updatedClaim2)).To(Succeed())
			addr2 := &ipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: updatedClaim2.Status.Address.Namespace,
				Name:      updatedClaim2.Status.Address.Name,
			}, addr2)).To(Succeed())
			Expect(addr2.Spec.Address).To(Equal(prealloc2))
		})
	})

	Context("Annotation-based IP request", func() {
		It("Should allocate the specific IP requested via Metal3 IPClaim annotation", func() {
			cl := bootstrapClusterProxy.GetClient()

			By("Creating an IPPool")
			ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
				Name:       "test-ippool-annot-m3",
				Namespace:  namespace,
				Start:      "192.168.1.10",
				End:        "192.168.1.100",
				Subnet:     "192.168.1.0/24",
				Prefix:     24,
				Gateway:    "192.168.1.1",
				DNSServers: []string{"8.8.8.8", "8.8.4.4"},
				NamePrefix: "test-ip",
			})

			By("Creating an IPClaim with ipAddress annotation")
			requestedIP := "192.168.1.77"
			claimName := "test-annotation-claim"
			ipClaim := &ipamv1.IPClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      claimName,
					Namespace: namespace,
					Annotations: map[string]string{
						"ipAddress": requestedIP,
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

			By("Waiting for the IPClaim to get an address")
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			By("Verifying the allocated IP matches the annotation")
			updatedClaim := &ipamv1.IPClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipClaim), updatedClaim)).To(Succeed())

			ipAddress := &ipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: updatedClaim.Status.Address.Namespace,
				Name:      updatedClaim.Status.Address.Name,
			}, ipAddress)).To(Succeed())
			Expect(string(ipAddress.Spec.Address)).To(Equal(requestedIP))
		})

		It("Should allocate the specific IP requested via CAPI IPAddressClaim annotation", func() {
			cl := bootstrapClusterProxy.GetClient()

			By("Creating an IPPool")
			ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
				Name:       "test-ippool-annot-capi",
				Namespace:  namespace,
				Start:      "192.168.1.10",
				End:        "192.168.1.100",
				Subnet:     "192.168.1.0/24",
				Prefix:     24,
				Gateway:    "192.168.1.1",
				DNSServers: []string{"8.8.8.8", "8.8.4.4"},
				NamePrefix: "test-ip",
			})

			By("Creating a CAPI IPAddressClaim with ipAddress annotation")
			requestedIP := "192.168.1.88"
			claimName := "test-capi-annotation-claim"
			claim := &capipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      claimName,
					Namespace: namespace,
					Annotations: map[string]string{
						"ipAddress": requestedIP,
					},
				},
				Spec: capipamv1.IPAddressClaimSpec{
					PoolRef: capipamv1.IPPoolReference{
						Name:     ipPool.Name,
						Kind:     "IPPool",
						APIGroup: "ipam.metal3.io",
					},
				},
			}
			Expect(cl.Create(ctx, claim)).To(Succeed())

			By("Waiting for the CAPI IPAddressClaim to get an address")
			Eventually(func(g Gomega) {
				retrieved := &capipamv1.IPAddressClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.AddressRef.Name).ToNot(BeEmpty())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			By("Verifying the allocated IP matches the annotation")
			updatedClaim := &capipamv1.IPAddressClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), updatedClaim)).To(Succeed())

			capiAddr := &capipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: claim.Namespace,
				Name:      updatedClaim.Status.AddressRef.Name,
			}, capiAddr)).To(Succeed())
			Expect(capiAddr.Spec.Address).To(Equal(requestedIP))
		})
	})

	Context("Pool exhaustion and recovery", func() {
		It("Should handle pool exhaustion and recover when IPs are freed", func() {
			cl := bootstrapClusterProxy.GetClient()

			By("Creating a small IPPool with only 2 IPs")
			startAddr := ipamv1.IPAddressStr("192.168.2.10")
			endAddr := ipamv1.IPAddressStr("192.168.2.11")
			subnet := ipamv1.IPSubnetStr("192.168.2.0/24")
			gateway := ipamv1.IPAddressStr("192.168.2.1")

			ipPool := &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool-exhaust",
					Namespace: namespace,
				},
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:  &startAddr,
							End:    &endAddr,
							Subnet: &subnet,
							Prefix: 24,
						},
					},
					Prefix:     24,
					Gateway:    &gateway,
					NamePrefix: "test-exhaust",
				},
			}
			Expect(cl.Create(ctx, ipPool)).To(Succeed())

			By("Allocating both IPs")
			claim1Name := "test-exhaust-claim-1"
			claim2Name := "test-exhaust-claim-2"
			claim1 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, claim1Name, namespace)
			claim2 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, claim2Name, namespace)

			By("Waiting for both claims to be allocated")
			for _, claim := range []*ipamv1.IPClaim{claim1, claim2} {
				Eventually(func(g Gomega) {
					retrieved := &ipamv1.IPClaim{}
					g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), retrieved)).To(Succeed())
					g.Expect(retrieved.Status.Address).ToNot(BeNil())
				}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
			}

			By("Creating a third claim that should fail due to exhaustion")
			claim3Name := "test-exhaust-claim-3"
			claim3 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, claim3Name, namespace)

			By("Verifying the third claim shows an error")
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim3), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.ErrorMessage).ToNot(BeNil())
				g.Expect(strings.ToLower(*retrieved.Status.ErrorMessage)).To(ContainSubstring("exhausted"))
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			By("Freeing an IP by deleting claim1")
			Expect(cl.Delete(ctx, claim1)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(claim1), &ipamv1.IPClaim{}))
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue())

			By("Deleting the exhausted claim3 and creating a new claim to verify recovery")
			Expect(cl.Delete(ctx, claim3)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(claim3), &ipamv1.IPClaim{}))
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue())

			claim4Name := "test-exhaust-claim-4"
			claim4 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, claim4Name, namespace)

			By("Verifying the new claim gets an IP from the freed address")
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim4), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
		})
	})

	Context("Multiple pools in one IPPool", func() {
		It("Should allocate from second pool when first is exhausted", func() {
			cl := bootstrapClusterProxy.GetClient()

			By("Creating an IPPool with two separate pools")
			start1 := ipamv1.IPAddressStr("10.0.1.10")
			end1 := ipamv1.IPAddressStr("10.0.1.10") // only 1 IP
			subnet1 := ipamv1.IPSubnetStr("10.0.1.0/24")
			start2 := ipamv1.IPAddressStr("10.0.2.10")
			end2 := ipamv1.IPAddressStr("10.0.2.20")
			subnet2 := ipamv1.IPSubnetStr("10.0.2.0/24")
			gw := ipamv1.IPAddressStr("10.0.1.1")

			ipPool := &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool-multipools",
					Namespace: namespace,
				},
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:  &start1,
							End:    &end1,
							Subnet: &subnet1,
							Prefix: 24,
						},
						{
							Start:  &start2,
							End:    &end2,
							Subnet: &subnet2,
							Prefix: 16,
						},
					},
					Prefix:     24,
					Gateway:    &gw,
					NamePrefix: "test-multi",
				},
			}
			Expect(cl.Create(ctx, ipPool)).To(Succeed())

			By("Allocating first IP (should come from pool[0])")
			claim1 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "test-multi-1", namespace)
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim1), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			updatedClaim1 := &ipamv1.IPClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim1), updatedClaim1)).To(Succeed())
			addr1 := &ipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: updatedClaim1.Status.Address.Namespace,
				Name:      updatedClaim1.Status.Address.Name,
			}, addr1)).To(Succeed())
			Expect(string(addr1.Spec.Address)).To(Equal("10.0.1.10"))

			By("Allocating second IP (pool[0] exhausted, should come from pool[1])")
			claim2 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "test-multi-2", namespace)
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			updatedClaim2 := &ipamv1.IPClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), updatedClaim2)).To(Succeed())
			addr2 := &ipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: updatedClaim2.Status.Address.Namespace,
				Name:      updatedClaim2.Status.Address.Name,
			}, addr2)).To(Succeed())
			Expect(string(addr2.Spec.Address)).To(Equal("10.0.2.10"))
			Expect(addr2.Spec.Prefix).To(Equal(16), "Should inherit prefix from pool[1]")
		})
	})

	Context("Pool-level gateway and DNS override", func() {
		It("Should use pool-level gateway and DNS when set, falling back to spec-level", func() {
			cl := bootstrapClusterProxy.GetClient()

			By("Creating an IPPool with spec-level and pool-level overrides")
			start1 := ipamv1.IPAddressStr("172.16.0.10")
			end1 := ipamv1.IPAddressStr("172.16.0.10")
			subnet1 := ipamv1.IPSubnetStr("172.16.0.0/24")
			poolGw := ipamv1.IPAddressStr("172.16.0.254")
			specGw := ipamv1.IPAddressStr("172.16.0.1")

			start2 := ipamv1.IPAddressStr("172.16.1.10")
			end2 := ipamv1.IPAddressStr("172.16.1.10")
			subnet2 := ipamv1.IPSubnetStr("172.16.1.0/24")

			ipPool := &ipamv1.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ippool-override",
					Namespace: namespace,
				},
				Spec: ipamv1.IPPoolSpec{
					Pools: []ipamv1.Pool{
						{
							Start:   &start1,
							End:     &end1,
							Subnet:  &subnet1,
							Prefix:  24,
							Gateway: &poolGw,
							DNSServers: []ipamv1.IPAddressStr{
								"1.1.1.1",
							},
						},
						{
							Start:  &start2,
							End:    &end2,
							Subnet: &subnet2,
							Prefix: 24,
							// No pool-level gateway/DNS → inherits spec-level
						},
					},
					Prefix:  24,
					Gateway: &specGw,
					DNSServers: []ipamv1.IPAddressStr{
						"8.8.8.8",
						"8.8.4.4",
					},
					NamePrefix: "test-override",
				},
			}
			Expect(cl.Create(ctx, ipPool)).To(Succeed())

			By("Allocating from pool[0] which has its own gateway and DNS")
			claim1 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "test-override-1", namespace)
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim1), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			updatedClaim1 := &ipamv1.IPClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim1), updatedClaim1)).To(Succeed())
			addr1 := &ipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: updatedClaim1.Status.Address.Namespace,
				Name:      updatedClaim1.Status.Address.Name,
			}, addr1)).To(Succeed())
			Expect(string(*addr1.Spec.Gateway)).To(Equal("172.16.0.254"), "Should use pool-level gateway")
			Expect(addr1.Spec.DNSServers).To(HaveLen(1))
			Expect(string(addr1.Spec.DNSServers[0])).To(Equal("1.1.1.1"), "Should use pool-level DNS")

			By("Allocating from pool[1] which inherits spec-level gateway and DNS")
			claim2 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "test-override-2", namespace)
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			updatedClaim2 := &ipamv1.IPClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim2), updatedClaim2)).To(Succeed())
			addr2 := &ipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: updatedClaim2.Status.Address.Namespace,
				Name:      updatedClaim2.Status.Address.Name,
			}, addr2)).To(Succeed())
			Expect(string(*addr2.Spec.Gateway)).To(Equal("172.16.0.1"), "Should use spec-level gateway")
			Expect(addr2.Spec.DNSServers).To(HaveLen(2))
			Expect(string(addr2.Spec.DNSServers[0])).To(Equal("8.8.8.8"), "Should use spec-level DNS")
		})
	})

	Context("IPPool status tracking", func() {
		It("Should track allocations in IPPool status", func() {
			cl := bootstrapClusterProxy.GetClient()

			By("Creating an IPPool")
			ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
				Name:       "test-ippool-status",
				Namespace:  namespace,
				Start:      "192.168.1.10",
				End:        "192.168.1.100",
				Subnet:     "192.168.1.0/24",
				Prefix:     24,
				Gateway:    "192.168.1.1",
				DNSServers: []string{"8.8.8.8", "8.8.4.4"},
				NamePrefix: "test-ip",
			})

			By("Creating two IPClaims")
			claim1 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "test-status-1", namespace)
			claim2 := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "test-status-2", namespace)

			By("Waiting for both allocations")
			for _, claim := range []*ipamv1.IPClaim{claim1, claim2} {
				Eventually(func(g Gomega) {
					retrieved := &ipamv1.IPClaim{}
					g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), retrieved)).To(Succeed())
					g.Expect(retrieved.Status.Address).ToNot(BeNil())
				}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
			}

			By("Verifying IPPool status has 2 allocations")
			Eventually(func(g Gomega) {
				pool := &ipamv1.IPPool{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipPool), pool)).To(Succeed())
				g.Expect(pool.Status.Allocations).To(HaveLen(2))
				g.Expect(pool.Status.LastUpdated).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			By("Deleting one claim and verifying status updates")
			Expect(cl.Delete(ctx, claim1)).To(Succeed())
			Eventually(func() bool {
				return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(claim1), &ipamv1.IPClaim{}))
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue())

			Eventually(func(g Gomega) {
				pool := &ipamv1.IPPool{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(ipPool), pool)).To(Succeed())
				g.Expect(pool.Status.Allocations).To(HaveLen(1))
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())
		})
	})

	Context("Mixed Metal3 and CAPI claims on same pool", func() {
		It("Should allocate unique IPs for both Metal3 and CAPI claims from the same pool", func() {
			cl := bootstrapClusterProxy.GetClient()

			By("Creating an IPPool")
			ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
				Name:       "test-ippool-mixed",
				Namespace:  namespace,
				Start:      "192.168.1.10",
				End:        "192.168.1.100",
				Subnet:     "192.168.1.0/24",
				Prefix:     24,
				Gateway:    "192.168.1.1",
				DNSServers: []string{"8.8.8.8", "8.8.4.4"},
				NamePrefix: "test-ip",
			})

			By("Creating a Metal3 IPClaim and a CAPI IPAddressClaim on the same pool")
			m3ClaimName := "test-mixed-m3"
			capiClaimName := "test-mixed-capi"
			m3Claim := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, m3ClaimName, namespace)
			capiClaim := createCAPIIPAddressClaim(ctx, bootstrapClusterProxy, ipPool.Name, capiClaimName, namespace)

			By("Waiting for both allocations")
			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(m3Claim), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			Eventually(func(g Gomega) {
				retrieved := &capipamv1.IPAddressClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(capiClaim), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.AddressRef.Name).ToNot(BeEmpty())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			By("Verifying both got different IPs")
			updatedM3 := &ipamv1.IPClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(m3Claim), updatedM3)).To(Succeed())
			m3Addr := &ipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: updatedM3.Status.Address.Namespace,
				Name:      updatedM3.Status.Address.Name,
			}, m3Addr)).To(Succeed())

			updatedCAPI := &capipamv1.IPAddressClaim{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(capiClaim), updatedCAPI)).To(Succeed())
			capiAddr := &capipamv1.IPAddress{}
			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: capiClaim.Namespace,
				Name:      updatedCAPI.Status.AddressRef.Name,
			}, capiAddr)).To(Succeed())

			m3IP := net.ParseIP(string(m3Addr.Spec.Address))
			capiIP := net.ParseIP(capiAddr.Spec.Address)
			Expect(m3IP).ToNot(BeNil())
			Expect(capiIP).ToNot(BeNil())
			Expect(m3IP.Equal(capiIP)).To(BeFalse(), "Metal3 and CAPI allocations should get different IPs")
		})
	})

	Context("IPPool deletion with active allocations", func() {
		It("Should block IPPool deletion via finalizer while claims exist", func() {
			cl := bootstrapClusterProxy.GetClient()

			By("Creating an IPPool and allocating an IP")
			ipPool := createIPPool(ctx, bootstrapClusterProxy, CreateIPPoolInput{
				Name:       "test-ippool-finalizer",
				Namespace:  namespace,
				Start:      "192.168.1.10",
				End:        "192.168.1.100",
				Subnet:     "192.168.1.0/24",
				Prefix:     24,
				Gateway:    "192.168.1.1",
				DNSServers: []string{"8.8.8.8", "8.8.4.4"},
				NamePrefix: "test-ip",
			})
			claim := createIPClaim(ctx, bootstrapClusterProxy, ipPool.Name, "test-finalizer", namespace)

			Eventually(func(g Gomega) {
				retrieved := &ipamv1.IPClaim{}
				g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(claim), retrieved)).To(Succeed())
				g.Expect(retrieved.Status.Address).ToNot(BeNil())
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(Succeed())

			By("Attempting to delete the IPPool (should be blocked by finalizer)")
			Expect(cl.Delete(ctx, ipPool)).To(Succeed()) // Delete is accepted but finalizer holds

			By("Verifying IPPool still exists (finalizer prevents actual deletion)")
			Consistently(func() bool {
				pool := &ipamv1.IPPool{}
				err := cl.Get(ctx, client.ObjectKeyFromObject(ipPool), pool)
				if err != nil {
					return false
				}
				return pool.DeletionTimestamp != nil
			}, "5s", "1s").Should(BeTrue(), "IPPool should have DeletionTimestamp but still exist")

			By("Deleting the claim to release the finalizer")
			Expect(cl.Delete(ctx, claim)).To(Succeed())

			By("Verifying IPPool is eventually deleted")
			Eventually(func() bool {
				return apierrors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(ipPool), &ipamv1.IPPool{}))
			}, e2eConfig.GetIntervals("default", "wait-ippool")...).Should(BeTrue(), "IPPool should be deleted after claims are removed")
		})
	})
})
