package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	metal3api "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	irsov1alpha1 "github.com/metal3-io/ironic-standalone-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
	yaml2 "gopkg.in/yaml.v2"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	testexec "sigs.k8s.io/cluster-api/test/framework/exec"
	"sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type PowerState string

const (
	PoweredOn  PowerState = "on"
	PoweredOff PowerState = "off"

	filePerm600 = 0600
	filePerm750 = 0750

	artifactoryURL = "https://artifactory.nordix.org/artifactory/metal3/images/k8s"
	imagesURL      = "http://172.22.0.1/images"
	ironicImageDir = "/opt/metal3-dev-env/ironic/html/images"
	osTypeCentos   = "centos"
	osTypeUbuntu   = "ubuntu"
	osTypeLeap     = "opensuse-leap"
)

func isUndesiredState(currentState metal3api.ProvisioningState, undesiredStates []metal3api.ProvisioningState) bool {
	if undesiredStates == nil {
		return false
	}

	for _, state := range undesiredStates {
		if (state == "" && currentState == "") || currentState == state {
			return true
		}
	}
	return false
}

type WaitForBmhInProvisioningStateInput struct {
	Client          client.Client
	Bmh             metal3api.BareMetalHost
	State           metal3api.ProvisioningState
	UndesiredStates []metal3api.ProvisioningState
}

type WaitForBmhInOperationalStatusInput struct {
	Client          client.Client
	Bmh             metal3api.BareMetalHost
	State           metal3api.OperationalStatus
	UndesiredStates []metal3api.OperationalStatus
}

type PatchBMHForProvisioningInput struct {
	client         client.Client
	bmh            *metal3api.BareMetalHost
	bmc            BMC
	e2eConfig      *Config
	userDataSecret *corev1.SecretReference
}

func WaitForBmhInProvisioningState(ctx context.Context, input WaitForBmhInProvisioningStateInput, intervals ...interface{}) {
	Eventually(func(g Gomega) {
		bmh := metal3api.BareMetalHost{}
		key := types.NamespacedName{Namespace: input.Bmh.Namespace, Name: input.Bmh.Name}
		g.Expect(input.Client.Get(ctx, key, &bmh)).To(Succeed())

		currentStatus := bmh.Status.Provisioning.State

		// Check if the current state matches any of the undesired states
		if isUndesiredState(currentStatus, input.UndesiredStates) {
			StopTrying(fmt.Sprintf("BMH is in an unexpected state: %s", currentStatus)).Now()
		}

		g.Expect(currentStatus).To(Equal(input.State))
	}, intervals...).Should(Succeed())
}

func WaitForBmhInOperationalStatus(ctx context.Context, input WaitForBmhInOperationalStatusInput, intervals ...interface{}) {
	Eventually(func(g Gomega) {
		bmh := metal3api.BareMetalHost{}
		key := types.NamespacedName{Namespace: input.Bmh.Namespace, Name: input.Bmh.Name}
		g.Expect(input.Client.Get(ctx, key, &bmh)).To(Succeed())

		currentStatus := bmh.Status.OperationalStatus

		// Check if the current state matches any of the undesired states
		if slices.Contains(input.UndesiredStates, currentStatus) {
			StopTrying(fmt.Sprintf("BMH is in an unexpected state: %s", currentStatus)).Now()
		}

		g.Expect(currentStatus).To(Equal(input.State))
	}, intervals...).Should(Succeed())
}

// PatchBMHForProvisioning patches the BMH to set the image and root device hints.
// If setUserDataSecret is true, it also sets the user data secret for SSH access.
func PatchBMHForProvisioning(ctx context.Context, input PatchBMHForProvisioningInput) error {
	helper, err := patch.NewHelper(input.bmh, input.client)
	if err != nil {
		return err
	}
	input.bmh.Spec.Image = &metal3api.Image{
		URL:          input.e2eConfig.GetVariable("IMAGE_URL"),
		Checksum:     input.e2eConfig.GetVariable("IMAGE_CHECKSUM"),
		ChecksumType: metal3api.AutoChecksum,
	}
	input.bmh.Spec.RootDeviceHints = &input.bmc.RootDeviceHints
	if input.userDataSecret != nil {
		input.bmh.Spec.UserData = input.userDataSecret
	}
	return helper.Patch(ctx, input.bmh)
}

// DeleteBmhsInNamespace deletes all BMHs in the given namespace.
func DeleteBmhsInNamespace(ctx context.Context, deleter client.Client, namespace string) {
	bmh := metal3api.BareMetalHost{}
	opts := client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			Namespace: namespace,
		},
	}
	err := deleter.DeleteAllOf(ctx, &bmh, &opts)
	Expect(err).NotTo(HaveOccurred(), "Unable to delete BMHs")
}

// WaitForBmhDeletedInput is the input for WaitForBmhDeleted.
type WaitForBmhDeletedInput struct {
	Client          client.Client
	BmhName         string
	Namespace       string
	UndesiredStates []metal3api.ProvisioningState
}

// WaitForBmhDeleted waits until the BMH object has been deleted.
func WaitForBmhDeleted(ctx context.Context, input WaitForBmhDeletedInput, intervals ...interface{}) {
	Eventually(func(g Gomega) bool {
		bmh := &metal3api.BareMetalHost{}
		key := types.NamespacedName{Namespace: input.Namespace, Name: input.BmhName}
		err := input.Client.Get(ctx, key, bmh)

		// If BMH is not found, it's considered deleted, which is the desired outcome.
		if apierrors.IsNotFound(err) {
			return true
		}
		g.Expect(err).NotTo(HaveOccurred())

		currentStatus := bmh.Status.Provisioning.State

		// If the BMH is found, check for undesired states.
		if isUndesiredState(currentStatus, input.UndesiredStates) {
			StopTrying(fmt.Sprintf("BMH is in an unexpected state: %s", currentStatus)).Now()
		}

		return false
	}, intervals...).Should(BeTrue(), fmt.Sprintf("BMH %s in namespace %s should be deleted", input.BmhName, input.Namespace))
}

// WaitForNamespaceDeletedInput is the input for WaitForNamespaceDeleted.
type WaitForNamespaceDeletedInput struct {
	Getter    framework.Getter
	Namespace corev1.Namespace
}

// WaitForNamespaceDeleted waits until the namespace object has been deleted.
func WaitForNamespaceDeleted(ctx context.Context, input WaitForNamespaceDeletedInput, intervals ...interface{}) {
	Eventually(func() bool {
		namespace := &corev1.Namespace{}
		key := client.ObjectKey{
			Name: input.Namespace.Name,
		}
		return apierrors.IsNotFound(input.Getter.Get(ctx, key, namespace))
	}, intervals...).Should(BeTrue())
}

func Cleanup(ctx context.Context, clusterProxy framework.ClusterProxy, namespace *corev1.Namespace, cancelWatches context.CancelFunc, isNamespaced bool, intervals ...interface{}) {
	// Due to limitation in controller runtime watched namespaces cannot be deleted
	if !isNamespaced {
		// Trigger deletion of BMHs before deleting the namespace.
		// This way there should be no risk of BMO getting stuck trying to progress
		// and create HardwareDetails or similar, while the namespace is terminating.
		DeleteBmhsInNamespace(ctx, clusterProxy.GetClient(), namespace.Name)
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: clusterProxy.GetClient(),
			Name:    namespace.Name,
		})
		WaitForNamespaceDeleted(ctx, WaitForNamespaceDeletedInput{
			Getter:    clusterProxy.GetClient(),
			Namespace: *namespace,
		}, intervals...)
	}
	cancelWatches()
}

type WaitForBmhInPowerStateInput struct {
	Client client.Client
	Bmh    metal3api.BareMetalHost
	State  PowerState
}

func WaitForBmhInPowerState(ctx context.Context, input WaitForBmhInPowerStateInput, intervals ...interface{}) {
	Eventually(func(g Gomega) {
		bmh := metal3api.BareMetalHost{}
		key := types.NamespacedName{Namespace: input.Bmh.Namespace, Name: input.Bmh.Name}
		g.Expect(input.Client.Get(ctx, key, &bmh)).To(Succeed())
		g.Expect(bmh.Status.PoweredOn).To(Equal(input.State == PoweredOn))
	}, intervals...).Should(Succeed())
}

func BuildKustomizeManifest(source string) ([]byte, error) {
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	fSys := filesys.MakeFsOnDisk()
	resources, err := kustomizer.Run(fSys, source)
	if err != nil {
		return nil, err
	}
	return resources.AsYaml()
}

func CreateSecret(ctx context.Context, client client.Client, secretNamespace, secretName string, data map[string]string) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		StringData: data,
	}

	Expect(client.Create(ctx, &secret)).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create secret '%s/%s'", secretNamespace, secretName))
}

func executeSSHCommand(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("failed to execute command '%s': %v", command, err)
	}

	return string(output), nil
}

// HasRootOnDisk parses the output from 'df -h' and checks if the root filesystem is on a disk (as opposed to tmpfs).
func HasRootOnDisk(output string) bool {
	lines := strings.Split(output, "\n")

	for _, line := range lines[1:] { // Skip header line
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 { //nolint: mnd
			continue // Skip malformed lines
		}

		// When booting from memory or live-ISO we can have root on tmpfs or airootfs
		if fields[5] == "/" && !(strings.Contains(fields[0], "tmpfs") || strings.Contains(fields[0], "airootfs")) {
			return true
		}
	}

	return false
}

// IsBootedFromDisk checks if the system, accessed via the provided ssh.Client, is booted
// from a disk. It executes the 'df -h' command on the remote system to analyze the filesystem
// layout. In the case of a disk boot, the output includes a disk-based root filesystem
// (e.g., '/dev/vda1'). Conversely, in the case of a Live-ISO boot, the primary filesystems
// are memory-based (tmpfs).
func IsBootedFromDisk(client *ssh.Client) (bool, error) {
	cmd := "df -h"
	output, err := executeSSHCommand(client, cmd)
	if err != nil {
		return false, fmt.Errorf("error executing 'df -h': %w", err)
	}

	bootedFromDisk := HasRootOnDisk(output)
	if bootedFromDisk {
		Logf("System is booted from a disk.")
	} else {
		Logf("System is booted from a live ISO.")
	}

	return bootedFromDisk, nil
}

func EstablishSSHConnection(e2eConfig *Config, ipAddress string) *ssh.Client {
	user := e2eConfig.GetVariable("SSH_USERNAME")
	keyPath := e2eConfig.GetVariable("SSH_PRIV_KEY")
	key, err := os.ReadFile(keyPath)
	Expect(err).NotTo(HaveOccurred(), "unable to read private key")
	signer, err := ssh.ParsePrivateKey(key)
	Expect(err).NotTo(HaveOccurred(), "unable to parse private key")
	auth := ssh.PublicKeys(signer)
	address := fmt.Sprintf("%s:%s", ipAddress, e2eConfig.GetVariable("SSH_PORT"))

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // #nosec G106
	}

	var client *ssh.Client
	Eventually(func() error {
		client, err = ssh.Dial("tcp", address, config)
		return err
	}, e2eConfig.GetIntervals("default", "wait-connect-ssh")...).Should(Succeed(), "Failed to establish SSH connection")

	return client
}

// PerformSSHBootCheck performs an SSH check to verify the node's boot source.
// The `expectedBootMode` parameter should be "disk" or "memory".
// The `auth` parameter is an ssh.AuthMethod for authentication.
func PerformSSHBootCheck(e2eConfig *Config, expectedBootMode string, ipAddress string) {
	client := EstablishSSHConnection(e2eConfig, ipAddress)
	defer func() {
		if client != nil {
			client.Close()
		}
	}()

	bootedFromDisk, err := IsBootedFromDisk(client)
	Expect(err).NotTo(HaveOccurred(), "Error in verifying boot mode")

	// Compare actual boot source with expected
	isExpectedBootMode := (expectedBootMode == "disk" && bootedFromDisk) ||
		(expectedBootMode == "memory" && !bootedFromDisk)
	Expect(isExpectedBootMode).To(BeTrue(), fmt.Sprintf("Expected booting from %s, but found different mode", expectedBootMode))
}

// BuildAndApplyKustomizationInput provides input for BuildAndApplyKustomize().
// If WaitForDeployment and/or WatchDeploymentLogs is set to true, then DeploymentName
// and DeploymentNamespace are expected.
type BuildAndApplyKustomizationInput struct {
	// Path to the kustomization to build
	Kustomization string

	ClusterProxy framework.ClusterProxy

	// If this is set to true. Perform a wait until the deployment specified by
	// DeploymentName and DeploymentNamespace is available or WaitIntervals is timed out
	WaitForDeployment bool

	// If this is set to true. Set up a log watcher for the deployment specified by
	// DeploymentName and DeploymentNamespace
	WatchDeploymentLogs bool

	// DeploymentName and DeploymentNamespace specified a deployment that will be waited and/or logged
	DeploymentName      string
	DeploymentNamespace string

	// Path to store the deployment logs
	LogPath string

	// Intervals to use in checking and waiting for the deployment
	WaitIntervals []interface{}
}

func (input *BuildAndApplyKustomizationInput) validate() error {
	// If neither WaitForDeployment nor WatchDeploymentLogs is true, we don't need to validate the input
	if !input.WaitForDeployment && !input.WatchDeploymentLogs {
		return nil
	}
	if input.WaitForDeployment && input.WaitIntervals == nil {
		return errors.New("WaitIntervals is expected if WaitForDeployment is set to true")
	}
	if input.WatchDeploymentLogs && input.LogPath == "" {
		return errors.New("LogPath is expected if WatchDeploymentLogs is set to true")
	}
	if input.DeploymentName == "" || input.DeploymentNamespace == "" {
		return errors.New("DeploymentName and DeploymentNamespace are expected if WaitForDeployment or WatchDeploymentLogs is true")
	}
	return nil
}

// BuildAndApplyKustomization takes input from BuildAndApplyKustomizationInput. It builds the provided kustomization
// and apply it to the cluster provided by clusterProxy.
func BuildAndApplyKustomization(ctx context.Context, input *BuildAndApplyKustomizationInput) error {
	var err error
	if err = input.validate(); err != nil {
		return err
	}
	kustomization := input.Kustomization
	clusterProxy := input.ClusterProxy
	manifest, err := BuildKustomizeManifest(kustomization)
	if err != nil {
		return err
	}

	err = clusterProxy.CreateOrUpdate(ctx, manifest)
	if err != nil {
		return err
	}

	if !input.WaitForDeployment && !input.WatchDeploymentLogs {
		return nil
	}

	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.DeploymentName,
			Namespace: input.DeploymentNamespace,
		},
	}

	if input.WaitForDeployment {
		// Wait for the deployment to become available
		framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter:     clusterProxy.GetClient(),
			Deployment: deployment,
		}, input.WaitIntervals...)
	}

	if input.WatchDeploymentLogs {
		// Set up log watcher
		framework.WatchDeploymentLogsByName(ctx, framework.WatchDeploymentLogsByNameInput{
			GetLister:  clusterProxy.GetClient(),
			Cache:      clusterProxy.GetCache(ctx),
			ClientSet:  clusterProxy.GetClientSet(),
			Deployment: deployment,
			LogPath:    input.LogPath,
		})
	}
	return nil
}

func DeploymentRolledOut(ctx context.Context, clusterProxy framework.ClusterProxy, name string, namespace string, desiredGeneration int64) bool {
	clientSet := clusterProxy.GetClientSet()
	deploy, err := clientSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	if deploy != nil {
		// When the number of replicas is equal to the number of available and updated
		// replicas, we know that only "new" pods are running. When we also
		// have the desired number of replicas and a new enough generation, we
		// know that the rollout is complete.
		return (deploy.Status.UpdatedReplicas == *deploy.Spec.Replicas) &&
			(deploy.Status.AvailableReplicas == *deploy.Spec.Replicas) &&
			(deploy.Status.Replicas == *deploy.Spec.Replicas) &&
			(deploy.Status.ObservedGeneration >= desiredGeneration)
	}
	return false
}

// KubectlDelete shells out to kubectl delete.
func KubectlDelete(ctx context.Context, kubeconfigPath string, resources []byte, args ...string) error {
	aargs := append([]string{"delete", "--kubeconfig", kubeconfigPath, "-f", "-"}, args...)
	rbytes := bytes.NewReader(resources)
	deleteCmd := testexec.NewCommand(
		testexec.WithCommand("kubectl"),
		testexec.WithArgs(aargs...),
		testexec.WithStdin(rbytes),
	)

	log.Printf("Running kubectl %s\n", strings.Join(aargs, " "))
	stdout, stderr, err := deleteCmd.Run(ctx)
	log.Printf("stderr:\n%s\n", string(stderr))
	log.Printf("stdout:\n%s\n", string(stdout))
	return err
}

// BuildAndRemoveKustomization builds the provided kustomization to resources and removes them from the cluster
// provided by clusterProxy.
func BuildAndRemoveKustomization(ctx context.Context, kustomization string, clusterProxy framework.ClusterProxy) error {
	manifest, err := BuildKustomizeManifest(kustomization)
	if err != nil {
		return err
	}
	return KubectlDelete(ctx, clusterProxy.GetKubeconfigPath(), manifest)
}

// AnnotateBmh annotates BaremetalHost with a given key and value.
func AnnotateBmh(ctx context.Context, client client.Client, host metal3api.BareMetalHost, key string, value *string) {
	helper, err := patch.NewHelper(&host, client)
	Expect(err).NotTo(HaveOccurred())
	annotations := host.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if value == nil {
		delete(annotations, key)
	} else {
		annotations[key] = *value
	}
	host.SetAnnotations(annotations)
	Expect(helper.Patch(ctx, &host)).To(Succeed())
}

func Logf(format string, a ...interface{}) {
	fmt.Fprintf(GinkgoWriter, "INFO: "+format+"\n", a...)
}

// FlakeAttempt retries the given function up to attempts times.
func FlakeAttempt(attempts int, f func() error) error {
	var err error
	for i := range attempts {
		err = f()
		if err == nil {
			return nil
		}
		Logf("Attempt %d failed: %v", i+1, err)
	}
	return err
}

// GetKubeconfigPath returns the path to the kubeconfig file.
func GetKubeconfigPath() string {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("HOME") + "/.kube/config"
	}
	return kubeconfigPath
}

// DumpObj tries to dump the given object into a file in YAML format.
func dumpObj[T any](obj T, name string, path string) {
	objYaml, err := yaml2.Marshal(obj)
	Expect(err).ToNot(HaveOccurred(), "Failed to marshal %s", name)
	fullpath := filepath.Join(path, name)
	filepath.Clean(fullpath)
	Expect(os.MkdirAll(filepath.Dir(fullpath), filePerm750)).To(Succeed(), "Failed to create folders on path %s", filepath.Dir(fullpath))
	f, err := os.OpenFile(fullpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerm600)
	Expect(err).ToNot(HaveOccurred(), "Failed to open file with path %s", fullpath)
	defer f.Close()
	Expect(os.WriteFile(f.Name(), objYaml, filePerm600)).To(Succeed())
}

// DumpCRDs fetches all CRDs and filedumps them.
func dumpCRDS(ctx context.Context, cli client.Client, artifactFolder string) {
	crds := apiextensionsv1.CustomResourceDefinitionList{}
	Expect(cli.List(ctx, &crds)).To(Succeed())
	for _, crd := range crds.Items {
		dumpObj(crd, crd.ObjectMeta.Name, artifactFolder)
		crGVK, _ := schema.ParseKindArg(crd.Status.AcceptedNames.ListKind + "." + crd.Status.StoredVersions[0] + "." + crd.Spec.Group)
		crs := &unstructured.UnstructuredList{}
		crs.SetGroupVersionKind(*crGVK)
		Expect(cli.List(ctx, crs)).To(Succeed())
		for _, cr := range crs.Items {
			dumpObj(cr, cr.GetName(), path.Join(artifactFolder, crd.Spec.Names.Plural))
		}
	}
}

// DumpResources dumps resources related to BMO e2e tests as YAML.
func DumpResources(ctx context.Context, e2eConfig *Config, clusterProxy framework.ClusterProxy, artifactFolder string) {
	dumpCRDS(ctx, clusterProxy.GetClient(), filepath.Join(artifactFolder, "crd"))
	if e2eConfig.GetBoolVariable("FETCH_IRONIC_NODES") {
		dumpIronicNodes(ctx, e2eConfig, artifactFolder)
	}
}

// dumpIronicNodes dumps the nodes in ironic's view into json file inside the provided artifactFolder.
func dumpIronicNodes(ctx context.Context, e2eConfig *Config, artifactFolder string) {
	ironicProvisioningIP := e2eConfig.GetVariable("IRONIC_PROVISIONING_IP")
	ironicProvisioningPort := e2eConfig.GetVariable("IRONIC_PROVISIONING_PORT")
	ironicURL := fmt.Sprintf("https://%s/v1/nodes", net.JoinHostPort(ironicProvisioningIP, ironicProvisioningPort))
	username := e2eConfig.GetVariable("IRONIC_USERNAME")
	password := e2eConfig.GetVariable("IRONIC_PASSWORD")

	// Create HTTP client with TLS settings
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // #nosec G402 Skip verification as we are using self-signed certificates
	}
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	// Create the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ironicURL, http.NoBody)
	Expect(err).ToNot(HaveOccurred(), "Failed to create request")

	// Set basic auth header
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Add("Authorization", "Basic "+auth)

	// Make the request
	resp, err := client.Do(req)
	Expect(err).ToNot(HaveOccurred(), "Failed to send request")
	Expect(resp.StatusCode).To(Equal(http.StatusOK), fmt.Sprintf("Unexpected Status Code: %d", resp.StatusCode))

	defer resp.Body.Close()
	// Read and output the response
	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred(), "Failed to read response body")

	var logOutput bytes.Buffer

	// Format the JSON with indentation
	err = json.Indent(&logOutput, body, "", "    ")
	Expect(err).ToNot(HaveOccurred(), "Error formatting JSON")

	file, err := os.Create(path.Join(artifactFolder, "ironic-nodes.json"))
	Expect(err).ToNot(HaveOccurred(), "Error creating file")
	defer file.Close()

	// Write indented JSON to file
	_, err = file.Write(logOutput.Bytes())
	Expect(err).ToNot(HaveOccurred(), "Error writing JSON to file")
}

// WaitForIronicReady waits until the given Ironic resource has Ready condition = True.
func WaitForIronicReady(ctx context.Context, input WaitForIronicInput) {
	Logf("Waiting for Ironic %q to be Ready", input.Name)

	Eventually(func(g Gomega) {
		ironic := &irsov1alpha1.Ironic{}
		err := input.Client.Get(ctx, client.ObjectKey{
			Namespace: input.Namespace,
			Name:      input.Name,
		}, ironic)
		g.Expect(err).ToNot(HaveOccurred())

		ready := false
		for _, cond := range ironic.Status.Conditions {
			if cond.Type == string(irsov1alpha1.IronicStatusReady) && cond.Status == metav1.ConditionTrue && ironic.Status.InstalledVersion != "" {
				ready = true
				break
			}
		}
		g.Expect(ready).To(BeTrue(), "Ironic %q is not Ready yet", input.Name)
	}, input.Intervals...).Should(Succeed())

	Logf("Ironic %q is Ready", input.Name)
}

// WaitForIronicInput bundles the parameters for WaitForIronicReady.
type WaitForIronicInput struct {
	Client    client.Client
	Name      string
	Namespace string
	Intervals []interface{} // e.g. []interface{}{time.Minute * 15, time.Second * 5}
}

type WaitForNumInput struct {
	Client    client.Client
	Options   []client.ListOption
	Replicas  int
	Intervals []any
}

func ApplyBmh(ctx context.Context, e2eConfig *clusterctl.E2EConfig, clusterProxy framework.ClusterProxy, clusterNamespace string, specName string) {
	workingDir := "/opt/metal3-dev-env/"
	numNodes := int(*e2eConfig.MustGetInt32PtrVariable("NUM_NODES"))
	// Apply secrets and bmhs for [node_0 and node_1] in the management cluster to host the target management cluster
	for i := range numNodes {
		resource, err := os.ReadFile(filepath.Join(workingDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(CreateOrUpdateWithNamespace(ctx, clusterProxy, resource, clusterNamespace)).ShouldNot(HaveOccurred())
	}
	clusterClient := clusterProxy.GetClient()
	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
	WaitForNumBmhInState(ctx, metal3api.StateAvailable, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(clusterNamespace)},
		Replicas:  numNodes,
		Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
	})
	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(clusterNamespace))
}

// CreateOrUpdateWithNamespace creates or updates objects using the clusterProxy client with specific namespace.
func CreateOrUpdateWithNamespace(ctx context.Context, p framework.ClusterProxy, resources []byte, namespace string) error {
	Expect(ctx).NotTo(BeNil(), "ctx is required for CreateOrUpdate")
	Expect(resources).NotTo(BeNil(), "resources is required for CreateOrUpdate")
	objs, err := yaml.ToUnstructured(resources)
	if err != nil {
		return err
	}
	existingObject := &unstructured.Unstructured{}
	var retErrs []error
	for _, o := range objs {
		o.SetNamespace(namespace)
		objectKey := types.NamespacedName{
			Name:      o.GetName(),
			Namespace: o.GetNamespace(),
		}
		existingObject.SetAPIVersion(o.GetAPIVersion())
		existingObject.SetKind(o.GetKind())
		if err := p.GetClient().Get(ctx, objectKey, existingObject); err != nil {
			// Expected error -- if the object does not exist, create it
			if apierrors.IsNotFound(err) {
				if err = p.GetClient().Create(ctx, &o); err != nil {
					retErrs = append(retErrs, err)
				}
			} else {
				retErrs = append(retErrs, err)
			}
		} else {
			o.SetResourceVersion(existingObject.GetResourceVersion())
			if err := p.GetClient().Update(ctx, &o); err != nil {
				retErrs = append(retErrs, err)
			}
		}
	}
	return kerrors.NewAggregate(retErrs)
}

// ListBareMetalHosts logs the names, provisioning status, consumer and power status
// of all BareMetalHosts matching the opts. Similar to kubectl get baremetalhosts.
func ListBareMetalHosts(ctx context.Context, c client.Client, opts ...client.ListOption) {
	bmhs := metal3api.BareMetalHostList{}
	Expect(c.List(ctx, &bmhs, opts...)).To(Succeed())

	rows := make([][]string, len(bmhs.Items)+1)
	// Add column names
	rows[0] = []string{"Name:", "Status:", "Consumer:", "Online:"}

	for i, bmh := range bmhs.Items {
		consumer := ""
		if bmh.Spec.ConsumerRef != nil {
			consumer = bmh.Spec.ConsumerRef.Name
		}
		rows[i+1] = []string{bmh.GetName(), fmt.Sprint(bmh.Status.Provisioning.State), consumer, strconv.FormatBool(bmh.Status.PoweredOn)}
	}
	logTable("Listing BareMetalHosts", rows)
}

// WaitForNumBmhInState will wait for the given number of BMHs to be in the given state.
func WaitForNumBmhInState(ctx context.Context, state metal3api.ProvisioningState, input WaitForNumInput) {
	Logf("Waiting for %d BMHs to be in %s state", input.Replicas, state)
	Eventually(func(g Gomega) {
		bmhList := metal3api.BareMetalHostList{}
		g.Expect(input.Client.List(ctx, &bmhList, input.Options...)).To(Succeed())
		g.Expect(FilterBmhsByProvisioningState(bmhList.Items, state)).To(HaveLen(input.Replicas))
	}, input.Intervals...).Should(Succeed())
	ListBareMetalHosts(ctx, input.Client, input.Options...)
}

// FilterBmhsByProvisioningState returns a filtered list of BaremetalHost objects in certain provisioning state.
func FilterBmhsByProvisioningState(bmhs []metal3api.BareMetalHost, state metal3api.ProvisioningState) (result []metal3api.BareMetalHost) {
	for _, bmh := range bmhs {
		if bmh.Status.Provisioning.State == state {
			result = append(result, bmh)
		}
	}
	return
}

// logTable print a formatted table into the e2e logs.
func logTable(title string, rows [][]string) {
	getRowFormatted := func(row []string) string {
		rowFormatted := ""
		for i := range row {
			rowFormatted = fmt.Sprintf("%s\t%s\t", rowFormatted, row[i])
		}
		return rowFormatted
	}
	w := tabwriter.NewWriter(GinkgoWriter, 0, 0, 2, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, title)
	for _, r := range rows {
		fmt.Fprintln(w, getRowFormatted(r))
	}
	fmt.Fprintln(w, "")
	w.Flush()
}

type CreateTargetClusterInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	SpecName              string
	ClusterName           string
	K8sVersion            string
	KCPMachineCount       int64
	WorkerMachineCount    int64
	ClusterctlLogFolder   string
	ClusterctlConfigPath  string
	OSType                string
	Namespace             string
}

func CreateTargetCluster(ctx context.Context, inputGetter func() CreateTargetClusterInput) (framework.ClusterProxy, *clusterctl.ApplyClusterTemplateAndWaitResult) {
	By("Creating a high available cluster")
	input := inputGetter()
	imageURL, imageChecksum := EnsureImage(input.K8sVersion)
	os.Setenv("IMAGE_RAW_CHECKSUM", imageChecksum)
	os.Setenv("IMAGE_RAW_URL", imageURL)
	controlPlaneMachineCount := input.KCPMachineCount
	workerMachineCount := input.WorkerMachineCount
	result := clusterctl.ApplyClusterTemplateAndWaitResult{}
	clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
		ClusterProxy: input.BootstrapClusterProxy,
		ConfigCluster: clusterctl.ConfigClusterInput{
			LogFolder:                input.ClusterctlLogFolder,
			ClusterctlConfigPath:     input.ClusterctlConfigPath,
			KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
			InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
			Flavor:                   input.OSType,
			Namespace:                input.Namespace,
			ClusterName:              input.ClusterName,
			KubernetesVersion:        input.K8sVersion,
			ControlPlaneMachineCount: &controlPlaneMachineCount,
			WorkerMachineCount:       &workerMachineCount,
		},
		WaitForClusterIntervals:      input.E2EConfig.GetIntervals(input.SpecName, "wait-cluster"),
		WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-control-plane"),
		WaitForMachineDeployments:    input.E2EConfig.GetIntervals(input.SpecName, "wait-worker-nodes"),
	}, &result)
	targetCluster := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace, result.Cluster.Name)
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister:      targetCluster.GetClient(),
		ListOptions: &client.ListOptions{LabelSelector: labels.Everything(), Namespace: "kube-system"},
		Condition:   framework.PhasePodCondition(corev1.PodRunning),
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-all-pod-to-be-running-on-target-cluster")...)
	return targetCluster, &result
}

func EnsureImage(k8sVersion string) (imageURL string, imageChecksum string) {
	osType := strings.ToLower(os.Getenv("OS"))
	Expect(osType).To(BeElementOf([]string{osTypeUbuntu, osTypeCentos, osTypeLeap}))
	imageNamePrefix := ""
	switch osType {
	case osTypeCentos:
		imageNamePrefix = "CENTOS_9_NODE_IMAGE_K8S"
	case osTypeUbuntu:
		imageNamePrefix = "UBUNTU_24.04_NODE_IMAGE_K8S"
	case osTypeLeap:
		imageNamePrefix = "LEAP_15_6_NODE_IMAGE_K8S"
	}
	imageName := fmt.Sprintf("%s_%s.qcow2", imageNamePrefix, k8sVersion)
	rawImageName := fmt.Sprintf("%s_%s-raw.img", imageNamePrefix, k8sVersion)
	imageLocation := fmt.Sprintf("%s_%s/", artifactoryURL, k8sVersion)
	imageURL = fmt.Sprintf("%s/%s", imagesURL, rawImageName)
	imageChecksum = fmt.Sprintf("%s/%s.sha256sum", imagesURL, rawImageName)

	// Check if node image with upgraded k8s version exist, if not download it
	imagePath := filepath.Join(ironicImageDir, imageName)
	rawImagePath := filepath.Join(ironicImageDir, rawImageName)
	if _, err := os.Stat(rawImagePath); err == nil {
		Logf("Local image %v already exists", rawImagePath)
	} else if os.IsNotExist(err) {
		Logf("Local image %v is not found \nDownloading..", rawImagePath)
		err = DownloadFile(imagePath, fmt.Sprintf("%s/%s", imageLocation, imageName))
		Expect(err).ToNot(HaveOccurred())
		cmd := exec.Command("qemu-img", "convert", "-O", "raw", imagePath, rawImagePath) // #nosec G204:gosec
		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred())
		var sha256sum []byte
		sha256sum, err = getSha256Hash(rawImagePath)
		Expect(err).ToNot(HaveOccurred())
		formattedSha256sum := hex.EncodeToString(sha256sum)
		err = os.WriteFile(fmt.Sprintf("%s/%s.sha256sum", ironicImageDir, rawImageName), []byte(formattedSha256sum), 0544) //#nosec G306:gosec
		Expect(err).ToNot(HaveOccurred())
		Logf("Image: %v downloaded", rawImagePath)
	} else {
		fmt.Fprintf(GinkgoWriter, "ERROR: %v\n", err)
		os.Exit(1)
	}
	return imageURL, imageChecksum
}

// DownloadFile will download a url and store it in local filepath.
func DownloadFile(filePath string, url string) error {
	// TODO: Lets change the wget to use go's native http client when network
	// more resilient
	cmd := exec.Command("wget", "-O", filePath, url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wget failed: %w, output: %s", err, string(output))
	}
	return nil
}

// getSha256Hash return sha256 hash of given file.
func getSha256Hash(filename string) ([]byte, error) {
	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = file.Close()
		Expect(err).ToNot(HaveOccurred(), "Error closing file: "+filename)
	}()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
