/*
Copyright 2019 The Kubernetes Authors.

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

package ipam

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-logr/logr"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var timeNow = metav1.Now()

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestManagers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Manager Suite")
}

var _ = BeforeSuite(func() {
	done := make(chan interface{})

	go func() {
		// klog.Background will automatically use the right logger.
		ctrl.SetLogger(klog.Background())

		By("bootstrapping test environment")
		capiCRDPaths, err := capiIPAMCRDPaths(context.Background())
		Expect(err).ToNot(HaveOccurred())

		testEnv = &envtest.Environment{
			CRDDirectoryPaths: append([]string{filepath.Join("..", "config", "crd", "bases")}, capiCRDPaths...),
			// Fail instead of starting without a CRD.
			ErrorIfCRDPathMissing: true,
		}

		cfg, err = testEnv.Start()
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg).ToNot(BeNil())

		err = ipamv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = capipamv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = clusterv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = apiextensionsv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		// +kubebuilder:scaffold:scheme

		k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient).ToNot(BeNil())
		err = k8sClient.Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "myns"},
		})
		Expect(err).NotTo(HaveOccurred())

		close(done)
	}()
	Eventually(done, 60).Should(BeClosed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

// -----------------------------------
// ------ Helper functions -----------
// -----------------------------------.

// capiIPAMCRDPaths returns the paths of the Cluster API IPAddressClaim and
// IPAddress CRDs, taken from the sigs.k8s.io/cluster-api version pinned in
// go.mod. The module is in the module cache already, as the tests import it.
func capiIPAMCRDPaths(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}", "sigs.k8s.io/cluster-api").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find the sigs.k8s.io/cluster-api module: %w", err)
	}
	crdDir := filepath.Join(strings.TrimSpace(string(out)), "core", "config", "crd", "bases")
	return []string{
		filepath.Join(crdDir, "ipam.cluster.x-k8s.io_ipaddressclaims.yaml"),
		filepath.Join(crdDir, "ipam.cluster.x-k8s.io_ipaddresses.yaml"),
	}, nil
}

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := ipamv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := capipamv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := clusterv1.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}
