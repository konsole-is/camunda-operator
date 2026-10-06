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

package camundacluster

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// LiveZeebeWorkload reads the Zeebe StatefulSet of cluster through reader. Its
// pod template is what Zeebe runs, or rolls to. A workload that does not
// exist yet returns a failure with reason Progressing. Any other read error
// comes back as an error.
func LiveZeebeWorkload(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (*appsv1.StatefulSet, *conditions.PreCheckFailure, error) {
	var workload appsv1.StatefulSet
	key := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      WorkloadName(cluster, ComponentZeebe),
	}
	if err := reader.Get(ctx, key, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonProgressing,
				Message: fmt.Sprintf("the Zeebe workload %s is not rendered yet", key),
			}, nil
		}

		return nil, nil, fmt.Errorf("reading the Zeebe workload %s: %w", key, err)
	}

	return &workload, nil, nil
}

// RunningConfigHash returns the config hash that the pod template of
// workload carries. Mutable referents, for example a SecondaryStorageConfig,
// change this hash without a change of the cluster generation. A template
// without a hash returns a failure with reason Progressing.
func RunningConfigHash(workload *appsv1.StatefulSet) (string, *conditions.PreCheckFailure) {
	hash := workload.Spec.Template.Annotations[ConfigHashAnnotation]
	if hash == "" {
		return "", &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"the Zeebe workload %s/%s carries no config hash yet", workload.Namespace, workload.Name,
			),
		}
	}

	return hash, nil
}

// RunningVersion returns the Camunda version that workload carries in
// BrokerVersionAnnotation. The config hash does not cover the version, so a
// version change can keep the hash. A workload without the annotation
// returns a failure with reason Progressing.
func RunningVersion(workload *appsv1.StatefulSet) (string, *conditions.PreCheckFailure) {
	version := workload.Annotations[BrokerVersionAnnotation]
	if version == "" {
		return "", &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"the Zeebe workload %s/%s carries no Camunda version yet", workload.Namespace, workload.Name,
			),
		}
	}

	return version, nil
}

// RunsPublishedVersion returns nil when workload carries the Camunda version
// that the management binding of cluster publishes. A workload without a
// version, a cluster without a binding, and a binding that names another
// version return a failure with reason Progressing.
func RunsPublishedVersion(workload *appsv1.StatefulSet, cluster *v1.CamundaCluster) *conditions.PreCheckFailure {
	running, failure := RunningVersion(workload)
	if failure != nil {
		return failure
	}

	var published string
	if binding := cluster.Status.Management; binding != nil {
		published = binding.Version
	}
	if running == published {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonProgressing,
		Message: fmt.Sprintf(
			"Zeebe of CamundaCluster %s/%s runs Camunda %s, but the cluster publishes %q",
			cluster.Namespace, cluster.Name, running, published,
		),
	}
}

// RolledOut returns nil when every replica of workload runs its current pod
// template and is ready. A workload that still rolls returns a failure with reason
// Progressing. The pod template changes before the first pod restarts, so
// only this check shows that the pods run what the template says.
func RolledOut(workload *appsv1.StatefulSet) *conditions.PreCheckFailure {
	replicas := int32(1)
	if workload.Spec.Replicas != nil {
		replicas = *workload.Spec.Replicas
	}

	status := workload.Status
	if status.ObservedGeneration == workload.Generation && status.UpdatedReplicas == replicas &&
		status.ReadyReplicas == replicas && status.CurrentRevision == status.UpdateRevision {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonProgressing,
		Message: fmt.Sprintf(
			"the Zeebe workload %s/%s has not rolled out its current pod template yet (generation %d, "+
				"observed %d, %d of %d replicas updated, %d ready)",
			workload.Namespace, workload.Name, workload.Generation, status.ObservedGeneration,
			status.UpdatedReplicas, replicas, status.ReadyReplicas,
		),
	}
}

// BackupStoreEnv returns the environment of the backup store that the Zeebe
// pod template of cluster carries when bucket is its backupStorageRef.
func BackupStoreEnv(cluster *v1.CamundaCluster, bucket *v1.ObjectStorageConfig) []corev1.EnvVar {
	return backupStoreEnv(Input{Cluster: cluster, Backup: bucket}).env
}

// RunsBackupStore returns nil when template carries the backup store that
// bucket declares for cluster. An edit of the ObjectStorageConfig moves the
// store without a change of the cluster generation. A template that carries
// another store returns a failure with reason Progressing. Credentials are
// not compared.
func RunsBackupStore(
	template *corev1.PodTemplateSpec,
	cluster *v1.CamundaCluster,
	bucket *v1.ObjectStorageConfig,
) *conditions.PreCheckFailure {
	declared := map[string]string{}
	for _, env := range BackupStoreEnv(cluster, bucket) {
		if env.ValueFrom == nil {
			declared[env.Name] = env.Value
		}
	}

	// A key that the declared store does not render must be gone from the
	// template too. A stale endpoint alone sends the brokers elsewhere.
	for _, key := range backupDestinationKeys {
		want, wanted := declared[key.Env()]
		running, present := TemplateEnvValue(template, key.Env())
		if wanted == present && running == want {
			continue
		}

		return &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"Zeebe of CamundaCluster %s/%s does not run the backup store of ObjectStorageConfig %s yet: "+
					"%s is %s, not %s",
				cluster.Namespace, cluster.Name, bucket.Name, key.Env(),
				envState(running, present), envState(want, wanted),
			),
		}
	}

	return nil
}

// backupDestinationKeys are the keys of the backup store that decide where
// the brokers write. Credentials and client settings are not among them.
var backupDestinationKeys = []camundaconfig.Key{
	camundaconfig.KeyPrimaryBackupStore,
	camundaconfig.KeyPrimaryBackupS3BucketName,
	camundaconfig.KeyPrimaryBackupS3BasePath,
	camundaconfig.KeyPrimaryBackupS3Region,
	camundaconfig.KeyPrimaryBackupS3Endpoint,
	camundaconfig.KeyPrimaryBackupS3ForcePathStyleAccess,
	camundaconfig.KeyPrimaryBackupGCSBucketName,
	camundaconfig.KeyPrimaryBackupGCSBasePath,
	camundaconfig.KeyPrimaryBackupAzureEndpoint,
	camundaconfig.KeyPrimaryBackupAzureAccountName,
	camundaconfig.KeyPrimaryBackupAzureBasePath,
}

// envState renders an environment value for a message, or "absent".
func envState(value string, set bool) string {
	if !set {
		return "absent"
	}

	return fmt.Sprintf("%q", value)
}

// TemplateEnvValue returns the plain value of the environment variable name
// on any container of template, and whether one carries it as a plain value.
// A variable that takes its value from a reference does not count.
func TemplateEnvValue(template *corev1.PodTemplateSpec, name string) (string, bool) {
	for i := range template.Spec.Containers {
		for _, env := range template.Spec.Containers[i].Env {
			if env.Name == name && env.ValueFrom == nil {
				return env.Value, true
			}
		}
	}

	return "", false
}
