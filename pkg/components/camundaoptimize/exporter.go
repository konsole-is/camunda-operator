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

package camundaoptimize

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// ExporterFieldManager is the field manager of the exporter patch. The patch
// is applied with it directly, not through an ocf component: a component
// derives its field manager from the owner kind and the component name, and it
// would then own every field of the CamundaCluster that the patch object
// carries. With its own manager and a map list of entries, this controller
// owns only the entries it applies, and a user or a GitOps tool keeps theirs.
const ExporterFieldManager = "camunda-operator/camundaoptimize"

// ExporterClassName is the exporter that writes the records Optimize reads
// (ElasticsearchExporter.java of the camunda/camunda repository). From 8.8 on,
// the Camunda Exporter writes the indices of the orchestration cluster, and
// this exporter writes the zeebe-record indices for Optimize only. It is
// packaged with the binary, so the patch names no jar path.
const ExporterClassName = "io.camunda.zeebe.exporter.ElasticsearchExporter"

// ExporterEnv returns the environment entries that turn the legacy Zeebe
// Elasticsearch exporter on for the given secondary storage. The keys are the
// unified configuration of Camunda 8.9. The legacy zeebe.broker.exporters.*
// properties describe the same exporter, and the binary refuses to start when
// both are set, so the operator never writes them.
//
// The entries run in the broker container, so storage must carry the
// credentials reference as the cluster resolves it: the Secret of the cluster
// namespace, which is the copy that the cluster's own controller makes when
// the contract names another namespace. Passing the copy that this controller
// makes for the Optimize pods names a Secret the broker cannot read.
//
// The set carries no TLS setting, because the exporter has none. An
// Elasticsearch with a private CA therefore needs that CA in the JVM trust
// store of the broker before the exporter can reach it
// (camunda/camunda#9839). pkg/components/camundacluster builds that trust
// store for every binding that names a certificate authority. This exporter
// then reaches such an endpoint without a setting of its own.
func ExporterEnv(storage v1.ElasticsearchStorage) []corev1.EnvVar {
	creds := storage.CredentialsSecretRef

	return []corev1.EnvVar{
		camundaconfig.Var(camundaconfig.KeyExporterElasticsearchClassName, ExporterClassName),
		camundaconfig.Var(camundaconfig.KeyExporterElasticsearchURL, storage.Endpoint),
		camundaconfig.Var(camundaconfig.KeyExporterElasticsearchIndexPrefix, ZeebeRecordPrefix),
		camundaconfig.VarFrom(
			camundaconfig.KeyExporterElasticsearchUsername,
			secretSource(creds.Name, creds.UsernameKey),
		),
		camundaconfig.VarFrom(
			camundaconfig.KeyExporterElasticsearchPassword,
			secretSource(creds.Name, creds.PasswordKey),
		),
	}
}

// ExporterConflicts returns the names that desired and existing both carry
// while they supply the value the other way: one holds a literal and the other
// a reference. Server-side apply merges the two field by field inside the
// entry and reports no conflict, so the stored entry ends up with both a value
// and a valueFrom. A container rejects such an entry, and the rollout of the
// cluster stalls while the CamundaCluster still looks healthy. The schema of
// the CamundaCluster refuses the combination too; this function reports it
// before the apply, so the user reads a condition instead of an admission
// error.
//
// A name that both sides supply the same way is not a conflict: the apply
// takes ownership of that entry with forced ownership, which is the documented
// behavior.
func ExporterConflicts(desired, existing []corev1.EnvVar) []string {
	byName := make(map[string]corev1.EnvVar, len(existing))
	for _, e := range existing {
		byName[e.Name] = e
	}

	var conflicts []string
	for _, want := range desired {
		found, ok := byName[want.Name]
		if !ok {
			continue
		}
		if (want.ValueFrom != nil) != (found.ValueFrom != nil) {
			conflicts = append(conflicts, want.Name)
		}
	}

	return conflicts
}

// ExporterPatch returns the apply object of the exporter patch: a
// CamundaCluster that carries its own identity and nothing but
// spec.zeebe.extraEnv. Applied with ExporterFieldManager, it adds the given
// entries to that list and leaves every other field of the cluster to whoever
// owns it. An empty env removes the entries this manager owns, which is what
// the finalizer applies.
func ExporterPatch(
	cluster types.NamespacedName,
	uid types.UID,
	env []corev1.EnvVar,
) *v1.CamundaCluster {
	return &v1.CamundaCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			UID:       uid,
		},
		Spec: v1.CamundaClusterSpec{
			Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{ExtraEnv: env}},
		},
	}
}
