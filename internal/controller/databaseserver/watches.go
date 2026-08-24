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

package databaseserver

import (
	"context"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// databaseServerPresetRefField indexes DatabaseServers by spec.presetRef, so a
// preset edit enqueues every server that references it.
const databaseServerPresetRefField = "databaseserver.spec.presetRef"

// watches registers the controller with every watch it needs.
func (r *DatabaseServerReconciler) watches(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.DatabaseServer{},
		databaseServerPresetRefField, func(o client.Object) []string {
			if ref := o.(*v1.DatabaseServer).Spec.PresetRef; ref != "" {
				return []string{ref}
			}
			return nil
		},
	); err != nil {
		return err
	}

	if err := refindex.EnsureObjectStorageConfigSecretIndex(mgr); err != nil {
		return err
	}

	controller := ctrl.NewControllerManagedBy(mgr).
		For(&v1.DatabaseServer{})

	if r.cnpgInstalled {
		controller = controller.
			Owns(&cnpgv1.Cluster{}).
			Owns(&cnpgv1.ScheduledBackup{}).
			// A base backup is what makes the archive recoverable, so its
			// completion is what turns ArchiveReady True. CloudNativePG owns
			// the Backup, so the event is mapped back by the cluster it names.
			Watches(&cnpgv1.Backup{}, r.enqueueForBaseBackup())
	} else {
		mgr.GetLogger().Info(
			"CloudNativePG Cluster CRD not found, every DatabaseServer reports CNPGNotInstalled " +
				"until CloudNativePG is installed and the operator restarts",
		)
	}

	if r.barmanInstalled {
		controller = controller.Owns(&barmanobjectstore.ObjectStore{})
	} else {
		mgr.GetLogger().Info(
			"Barman Cloud ObjectStore CRD not found, every DatabaseServer with an archive reports " +
				"BarmanPluginNotInstalled until the plugin is installed and the operator restarts",
		)
	}

	if r.podMonitorSupported() {
		controller = controller.Owns(&monitoringv1.PodMonitor{})
	}

	return controller.
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Owns(&v1.DatabaseServerConfig{}).
		Watches(
			&v1.DatabaseServerPreset{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.DatabaseServerList{},
				databaseServerPresetRefField, refindex.ObjectName,
			),
		).
		Watches(&v1.CamundaPlatformConfig{}, r.enqueueForPlatformConfig()).
		Watches(&v1.ObjectStorageConfig{}, r.enqueueForArchiveStorage()).
		Watches(&corev1.Secret{}, r.enqueueForBucketSecret(), builder.OnlyMetadata).
		Watches(&corev1.PersistentVolumeClaim{}, r.enqueueForDataClaim()).
		Named("databaseserver").
		Complete(r)
}

// enqueueForBaseBackup maps a base backup event to the server whose current
// cluster the backup names.
func (r *DatabaseServerReconciler) enqueueForBaseBackup() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		backup, ok := o.(*cnpgv1.Backup)
		if !ok {
			return nil
		}

		return r.serversMatching(ctx, backup.Namespace, func(server *v1.DatabaseServer) bool {
			return components.ClusterName(server) == backup.Spec.Cluster.Name
		})
	})
}

// enqueueForDataClaim maps a data volume claim event to the server whose
// current cluster CloudNativePG labelled it with, so a resize outside the spec
// updates status.volumes.
func (r *DatabaseServerReconciler) enqueueForDataClaim() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		cluster, ok := o.GetLabels()[components.CNPGClusterNameLabel]
		if !ok {
			return nil
		}

		return r.serversMatching(ctx, o.GetNamespace(), func(server *v1.DatabaseServer) bool {
			return components.ClusterName(server) == cluster
		})
	})
}

// enqueueForPlatformConfig maps a platform config event to every server whose
// effective reference names it, preset-provided references included.
func (r *DatabaseServerReconciler) enqueueForPlatformConfig() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		return r.serversMatching(ctx, "", func(server *v1.DatabaseServer) bool {
			return r.effectiveSpec(ctx, server).PlatformConfigRef == o.GetName()
		})
	})
}

// enqueueForArchiveStorage maps a bucket event to every server whose effective
// archive names it, preset-provided archives included.
func (r *DatabaseServerReconciler) enqueueForArchiveStorage() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		return r.serversMatching(ctx, "", func(server *v1.DatabaseServer) bool {
			return archiveRef(r.effectiveSpec(ctx, server)) == o.GetName()
		})
	})
}

// enqueueForBucketSecret maps a Secret event to every server whose effective
// bucket holds its static credentials in that Secret. The bucket's Secret
// carries no owner reference to any server, so without this watch a rotated
// credential reaches the archive only when something unrelated triggers a
// reconcile.
func (r *DatabaseServerReconciler) enqueueForBucketSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		key := refindex.ObjectNamespacedName(o)

		var buckets v1.ObjectStorageConfigList
		if err := r.List(
			ctx, &buckets,
			client.MatchingFields{refindex.ObjectStorageConfigSecretField: key},
		); err != nil {
			logf.FromContext(ctx).Error(err, "Could not list buckets for a Secret event", "secret", key)
			return nil
		}
		if len(buckets.Items) == 0 {
			return nil
		}

		names := make(map[string]bool, len(buckets.Items))
		for i := range buckets.Items {
			names[buckets.Items[i].Name] = true
		}

		return r.serversMatching(ctx, "", func(server *v1.DatabaseServer) bool {
			return names[archiveRef(r.effectiveSpec(ctx, server))]
		})
	})
}

// serversMatching returns a request for every server in namespace that match
// accepts. An empty namespace searches every namespace. The servers are listed
// in full, because an effective reference can come from a preset and a field
// index cannot resolve one; fleets are small enough for that.
func (r *DatabaseServerReconciler) serversMatching(
	ctx context.Context,
	namespace string,
	match func(*v1.DatabaseServer) bool,
) []reconcile.Request {
	var servers v1.DatabaseServerList
	if err := r.List(ctx, &servers, client.InNamespace(namespace)); err != nil {
		logf.FromContext(ctx).Error(err, "Could not list database servers for a referenced object event")
		return nil
	}

	var requests []reconcile.Request
	for i := range servers.Items {
		if match(&servers.Items[i]) {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&servers.Items[i]),
			})
		}
	}

	return requests
}

// effectiveSpec returns the preset-merged spec of server, for the enqueue
// mappings that must see a reference a preset provides. A preset that does not
// resolve yields the inline spec: the reconcile reports the dangling reference,
// and an enqueue is not the place to.
func (r *DatabaseServerReconciler) effectiveSpec(
	ctx context.Context,
	server *v1.DatabaseServer,
) v1.DatabaseServerSpec {
	if server.Spec.PresetRef == "" {
		return server.Spec
	}

	var preset v1.DatabaseServerPreset
	if err := r.Get(ctx, types.NamespacedName{Name: server.Spec.PresetRef}, &preset); err != nil {
		return server.Spec
	}

	return components.MergePreset(server.Spec, &preset.Spec)
}

// archiveRef returns the bucket the merged spec archives to, or the empty
// string when it archives nowhere.
func archiveRef(merged v1.DatabaseServerSpec) string {
	if merged.Archive == nil {
		return ""
	}

	return merged.Archive.ObjectStorageRef
}
