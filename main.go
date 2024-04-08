/*
Copyright 2019 The Kubernetes Authors.

DOT NOT MERGE

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
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/flags"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

// Constants for TLS versions.
const (
	TLSVersion12 = "TLS12"
	TLSVersion13 = "TLS13"
)

type TLSOptions struct {
	TLSMaxVersion   string
	TLSMinVersion   string
	TLSCipherSuites string
}

var (
	myscheme             = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
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
	diagnosticsOptions   = flags.DiagnosticsOptions{}
	logOptions           = logs.NewOptions()
	tlsOptions           = TLSOptions{}
	tlsSupportedVersions = []string{TLSVersion12, TLSVersion13}
)

func init() {
	_ = scheme.AddToScheme(myscheme)
	_ = ipamv1.AddToScheme(myscheme)
	_ = clusterv1.AddToScheme(myscheme)
}

// Add RBAC for the authorized diagnostics endpoint.
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

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

	diagnosticsOpts := flags.GetDiagnosticsOptions(diagnosticsOptions)

	var watchNamespaces map[string]cache.Config
	if watchNamespace != "" {
		watchNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	req, _ := labels.NewRequirement(clusterv1.ClusterNameLabel, selection.Exists, nil)
	clusterSecretCacheSelector := labels.NewSelector().Add(*req)

	restConfig := ctrl.GetConfigOrDie()
	restConfig.QPS = float32(restConfigQPS)
	restConfig.Burst = restConfigBurst
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                     myscheme,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "controller-leader-election-ipam-capm3",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		HealthProbeBindAddress:     healthAddr,
		Metrics:                    diagnosticsOpts,
		Cache: cache.Options{
			DefaultNamespaces: watchNamespaces,
			SyncPeriod:        &syncPeriod,
			ByObject: map[client.Object]cache.ByObject{
				// Note: Only Secrets with the cluster name label are cached.
				// The default client of the manager won't use the cache for secrets at all (see Client.Cache.DisableFor).
				// The cached secrets will only be used by the secretCachingClient we create below.
				&corev1.Secret{}: {
					Label: clusterSecretCacheSelector,
				},
			},
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
			},
		},
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Port:    webhookPort,
				CertDir: webhookCertDir,
				TLSOpts: tlsOptionOverrides,
			},
		),
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

func initFlags(fs *pflag.FlagSet) {
	logs.AddFlags(fs, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logOptions, fs)

	fs.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&watchNamespace, "namespace", "",
		"Namespace that the controller watches to reconcile IPAM objects. If unspecified, the controller watches for IPAM objects across all namespaces.")
	fs.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")
	fs.IntVar(&webhookPort, "webhook-port", 9443,
		"Webhook Server port")
	fs.StringVar(
		&webhookCertDir,
		"webhook-cert-dir",
		"/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.",
	)
	fs.StringVar(
		&watchFilterValue,
		"watch-filter",
		"",
		fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel),
	)
	fs.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")

	fs.IntVar(&ippoolConcurrency, "ippool-concurrency", 10,
		"Number of ippools to process simultaneously")

	fs.Float64Var(&restConfigQPS, "kube-api-qps", 20,
		"Maximum queries per second from the controller client to the Kubernetes API server. Default 20")

	fs.IntVar(&restConfigBurst, "kube-api-burst", 30,
		"Maximum number of queries that should be allowed in one burst from the controller client to the Kubernetes API server. Default 30")
	fs.StringVar(&tlsOptions.TLSMinVersion, "tls-min-version", TLSVersion12,
		"The minimum TLS version in use by the webhook server.\n"+
			fmt.Sprintf("Possible values are %s.", strings.Join(tlsSupportedVersions, ", ")),
	)

	fs.StringVar(&tlsOptions.TLSMaxVersion, "tls-max-version", TLSVersion13,
		"The maximum TLS version in use by the webhook server.\n"+
			fmt.Sprintf("Possible values are %s.", strings.Join(tlsSupportedVersions, ", ")),
	)

	tlsCipherPreferredValues := cliflag.PreferredTLSCipherNames()
	tlsCipherInsecureValues := cliflag.InsecureTLSCipherNames()
	fs.StringVar(&tlsOptions.TLSCipherSuites, "tls-cipher-suites", "",
		"Comma-separated list of cipher suites for the webhook server. "+
			"If omitted, the default Go cipher suites will be used. \n"+
			"Preferred values: "+strings.Join(tlsCipherPreferredValues, ", ")+". \n"+
			"Insecure values: "+strings.Join(tlsCipherInsecureValues, ", ")+".")

	flags.AddDiagnosticsOptions(fs, &diagnosticsOptions)
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
	if tlsMinVersion >= tls.VersionTLS13 &&
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
	case TLSVersion12:
		v = tls.VersionTLS12
	case TLSVersion13:
		v = tls.VersionTLS13
	default:
		return 0, fmt.Errorf("unexpected TLS version %q (must be one of: %s)", version, strings.Join(tlsSupportedVersions, ", "))
	}

	return v, nil
}
