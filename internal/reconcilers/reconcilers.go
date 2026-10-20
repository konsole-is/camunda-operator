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

// Package reconcilers registers every reconciler of the operator and the
// startup writer of the plugin objects with a manager. cmd/main.go and
// hack/fleetscenarios both call Setup, so the page scenarios of the fleet
// manager run the reconcilers that the manager runs. A new controller
// registers here, never in cmd/main.go.
package reconcilers

import (
	"errors"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
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
	"github.com/konsole-is/camunda-operator/internal/controller/fleet/environment"
	"github.com/konsole-is/camunda-operator/internal/controller/logicalbackupelasticsearch"
	"github.com/konsole-is/camunda-operator/internal/controller/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/internal/controller/logicalrestoreelasticsearch"
	"github.com/konsole-is/camunda-operator/internal/controller/logicalrestorerdbms"
	"github.com/konsole-is/camunda-operator/internal/controller/managementauthconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/objectstorageconfig"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtinregistration"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/pluginregistration"
	"github.com/konsole-is/camunda-operator/internal/controller/pointintimerestore"
	"github.com/konsole-is/camunda-operator/internal/controller/secondarystorageconfig"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/grace"
)

// Options holds the settings of the manager that the reconcilers read.
type Options struct {
	// Namespace is the namespace of the operator. It holds the Leases that
	// serialize the cross-namespace claims.
	Namespace string
	// CLIImage is the camunda-operator-cli image that the logical backup and
	// restore Jobs of a PostgreSQL database run.
	CLIImage string
	// GracePeriods are how long a workload or a datastore may take to become
	// ready before its condition reports Degraded or Down.
	GracePeriods grace.Periods
	// OperatorVersion is the version of the operator build, in strict semver.
	// The built-in PluginRegistrations carry it as the version of their
	// controller, and the PluginRegistration controller checks it against
	// spec.operatorVersions.
	OperatorVersion string
	// ServiceAccount is the ServiceAccount of the operator. The built-in
	// PluginRegistrations name it.
	ServiceAccount pluginsv1alpha1.ServiceAccountRef
}

// Setup registers every reconciler of the operator with mgr. It stops at the
// first reconciler that cannot register, and its error names the kind of
// that reconciler. It also adds the Writer of internal/pluginstartup, which
// writes the core PluginSections and the built-in PluginRegistrations at
// each start. Setup refuses an opts.ServiceAccount with an empty namespace
// or name.
func Setup(mgr ctrl.Manager, opts Options) error {
	if opts.ServiceAccount.Namespace == "" || opts.ServiceAccount.Name == "" {
		return errors.New("setting up the plugin startup writer: the ServiceAccount of the operator is required")
	}

	c, reader, scheme := mgr.GetClient(), mgr.GetAPIReader(), mgr.GetScheme()

	registrations := []struct {
		kind  string
		setup func() error
	}{
		{"CamundaCluster", func() error {
			return (&camundacluster.CamundaClusterReconciler{
				Client:         c,
				APIReader:      reader,
				Scheme:         scheme,
				ClaimNamespace: opts.Namespace,
				GracePeriods:   opts.GracePeriods,
			}).SetupWithManager(mgr)
		}},
		{"CamundaPlatformConfig", func() error {
			return (&camundaplatformconfig.CamundaPlatformConfigReconciler{
				Client:    c,
				APIReader: reader,
				Scheme:    scheme,
			}).SetupWithManager(mgr)
		}},
		{"ElasticsearchCluster", func() error {
			return (&elasticsearchcluster.ElasticsearchClusterReconciler{
				Client:       c,
				APIReader:    reader,
				Scheme:       scheme,
				GracePeriods: opts.GracePeriods,
			}).SetupWithManager(mgr)
		}},
		{"LogicalBackupRDBMS", func() error {
			return (&logicalbackuprdbms.LogicalBackupRDBMSReconciler{
				Client:    c,
				APIReader: reader,
				Scheme:    scheme,
			}).SetupWithManager(mgr, logicalbackuprdbms.Options{CLIImage: opts.CLIImage})
		}},
		{"Database", func() error {
			return (&database.DatabaseReconciler{
				Client:         c,
				APIReader:      reader,
				Scheme:         scheme,
				ClaimNamespace: opts.Namespace,
			}).SetupWithManager(mgr)
		}},
		{"DatabaseServerConfig", func() error {
			return (&databaseserverconfig.DatabaseServerConfigReconciler{
				Client:    c,
				APIReader: reader,
				Scheme:    scheme,
			}).SetupWithManager(mgr)
		}},
		{"DatabaseServer", func() error {
			return (&databaseserver.DatabaseServerReconciler{
				Client:       c,
				APIReader:    reader,
				Scheme:       scheme,
				GracePeriods: opts.GracePeriods,
			}).SetupWithManager(mgr)
		}},
		{"DatabaseConfig", func() error {
			return (&databaseconfig.DatabaseConfigReconciler{
				Client:    c,
				APIReader: reader,
				Scheme:    scheme,
			}).SetupWithManager(mgr)
		}},
		{"SecondaryStorageConfig", func() error {
			return (&secondarystorageconfig.SecondaryStorageConfigReconciler{
				Client:    c,
				APIReader: reader,
				Scheme:    scheme,
			}).SetupWithManager(mgr)
		}},
		{"ObjectStorageConfig", func() error {
			return (&objectstorageconfig.ObjectStorageConfigReconciler{
				Client:    c,
				APIReader: reader,
				Scheme:    scheme,
			}).SetupWithManager(mgr)
		}},
		{"LogicalBackupElasticsearch", func() error {
			return logicalbackupelasticsearch.New(
				c, reader, scheme, logicalbackupelasticsearch.Options{},
			).SetupWithManager(mgr)
		}},
		{"PointInTimeRestore", func() error {
			return pointintimerestore.New(
				c, reader, scheme, opts.Namespace, pointintimerestore.Options{},
			).SetupWithManager(mgr)
		}},
		{"BackupSchedule", func() error {
			return (&backupschedule.BackupScheduleReconciler{
				Client:    c,
				APIReader: reader,
				Scheme:    scheme,
			}).SetupWithManager(mgr, backupschedule.Options{})
		}},
		{"CamundaOptimize", func() error {
			return (&camundaoptimize.Reconciler{
				Client:         c,
				APIReader:      reader,
				Scheme:         scheme,
				ClaimNamespace: opts.Namespace,
				GracePeriods:   opts.GracePeriods,
			}).SetupWithManager(mgr)
		}},
		{"CamundaManagementCluster", func() error {
			r := camundamanagementcluster.New(c, reader, scheme)
			r.ClaimNamespace = opts.Namespace
			r.GracePeriods = opts.GracePeriods
			return r.SetupWithManager(mgr)
		}},
		{"ManagementAuthConfig", func() error {
			return (&managementauthconfig.ManagementAuthConfigReconciler{
				Client:    c,
				APIReader: reader,
				Scheme:    scheme,
			}).SetupWithManager(mgr)
		}},
		{"LogicalRestoreElasticsearch", func() error {
			return logicalrestoreelasticsearch.New(
				c, reader, scheme, logicalrestoreelasticsearch.Options{ClaimNamespace: opts.Namespace},
			).SetupWithManager(mgr)
		}},
		{"LogicalRestoreRDBMS", func() error {
			return logicalrestorerdbms.New(
				c, reader, scheme, logicalrestorerdbms.Options{CLIImage: opts.CLIImage, ClaimNamespace: opts.Namespace},
			).SetupWithManager(mgr)
		}},
		{"PluginRegistration", func() error {
			return (&pluginregistration.Reconciler{
				Client:          c,
				Scheme:          scheme,
				OperatorVersion: opts.OperatorVersion,
			}).SetupWithManager(mgr)
		}},
		{"PluginRequest", func() error {
			return (&builtin.Reconciler{
				Client:        c,
				APIReader:     reader,
				Scheme:        scheme,
				Registrations: pluginstartup.Registrations(opts.OperatorVersion, opts.ServiceAccount),
			}).SetupWithManager(mgr)
		}},
		{"built-in PluginRegistration", func() error {
			return (&builtinregistration.Reconciler{
				Client: c,
				Names:  builtin.Served(),
			}).SetupWithManager(mgr)
		}},
		{"Environment", func() error {
			return (&environment.Reconciler{
				Client: c,
				Scheme: scheme,
			}).SetupWithManager(mgr)
		}},
	}

	for _, r := range registrations {
		if err := r.setup(); err != nil {
			return fmt.Errorf("setting up the %s controller: %w", r.kind, err)
		}
	}

	if err := mgr.Add(pluginstartup.New(c, opts.OperatorVersion, opts.ServiceAccount)); err != nil {
		return fmt.Errorf("setting up the plugin startup writer: %w", err)
	}

	return nil
}
