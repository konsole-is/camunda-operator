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
	"slices"
	"strings"

	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/selectors"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
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

// The JVM properties that name the trust store and its password. The equals
// sign is part of each name, so trustStoreFlag never matches
// trustStorePasswordFlag.
const (
	trustStoreFlag         = "-Djavax.net.ssl.trustStore="
	trustStorePasswordFlag = "-Djavax.net.ssl.trustStorePassword="
)

// trustStoreOptions are the JVM options that point the process at the trust
// store. appendTrustStoreOptions puts them on JAVA_TOOL_OPTIONS.
const trustStoreOptions = trustStoreFlag + TrustStorePath +
	" " + trustStorePasswordFlag + TrustStorePassword

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

// trustStoreMutation adds the trust store to a workload: the emptyDir that
// carries it, the init container that builds it from the mounted CA, and the
// read-only mount of it on the process container.
//
// Every step replaces what it finds, because the framework replays a mutation
// onto the object of the previous pass. EnsureVolume and EnsureInitContainer
// replace by name on their own. The mount needs ensureTrustStoreMount.
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
				ensureTrustStoreMount(c.Raw())
				return nil
			})

			return nil
		},
	}
}

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

// ensureTrustStoreMount puts the read-only trust store mount on the process
// container. It replaces the mount of the trust store volume when the
// container carries one already, and appends the mount when the container
// carries none. The volume name is the identity, so a mount of that volume at
// another path becomes the mount the operator owns.
//
// The framework replays a mutation onto the object of the previous pass, so a
// plain append doubles the entry. Server-side apply keys volumeMounts by
// mountPath, and a duplicate key fails the typed patch, so the whole apply
// never lands and the workload stops converging. editors.ContainerEditor
// carries no EnsureVolumeMount, so this rule lives here.
func ensureTrustStoreMount(container *corev1.Container) {
	mount := trustStoreMount(true)
	for i, existing := range container.VolumeMounts {
		if existing.Name == mount.Name {
			container.VolumeMounts[i] = mount
			return
		}
	}

	container.VolumeMounts = append(container.VolumeMounts, mount)
}

// appendTrustStoreOptions appends the trust store options to the
// JAVA_TOOL_OPTIONS entry of env, in place.
//
// It runs after the user layer, because JAVA_TOOL_OPTIONS is the one variable
// the JVM reads for its options. A user who tunes the heap replaces the value
// of the operator. The pods then carry the trust store and never read it, and
// the export fails again without a sign. That is the failure this removes.
//
// A value that already names a trust store file is left alone. That user
// states an intent, not an accident, and the operator honors it. The user
// then owns the trust of the JVM, and the Elasticsearch CA must be in that
// store. It is also the only way to trust a second private authority, for
// example an OIDC provider or a backup store, because the spec carries no CA
// bundle field.
//
// An entry that reads its value from a reference is left alone too, because a
// variable holds a value or a reference, never both. The controller warns
// about every process in that state, see ReferencedJavaToolOptions.
func appendTrustStoreOptions(env []corev1.EnvVar) {
	for i, e := range env {
		if e.Name != camundaconfig.EnvJavaToolOptions || e.ValueFrom != nil || namesATrustStore(e.Value) {
			continue
		}
		env[i].Value = strings.TrimSpace(e.Value + " " + trustStoreOptions)
	}
}

// namesATrustStore reports whether a JAVA_TOOL_OPTIONS value already names a
// trust store file. The match is the whole property name with its equals
// sign, so a value that carries the password alone does not count: a password
// without a file is not a trust store.
func namesATrustStore(options string) bool {
	for option := range strings.FieldsSeq(options) {
		if strings.HasPrefix(option, trustStoreFlag) {
			return true
		}
	}

	return false
}

// ReferencedJavaToolOptions returns the components of the processes that need
// the trust store but read JAVA_TOOL_OPTIONS from a reference, sorted. It
// returns nil when every process that needs the trust store gets its options.
//
// One variable holds a value or a reference, never both, so the operator
// cannot add its options to such an entry. The pods still build the store,
// and the JVM reads it only when the referenced value names it. The caller
// reports every component in this list, because a silent skip is the failure
// that this trust store removes.
func ReferencedJavaToolOptions(in Input) []string {
	var referenced []string
	for _, p := range Resolve(in.Effective) {
		if !p.Enabled || !usesTrustStore(in, p) {
			continue
		}

		if readsJavaToolOptionsFromReference(userEnv(in, p)) {
			referenced = append(referenced, p.Component)
		}
	}

	slices.Sort(referenced)
	return referenced
}

// readsJavaToolOptionsFromReference reports whether the last JAVA_TOOL_OPTIONS
// entry of env reads its value from a reference. The last entry decides,
// because dedupeEnv keeps the last entry of a name.
func readsJavaToolOptionsFromReference(env []corev1.EnvVar) bool {
	referenced := false
	for _, e := range env {
		if e.Name == camundaconfig.EnvJavaToolOptions {
			referenced = e.ValueFrom != nil
		}
	}

	return referenced
}
