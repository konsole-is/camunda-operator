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
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/selectors"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The names and paths of the JVM trust store that the processes run with when
// their Elasticsearch carries a private certificate authority.
const (
	// InitContainerTrustStore is the init container that builds the trust
	// store. It runs the image of the process, which already ships keytool.
	// The restore Job keeps this container when it copies a broker pod,
	// because the restore JVM runs with the same trust store options.
	InitContainerTrustStore = "es-truststore"
	// TrustStoreMountPath is where the trust store volume is mounted. The
	// init container writes it, and the process container reads it.
	TrustStoreMountPath = "/etc/camunda/es-truststore"
	// TrustStorePath is the trust store file that the JVM of every process
	// reads.
	TrustStorePath = TrustStoreMountPath + "/cacerts"
	// TrustStorePassword is the password of the trust store. It is the
	// password of the JDK cacerts file that the init container copies, and
	// the Camunda Helm chart keeps it too. The store holds public
	// certificates, so the password guards the integrity of the file, and
	// the process container mounts the file read only.
	TrustStorePassword = "changeit"
	// trustStoreVolumeName is the emptyDir that carries the trust store from
	// the init container to the process container.
	trustStoreVolumeName = "es-truststore"
)

// The environment of the init container. The script reads these names, so it
// stays one constant for every cluster.
const (
	envTrustStoreSource   = "TRUST_STORE_SOURCE"
	envTrustStoreFile     = "TRUST_STORE_FILE"
	envTrustStorePassword = "TRUST_STORE_PASSWORD"
)

// trustStoreOptions are the JVM options that point the process at the trust
// store. render appends them to JAVA_TOOL_OPTIONS.
const trustStoreOptions = "-Djavax.net.ssl.trustStore=" + TrustStorePath +
	" -Djavax.net.ssl.trustStorePassword=" + TrustStorePassword

// trustStoreScript copies the cacerts file of the JDK and imports every
// certificate of the mounted PEM bundle into the copy.
//
// It copies the JDK file instead of creating an empty store, so the process
// keeps its trust in every public certificate authority. It reads the bundle
// line by line and imports each certificate under an alias of its own,
// because keytool takes one certificate per alias and drops the rest of a
// bundle without a message.
//
// It fails when the bundle holds no certificate. A process that starts with a
// store that trusts nothing new reports itself healthy and exports nothing.
// That is the failure this trust store removes.
const trustStoreScript = `set -eu
umask 022
home="${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v keytool)")")")}"
cp -L "$home/lib/security/cacerts" "$TRUST_STORE_FILE"
chmod 0644 "$TRUST_STORE_FILE"
single="$(dirname "$TRUST_STORE_FILE")/certificate.pem"
count=0
block=""
while IFS= read -r line || [ -n "$line" ]; do
	block="$block$line
"
	case "$line" in
	*"END CERTIFICATE"*)
		count=$((count + 1))
		printf '%s' "$block" > "$single"
		block=""
		"$home/bin/keytool" -importcert -noprompt -trustcacerts \
			-keystore "$TRUST_STORE_FILE" -storepass "$TRUST_STORE_PASSWORD" \
			-alias "camunda-es-ca-$count" -file "$single"
		;;
	esac
done < "$TRUST_STORE_SOURCE"
rm -f "$single"
if [ "$count" -eq 0 ]; then
	echo "no certificate in $TRUST_STORE_SOURCE" >&2
	exit 1
fi
`

// usesTrustStore reports whether a process runs with the JVM trust store: its
// secondary storage is an Elasticsearch that names a certificate authority.
//
// The gate is the caSecretRef of the binding, not the kind of resource that
// published it. An ElasticsearchCluster always names one, and a hand-written
// contract that points at any private-CA endpoint gets the same trust store.
//
// Connectors are left out. They talk to the gateway alone, they never read
// the secondary storage, and render gives them no CA mount to read.
func usesTrustStore(in Input, p Process) bool {
	if p.Component == ComponentConnectors {
		return false
	}

	es := in.Storage.Elasticsearch
	return in.Storage.Type == v1.SecondaryStorageTypeElasticsearch && es != nil && es.CASecretRef != nil
}

// trustStoreMutation adds the trust store to a workload: the emptyDir that
// carries it, the init container that builds it from the mounted CA, and the
// read-only mount of it on the process container.
//
// The JVM options that point at the store come from render, so they travel
// with the rest of the environment and the config hash rolls the pods when
// the trust store appears or goes away.
func trustStoreMutation(in Input, p Process) workloadMutation {
	return workloadMutation{
		Name:    MutationTrustStore,
		Feature: feature.NewBooleanGate(usesTrustStore(in, p)),
		Mutate: func(m primitives.WorkloadMutator) error {
			m.EditPodSpec(func(spec *editors.PodSpecEditor) error {
				spec.EnsureVolume(corev1.Volume{
					Name:         trustStoreVolumeName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				})
				return nil
			})

			m.EnsureInitContainer(trustStoreInitContainer(in, p))

			m.EditContainers(selectors.ContainerNamed(containerName(p)), func(c *editors.ContainerEditor) error {
				c.Raw().VolumeMounts = append(c.Raw().VolumeMounts, trustStoreMount(true))
				return nil
			})

			return nil
		},
	}
}

// trustStoreInitContainer builds the init container that writes the trust
// store. It runs the image of the process, which ships the JDK that the
// process runs on, so the store is built with the keytool of that JDK and no
// second image is pulled.
func trustStoreInitContainer(in Input, p Process) corev1.Container {
	ca := in.Storage.Elasticsearch.CASecretRef

	return corev1.Container{
		Name:    InitContainerTrustStore,
		Image:   Image(in, p),
		Command: []string{"sh", "-c", trustStoreScript},
		Env: []corev1.EnvVar{
			{Name: envTrustStoreSource, Value: CAMountPath + "/" + ca.Key},
			{Name: envTrustStoreFile, Value: TrustStorePath},
			{Name: envTrustStorePassword, Value: TrustStorePassword},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: caVolumeName, MountPath: CAMountPath, ReadOnly: true},
			trustStoreMount(false),
		},
	}
}

// trustStoreMount returns the mount of the trust store volume. The init
// container writes it, and every other container reads it.
func trustStoreMount(readOnly bool) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      trustStoreVolumeName,
		MountPath: TrustStoreMountPath,
		ReadOnly:  readOnly,
	}
}
