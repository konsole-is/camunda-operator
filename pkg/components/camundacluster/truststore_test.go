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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A cluster whose Elasticsearch names no certificate authority renders no
// trust store at all: no init container, no volume, and the JVM options of
// the base.
func TestTrustStoreIsAbsentWithoutACA(t *testing.T) {
	t.Parallel()

	in := fixtureMinimal(t)
	comps, err := Build(in)
	require.NoError(t, err)

	for _, pc := range comps {
		assert.False(t, usesTrustStore(in, pc.Process), pc.Process.Component)

		template := previewedPodTemplate(t, previewObjects(t, pc.Component))
		assert.Empty(t, template.Spec.InitContainers, pc.Process.Component)
		assert.Empty(t, template.Spec.Volumes, pc.Process.Component)
	}

	r := render(in, process(t, in, ComponentZeebe))
	assertEnv(t, r.env, "JAVA_TOOL_OPTIONS", javaToolOptions)
}

// An RDBMS cluster renders no trust store either: the gate is the certificate
// authority of an Elasticsearch binding.
func TestTrustStoreIsAbsentOnRDBMS(t *testing.T) {
	t.Parallel()

	in := fixtureRDBMS(t)
	for _, p := range Resolve(in.Effective) {
		assert.False(t, usesTrustStore(in, p), p.Component)
	}
}

// The gate is the caSecretRef of the binding, so a hand-written contract that
// points at any private-CA Elasticsearch gets the same trust store as one an
// ElasticsearchCluster publishes.
func TestTrustStoreFollowsTheCASecretRef(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Storage.Elasticsearch.Endpoint = "https://es.example.com:9200"
		in.Storage.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
			Name: "corporate-ca", Namespace: "my-cluster-ns", Key: "bundle.pem",
		}
	})

	assert.True(t, usesTrustStore(in, process(t, in, ComponentZeebe)))
	assert.False(t, usesTrustStore(in, process(t, in, ComponentConnectors)))

	init := trustStoreInitContainer(in, process(t, in, ComponentZeebe))
	assert.Equal(t, InitContainerTrustStore, init.Name)
	assert.Equal(t, Image(in, process(t, in, ComponentZeebe)), init.Image)
	assert.Contains(t, init.Env, corev1.EnvVar{Name: envTrustStoreSource, Value: CAMountPath + "/bundle.pem"})
	assert.Contains(t, init.Env, corev1.EnvVar{Name: envTrustStoreFile, Value: TrustStorePath})
	assert.Equal(
		t,
		[]corev1.VolumeMount{
			{Name: caVolumeName, MountPath: CAMountPath, ReadOnly: true},
			{Name: trustStoreVolumeName, MountPath: TrustStoreMountPath},
		},
		init.VolumeMounts,
	)
}

// The trust store options are appended to JAVA_TOOL_OPTIONS. The base flag
// stays, because a process without it leaves a broken JVM running after an
// OutOfMemoryError.
func TestTrustStoreOptionsKeepTheBaseOptions(t *testing.T) {
	t.Parallel()

	in := fixtureDefault(t)
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "JAVA_TOOL_OPTIONS", javaToolOptions+" "+trustStoreOptions)
}

// A user who overrides JAVA_TOOL_OPTIONS keeps the value of the override, and
// the trust store options still reach the JVM. Without this the pods carry a
// trust store that nothing reads, and the export fails again without a
// sign.
func TestTrustStoreOptionsSurviveAUserOverride(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Storage.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
			Name: "es-ca", Namespace: "my-cluster-ns", Key: "ca.crt",
		}
		in.Cluster.Spec.ExtraEnv = []corev1.EnvVar{{Name: "JAVA_TOOL_OPTIONS", Value: "-Xmx6g"}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})

	r := render(in, process(t, in, ComponentZeebe))
	assertEnv(t, r.env, "JAVA_TOOL_OPTIONS", "-Xmx6g "+trustStoreOptions)
}

// A user who names a trust store of their own keeps it, and the operator adds
// nothing. That user states an intent, not an accident. It is also the only
// way to trust a second private authority, for example an OIDC provider,
// because the spec carries no CA bundle field. The Elasticsearch CA must then
// be in that store.
func TestTrustStoreOptionsYieldToAUserTrustStore(t *testing.T) {
	t.Parallel()

	own := "-Djavax.net.ssl.trustStore=/my/own.jks -Djavax.net.ssl.trustStorePassword=hunter2"
	in := newInput(t, func(in *Input) {
		in.Storage.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
			Name: "es-ca", Namespace: "my-cluster-ns", Key: "ca.crt",
		}
		in.Cluster.Spec.ExtraEnv = []corev1.EnvVar{{Name: "JAVA_TOOL_OPTIONS", Value: own}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})

	r := render(in, process(t, in, ComponentZeebe))
	assertEnv(t, r.env, "JAVA_TOOL_OPTIONS", own)

	// The store is still built. The user owns which store the JVM reads, not
	// whether the CA is on disk.
	assert.True(t, usesTrustStore(in, process(t, in, ComponentZeebe)))
}

// A password without a file is not a trust store, so a user who sets the
// password alone still gets the options of the operator.
func TestTrustStoreOptionsIgnoreAPasswordOnItsOwn(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Storage.Elasticsearch.CASecretRef = &v1.SecretKeyRef{
			Name: "es-ca", Namespace: "my-cluster-ns", Key: "ca.crt",
		}
		in.Cluster.Spec.ExtraEnv = []corev1.EnvVar{
			{Name: "JAVA_TOOL_OPTIONS", Value: "-Djavax.net.ssl.trustStorePassword=hunter2"},
		}
		in.Effective = NewEffective(in.Cluster.Spec)
	})

	r := render(in, process(t, in, ComponentZeebe))
	assertEnv(t, r.env, "JAVA_TOOL_OPTIONS", "-Djavax.net.ssl.trustStorePassword=hunter2 "+trustStoreOptions)
}

// Connectors never read the secondary storage, so their environment carries
// no trust store options.
func TestTrustStoreOptionsAreAbsentOnConnectors(t *testing.T) {
	t.Parallel()

	in := fixtureDefault(t)
	r := render(in, process(t, in, ComponentConnectors))

	assertNoEnv(t, r.env, "JAVA_TOOL_OPTIONS")
}

// The script copies the cacerts file of the JDK, so the process keeps its
// trust in every public certificate authority, and it imports each
// certificate of a bundle under an alias of its own. keytool takes one
// certificate per alias, so a single alias drops every certificate after the
// first without a message.
func TestTrustStoreScriptImportsEveryCertificate(t *testing.T) {
	t.Parallel()

	bundle := "-----BEGIN CERTIFICATE-----\nFIRST\n-----END CERTIFICATE-----\n" +
		"-----BEGIN CERTIFICATE-----\nSECOND\n-----END CERTIFICATE-----\n"

	env := runTrustStoreScript(t, bundle)
	require.NoError(t, env.err, env.output)

	store, err := os.ReadFile(env.storeFile)
	require.NoError(t, err)
	assert.Equal(t, "jdk-cacerts", string(store))

	imported, err := os.ReadFile(env.log)
	require.NoError(t, err)
	assert.Equal(t, "camunda-es-ca-1 FIRST\ncamunda-es-ca-2 SECOND\n", string(imported))

	left, err := filepath.Glob(filepath.Dir(env.storeFile) + "/*.pem")
	require.NoError(t, err)
	assert.Empty(t, left, "the script left a certificate file behind")
}

// A bundle without a certificate fails the init container. A process that
// starts with a store that trusts nothing new reports itself healthy and
// exports nothing.
func TestTrustStoreScriptFailsOnAnEmptyBundle(t *testing.T) {
	t.Parallel()

	env := runTrustStoreScript(t, "# no certificate here\n")
	assert.Error(t, env.err, env.output)
	assert.Contains(t, env.output, "no certificate")
}

// trustStoreRun is the result of one run of trustStoreScript against a stub
// JDK.
type trustStoreRun struct {
	storeFile string
	log       string
	output    string
	err       error
}

// runTrustStoreScript runs trustStoreScript with sh against a stub JDK whose
// keytool records the alias and the payload of every import. It proves the
// behavior of the script without a Java runtime.
func runTrustStoreScript(t *testing.T, bundle string) trustStoreRun {
	t.Helper()

	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on this machine")
	}

	root := t.TempDir()
	home := filepath.Join(root, "jdk")
	store := filepath.Join(root, "truststore")
	log := filepath.Join(root, "imported.txt")
	source := filepath.Join(root, "ca.pem")

	require.NoError(t, os.MkdirAll(filepath.Join(home, "lib", "security"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(home, "bin"), 0o755))
	require.NoError(t, os.MkdirAll(store, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, "lib", "security", "cacerts"), []byte("jdk-cacerts"), 0o644))
	require.NoError(t, os.WriteFile(source, []byte(bundle), 0o644))

	// The stub records the alias and the payload line of the certificate it
	// was given, so the test reads which certificates were imported and under
	// which alias.
	stub := `#!/bin/sh
alias=""
file=""
while [ $# -gt 0 ]; do
	case "$1" in
	-alias) alias="$2"; shift 2 ;;
	-file) file="$2"; shift 2 ;;
	*) shift ;;
	esac
done
printf '%s %s\n' "$alias" "$(sed -n 2p "$file")" >> "$KEYTOOL_LOG"
`
	require.NoError(t, os.WriteFile(filepath.Join(home, "bin", "keytool"), []byte(stub), 0o755))

	storeFile := filepath.Join(store, "cacerts")
	cmd := exec.Command(shell, "-c", trustStoreScript)
	cmd.Env = append(
		os.Environ(),
		"JAVA_HOME="+home,
		envTrustStoreSource+"="+source,
		envTrustStoreFile+"="+storeFile,
		envTrustStorePassword+"="+TrustStorePassword,
		"KEYTOOL_LOG="+log,
	)
	output, err := cmd.CombinedOutput()

	return trustStoreRun{storeFile: storeFile, log: log, output: string(output), err: err}
}
