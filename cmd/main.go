/*
Copyright 2026.

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
	"crypto/tls"
	"flag"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/konsole-is/camunda-operator/internal/controller/backupschedule"
	"github.com/konsole-is/camunda-operator/internal/controller/camundacluster"
	"github.com/konsole-is/camunda-operator/internal/controller/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/internal/controller/camundaoptimize"
	"github.com/konsole-is/camunda-operator/internal/controller/camundaplatformconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/database"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseserver"
	"github.com/konsole-is/camunda-operator/internal/controller/databaseserverconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/internal/controller/logicalbackupelasticsearch"
	"github.com/konsole-is/camunda-operator/internal/controller/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/internal/controller/logicalrestoreelasticsearch"
	"github.com/konsole-is/camunda-operator/internal/controller/logicalrestorerdbms"
	"github.com/konsole-is/camunda-operator/internal/controller/managementauthconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/objectstorageconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/pointintimerestore"
	"github.com/konsole-is/camunda-operator/internal/controller/secondarystorageconfig"
	"github.com/konsole-is/camunda-operator/internal/manager"
	"github.com/konsole-is/camunda-operator/pkg/grace"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(manager.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// cliImageEnv is the environment variable that defaults
// --camunda-operator-cli-image. The packaging sets it to the published image.
const cliImageEnv = "CAMUNDA_OPERATOR_CLI_IMAGE"

// namespaceEnv is the environment variable that defaults --namespace.
const namespaceEnv = "CAMUNDA_OPERATOR_NAMESPACE"

// serviceAccountNamespaceFile holds the namespace of the Pod that the manager
// runs in. Kubernetes mounts it with the service account token, so a manager
// in a cluster needs neither the flag nor the environment variable.
const serviceAccountNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// settings holds the values of the manager flags. Its fields are set when
// the FlagSet that bindFlags registered them on is parsed.
type settings struct {
	cliImage             string
	namespace            string
	metricsAddr          string
	metricsCertPath      string
	metricsCertName      string
	metricsCertKey       string
	webhookCertPath      string
	webhookCertName      string
	webhookCertKey       string
	enableLeaderElection bool
	probeAddr            string
	secureMetrics        bool
	enableHTTP2          bool
	gracePeriods         grace.Periods
	// checkGracePeriods returns the error of a grace period that the manager
	// must not start with. Call it after the parse.
	checkGracePeriods func() error
	zapOptions        zap.Options
}

// nolint:gocyclo
func main() {
	s := bindFlags(flag.CommandLine, os.Getenv)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&s.zapOptions)))

	if err := s.checkGracePeriods(); err != nil {
		setupLog.Error(err, "Failed to read the grace periods")
		os.Exit(1)
	}

	// The LogicalBackupRDBMS and LogicalRestoreRDBMS controllers render Jobs
	// that run the CLI image. Without one they can only guess, so the manager
	// refuses to start.
	if s.cliImage == "" {
		setupLog.Error(
			nil,
			"The camunda-operator-cli image is required: set --camunda-operator-cli-image "+
				"or the "+cliImageEnv+" environment variable to the published image, "+
				"for example ghcr.io/konsole-is/camunda-operator-cli:<version>",
		)
		os.Exit(1)
	}

	if s.namespace == "" {
		s.namespace = podNamespace()
	}
	// The Database and CamundaManagementCluster controllers serialize the
	// claim of a logical database and of a Keycloak realm through Leases of
	// this namespace. Without one they cannot tell two claimants apart, so
	// the manager refuses to start.
	if s.namespace == "" {
		setupLog.Error(
			nil,
			"The namespace of the operator is required: set --namespace or the "+
				namespaceEnv+" environment variable",
		)
		os.Exit(1)
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	var tlsOpts []func(*tls.Config)
	if !s.enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(s.webhookCertPath) > 0 {
		setupLog.Info(
			"Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path",
			s.webhookCertPath,
			"webhook-cert-name",
			s.webhookCertName,
			"webhook-cert-key",
			s.webhookCertKey,
		)

		webhookServerOptions.CertDir = s.webhookCertPath
		webhookServerOptions.CertName = s.webhookCertName
		webhookServerOptions.KeyName = s.webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   s.metricsAddr,
		SecureServing: s.secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if s.secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(s.metricsCertPath) > 0 {
		setupLog.Info(
			"Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path",
			s.metricsCertPath,
			"metrics-cert-name",
			s.metricsCertName,
			"metrics-cert-key",
			s.metricsCertKey,
		)

		metricsServerOptions.CertDir = s.metricsCertPath
		metricsServerOptions.CertName = s.metricsCertName
		metricsServerOptions.KeyName = s.metricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Cache:                  manager.CacheOptions(),
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: s.probeAddr,
		LeaderElection:         s.enableLeaderElection,
		LeaderElectionID:       "3d8c383c.camunda.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	if err := (&camundacluster.CamundaClusterReconciler{
		Client:         mgr.GetClient(),
		APIReader:      mgr.GetAPIReader(),
		Scheme:         mgr.GetScheme(),
		ClaimNamespace: s.namespace,
		GracePeriods:   s.gracePeriods,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "CamundaCluster")
		os.Exit(1)
	}
	if err := (&camundaplatformconfig.CamundaPlatformConfigReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "CamundaPlatformConfig")
		os.Exit(1)
	}
	if err := (&elasticsearchcluster.ElasticsearchClusterReconciler{
		Client:       mgr.GetClient(),
		APIReader:    mgr.GetAPIReader(),
		Scheme:       mgr.GetScheme(),
		GracePeriods: s.gracePeriods,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "ElasticsearchCluster")
		os.Exit(1)
	}
	if err := (&logicalbackuprdbms.LogicalBackupRDBMSReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr, logicalbackuprdbms.Options{CLIImage: s.cliImage}); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "LogicalBackupRDBMS")
		os.Exit(1)
	}
	if err := (&database.DatabaseReconciler{
		Client:         mgr.GetClient(),
		APIReader:      mgr.GetAPIReader(),
		Scheme:         mgr.GetScheme(),
		ClaimNamespace: s.namespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "Database")
		os.Exit(1)
	}
	if err := (&databaseserverconfig.DatabaseServerConfigReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "DatabaseServerConfig")
		os.Exit(1)
	}
	if err := (&databaseserver.DatabaseServerReconciler{
		Client:       mgr.GetClient(),
		APIReader:    mgr.GetAPIReader(),
		Scheme:       mgr.GetScheme(),
		GracePeriods: s.gracePeriods,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "DatabaseServer")
		os.Exit(1)
	}
	if err := (&databaseconfig.DatabaseConfigReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "DatabaseConfig")
		os.Exit(1)
	}
	if err := (&secondarystorageconfig.SecondaryStorageConfigReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "SecondaryStorageConfig")
		os.Exit(1)
	}
	if err := (&objectstorageconfig.ObjectStorageConfigReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "ObjectStorageConfig")
		os.Exit(1)
	}
	if err := logicalbackupelasticsearch.New(
		mgr.GetClient(), mgr.GetAPIReader(), mgr.GetScheme(), logicalbackupelasticsearch.Options{},
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "LogicalBackupElasticsearch")
		os.Exit(1)
	}
	pitrReconciler := pointintimerestore.New(
		mgr.GetClient(), mgr.GetAPIReader(), mgr.GetScheme(), s.namespace,
		pointintimerestore.Options{},
	)
	if err := pitrReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "PointInTimeRestore")
		os.Exit(1)
	}
	if err := (&backupschedule.BackupScheduleReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr, backupschedule.Options{}); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "BackupSchedule")
		os.Exit(1)
	}
	if err := (&camundaoptimize.Reconciler{
		Client:         mgr.GetClient(),
		APIReader:      mgr.GetAPIReader(),
		Scheme:         mgr.GetScheme(),
		ClaimNamespace: s.namespace,
		GracePeriods:   s.gracePeriods,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "CamundaOptimize")
		os.Exit(1)
	}
	managementCluster := camundamanagementcluster.New(
		mgr.GetClient(), mgr.GetAPIReader(), mgr.GetScheme(),
	)
	managementCluster.ClaimNamespace = s.namespace
	managementCluster.GracePeriods = s.gracePeriods
	if err := managementCluster.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "CamundaManagementCluster")
		os.Exit(1)
	}
	if err := (&managementauthconfig.ManagementAuthConfigReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "ManagementAuthConfig")
		os.Exit(1)
	}
	if err := logicalrestoreelasticsearch.New(
		mgr.GetClient(),
		mgr.GetAPIReader(),
		mgr.GetScheme(),
		logicalrestoreelasticsearch.Options{ClaimNamespace: s.namespace},
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "LogicalRestoreElasticsearch")
		os.Exit(1)
	}
	if err := logicalrestorerdbms.New(
		mgr.GetClient(),
		mgr.GetAPIReader(),
		mgr.GetScheme(),
		logicalrestorerdbms.Options{CLIImage: s.cliImage, ClaimNamespace: s.namespace},
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "LogicalRestoreRDBMS")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}

// bindFlags registers on fs every flag that the manager defines, and reads
// through getenv the environment variables that default some of them. The
// table "Manager settings" of dist/chart/README.md lists these flags, and
// TestManagerSettingsTable fails when the two differ.
func bindFlags(fs *flag.FlagSet, getenv func(string) string) *settings {
	s := &settings{
		zapOptions: zap.Options{
			Development: true,
		},
	}

	fs.StringVar(&s.metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	fs.StringVar(&s.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVar(
		&s.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.",
	)
	fs.BoolVar(
		&s.secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.",
	)
	fs.StringVar(&s.webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	fs.StringVar(&s.webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	fs.StringVar(&s.webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	fs.StringVar(
		&s.metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.",
	)
	fs.StringVar(&s.metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	fs.StringVar(&s.metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	fs.BoolVar(
		&s.enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers",
	)
	fs.StringVar(
		&s.cliImage, "camunda-operator-cli-image", getenv(cliImageEnv),
		"The camunda-operator-cli image that the logical backup and restore Jobs "+
			"of a PostgreSQL database run. "+
			"Required. Defaults to the "+cliImageEnv+" environment variable.",
	)
	fs.StringVar(
		&s.namespace, "namespace", getenv(namespaceEnv),
		"The namespace that the operator runs in. It holds the Leases that serialize the "+
			"cross-namespace claims: of a logical database, of a Keycloak realm, and of the "+
			"secondary storage backend of a cluster. Defaults to the "+namespaceEnv+
			" environment variable, and then to the namespace of the Pod.",
	)
	s.checkGracePeriods = s.gracePeriods.BindFlags(fs, getenv)
	s.zapOptions.BindFlags(fs)

	return s
}

// podNamespace reads the namespace of the Pod that the manager runs in. It
// returns the empty string when it cannot read the file. That covers a
// manager outside a cluster, where the file is absent, and a file that the
// manager may not read. The caller then asks for the flag or the environment
// variable.
func podNamespace() string {
	content, err := os.ReadFile(serviceAccountNamespaceFile)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}
