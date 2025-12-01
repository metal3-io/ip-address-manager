package e2e

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework"
)

var (
	specName            = "metal3"
	namespace           = "metal3"
	clusterName         = "test1"
	clusterctlLogFolder string
	targetCluster       framework.ClusterProxy
)

var _ = Describe("When testing basic cluster creation", Label("basic"), func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
	})

	It("Should create a workload cluster", func() {
		By("Apply BMH for workload cluster")
		ApplyBmh(ctx, clusterctlE2eConfig, bootstrapClusterProxy, namespace, specName)
		By("Fetching cluster configuration")
		k8sVersion := clusterctlE2eConfig.MustGetVariable("KUBERNETES_VERSION")
		By("Provision Workload cluster")
		targetCluster, _ = CreateTargetCluster(ctx, func() CreateTargetClusterInput {
			return CreateTargetClusterInput{
				E2EConfig:             clusterctlE2eConfig,
				BootstrapClusterProxy: bootstrapClusterProxy,
				SpecName:              specName,
				ClusterName:           clusterName,
				K8sVersion:            k8sVersion,
				KCPMachineCount:       int64(numberOfControlplane),
				WorkerMachineCount:    int64(numberOfWorkers),
				ClusterctlLogFolder:   clusterctlLogFolder,
				ClusterctlConfigPath:  clusterctlConfigPath,
				OSType:                osType,
				Namespace:             namespace,
			}
		})
	})

	AfterEach(func() {
		// Dump all the logs in the artifact folder
	})
})
