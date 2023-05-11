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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/metal3-io/ip-address-manager/controllers"
	"github.com/metal3-io/ip-address-manager/ipam"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	// +kubebuilder:scaffold:imports
)

type TLSVersion string

// Constants for TLS versions.
const (
	TLSVersion12 TLSVersion = "TLS12"
	TLSVersion13 TLSVersion = "TLS13"
)

type TLSOptions struct {
	TLSMaxVersion   string
	TLSMinVersion   string
	TLSCipherSuites string
}

var (
	myscheme             = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	metricsBindAddr      string
	enableLeaderElection bool
	syncPeriod           time.Duration
	ippoolConcurrency    int
	restConfigQPS        float64
	restConfigBurst      int
	webhookPort          int
	healthAddr           string
	watchNamespace       string
	webhookCertDir       string
	watchFilterValue     string
	logOptions           = logs.NewOptions()
	tlsOptions           = TLSOptions{}
	tlsSupportedVersions = []string{"TLS12", "TLS13"}
)

func init() {
	_ = scheme.AddToScheme(myscheme)
	_ = ipamv1.AddToScheme(myscheme)
	_ = clusterv1.AddToScheme(myscheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	flag.StringVar(&metricsBindAddr, "metrics-bind-addr", "localhost:8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&watchNamespace, "namespace", "",
		"Namespace that the controller watches to reconcile IPAM objects. If unspecified, the controller watches for IPAM objects across all namespaces.")
	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")
	flag.IntVar(&webhookPort, "webhook-port", 9443,
		"Webhook Server port")
	flag.StringVar(
		&webhookCertDir,
		"webhook-cert-dir",
		"/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.",
	)
	flag.StringVar(
		&watchFilterValue,
		"watch-filter",
		"",
		fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel),
	)
	flag.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")

	flag.IntVar(&ippoolConcurrency, "ippool-concurrency", 10,
		"Number of ippools to process simultaneously")

	flag.Float64Var(&restConfigQPS, "kube-api-qps", 20,
		"Maximum queries per second from the controller client to the Kubernetes API server. Default 20")

	flag.IntVar(&restConfigBurst, "kube-api-burst", 30,
		"Maximum number of queries that should be allowed in one burst from the controller client to the Kubernetes API server. Default 30")
	flag.StringVar(&tlsOptions.TLSMinVersion, "tls-min-version", "TLS12",
		"The minimum TLS version in use by the webhook server.\n"+
			fmt.Sprintf("Possible values are %s.", strings.Join(tlsSupportedVersions, ", ")),
	)

	flag.StringVar(&tlsOptions.TLSMaxVersion, "tls-max-version", "TLS13",
		"The maximum TLS version in use by the webhook server.\n"+
			fmt.Sprintf("Possible values are %s.", strings.Join(tlsSupportedVersions, ", ")),
	)

	tlsCipherPreferredValues := cliflag.PreferredTLSCipherNames()
	tlsCipherInsecureValues := cliflag.InsecureTLSCipherNames()
	flag.StringVar(&tlsOptions.TLSCipherSuites, "tls-cipher-suites", "",
		"Comma-separated list of cipher suites for the webhook server. "+
			"If omitted, the default Go cipher suites will be used. \n"+
			"Preferred values: "+strings.Join(tlsCipherPreferredValues, ", ")+". \n"+
			"Insecure values: "+strings.Join(tlsCipherInsecureValues, ", ")+".")
	flag.Parse()

	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// klog.Background will automatically use the right logger.
	ctrl.SetLogger(klog.Background())
	tlsOptionOverrides, err := GetTLSOptionOverrideFuncs(tlsOptions)
	if err != nil {
		setupLog.Error(err, "unable to add TLS settings to the webhook server")
		os.Exit(1)
	}

	restConfig := ctrl.GetConfigOrDie()
	restConfig.QPS = float32(restConfigQPS)
	restConfig.Burst = restConfigBurst
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                     myscheme,
		MetricsBindAddress:         metricsBindAddr,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "controller-leader-election-ipam-capm3",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		SyncPeriod:                 &syncPeriod,
		Port:                       webhookPort,
		HealthProbeBindAddress:     healthAddr,
		Namespace:                  watchNamespace,
		CertDir:                    webhookCertDir,
		TLSOpts:                    tlsOptionOverrides,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	setupChecks(mgr)
	setupReconcilers(ctx, mgr)
	setupWebhooks(mgr)

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupChecks(mgr ctrl.Manager) {
	if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}

func setupReconcilers(ctx context.Context, mgr ctrl.Manager) {
	if err := (&controllers.IPPoolReconciler{
		Client:           mgr.GetClient(),
		ManagerFactory:   ipam.NewManagerFactory(mgr.GetClient()),
		Log:              ctrl.Log.WithName("controllers").WithName("IPPool"),
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, concurrency(ippoolConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IPPoolReconciler")
		os.Exit(1)
	}
}

func setupWebhooks(mgr ctrl.Manager) {
	if err := (&ipamv1.IPPool{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IPPool")
		os.Exit(1)
	}

	if err := (&ipamv1.IPAddress{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IPAddress")
		os.Exit(1)
	}

	if err := (&ipamv1.IPClaim{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IPClaim")
		os.Exit(1)
	}
}

func concurrency(c int) controller.Options {
	return controller.Options{MaxConcurrentReconciles: c}
}

// GetTLSOptionOverrideFuncs returns a list of TLS configuration overrides to be used
// by the webhook server.
func GetTLSOptionOverrideFuncs(options TLSOptions) ([]func(*tls.Config), error) {
	var tlsOptions []func(config *tls.Config)

	tlsMinVersion, err := GetTLSVersion(options.TLSMinVersion)
	if err != nil {
		return nil, err
	}

	tlsMaxVersion, err := GetTLSVersion(options.TLSMaxVersion)
	if err != nil {
		return nil, err
	}

	if tlsMaxVersion != 0 && tlsMinVersion > tlsMaxVersion {
		return nil, fmt.Errorf("TLS version flag min version (%s) is greater than max version (%s)",
			options.TLSMinVersion, options.TLSMaxVersion)
	}

	tlsOptions = append(tlsOptions, func(cfg *tls.Config) {
		cfg.MinVersion = tlsMinVersion
	})

	tlsOptions = append(tlsOptions, func(cfg *tls.Config) {
		cfg.MaxVersion = tlsMaxVersion
	})
	// Cipher suites should not be set if empty.
	if options.TLSMinVersion == string(TLSVersion13) &&
		options.TLSMaxVersion == string(TLSVersion13) &&
		options.TLSCipherSuites != "" {
		setupLog.Info("warning: Cipher suites should not be set for TLS version 1.3. Ignoring ciphers")
		options.TLSCipherSuites = ""
	}

	if options.TLSCipherSuites != "" {
		tlsCipherSuites := strings.Split(options.TLSCipherSuites, ",")
		suites, err := cliflag.TLSCipherSuites(tlsCipherSuites)
		if err != nil {
			return nil, err
		}

		insecureCipherValues := cliflag.InsecureTLSCipherNames()
		for _, cipher := range tlsCipherSuites {
			for _, insecureCipherName := range insecureCipherValues {
				if insecureCipherName == cipher {
					setupLog.Info(fmt.Sprintf("warning: use of insecure cipher '%s' detected.", cipher))
				}
			}
		}
		tlsOptions = append(tlsOptions, func(cfg *tls.Config) {
			cfg.CipherSuites = suites
		})
	}

	return tlsOptions, nil
}

// GetTLSVersion returns the corresponding tls.Version or error.
func GetTLSVersion(version string) (uint16, error) {
	var v uint16

	switch version {
	case string(TLSVersion12):
		v = tls.VersionTLS12
	case string(TLSVersion13):
		v = tls.VersionTLS13
	default:
		return 0, fmt.Errorf("unexpected TLS version %q (must be one of: TLS12, TLS13)", version)
	}

	return v, nil
}
