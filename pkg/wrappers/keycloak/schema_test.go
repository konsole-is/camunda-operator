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

package keycloak_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
	"github.com/konsole-is/camunda-operator/test/utils"
)

// The Keycloak types are hand written against the CRD vendored in
// test/envtest/crds/keycloak, because the Keycloak project publishes no Go
// module for them. A field that the schema does not declare is pruned on
// write, without an error, and the operator would then run a Keycloak that
// silently ignores what the spec asked for. The subtests write every field
// the operator sets and read the object back.
func TestKeycloakTypesRoundTripThroughTheVendoredSchema(t *testing.T) {
	apiClient := startControlPlane(t)

	t.Run("round-trips every field the operator sets", func(t *testing.T) {
		ctx := t.Context()
		kc := &keycloak.Keycloak{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kc-" + utilrand.String(8),
				Namespace: "default",
			},
			Spec: keycloak.KeycloakSpec{
				Instances: new(int32(2)),
				Image:     "camunda/keycloak:quay-optimized-26.0.7",
				DB: &keycloak.KeycloakDBSpec{
					Vendor: "postgres",
					URL:    "jdbc:aws-wrapper:postgresql://postgres.my-cluster-ns.svc:5432/keycloak",
					Schema: "public",
					UsernameSecret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-db"},
						Key:                  "username",
					},
					PasswordSecret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-db"},
						Key:                  "password",
					},
				},
				HTTP: &keycloak.KeycloakHTTPSpec{
					HTTPEnabled: new(true),
					HTTPPort:    new(int32(8080)),
				},
				Hostname: &keycloak.KeycloakHostnameSpec{
					Hostname: "https://keycloak.example.com/auth",
					Strict:   new(false),
				},
				Ingress: &keycloak.KeycloakIngressSpec{Enabled: new(false)},
				Proxy:   &keycloak.KeycloakProxySpec{Headers: "xforwarded"},
				Scheduling: &keycloak.KeycloakSchedulingSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchExpressions: []corev1.NodeSelectorRequirement{{
										Key:      "pool",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"auth"},
									}},
								}},
							},
						},
					},
					Tolerations: []corev1.Toleration{{
						Key:      "dedicated",
						Operator: corev1.TolerationOpEqual,
						Value:    "auth",
						Effect:   corev1.TaintEffectNoSchedule,
					}},
				},
				AdditionalOptions: []keycloak.KeycloakValueOrSecret{
					{Name: "http-relative-path", Value: "/auth"},
					{Name: "log-level", Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-options"},
						Key:                  "log-level",
					}},
				},
				Unsupported: &keycloak.KeycloakUnsupportedSpec{
					PodTemplate: &corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"camunda.io/component": "keycloak"},
						},
					},
				},
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			},
		}
		want := kc.Spec.DeepCopy()

		require.NoError(t, apiClient.Create(ctx, kc))

		var stored keycloak.Keycloak
		require.NoError(t, apiClient.Get(ctx, client.ObjectKeyFromObject(kc), &stored))
		assert.Equal(t, want, &stored.Spec)
	})

	t.Run("reads the conditions of the status subresource", func(t *testing.T) {
		ctx := t.Context()
		kc := &keycloak.Keycloak{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kc-" + utilrand.String(8),
				Namespace: "default",
			},
			Spec: keycloak.KeycloakSpec{Instances: new(int32(1))},
		}
		require.NoError(t, apiClient.Create(ctx, kc))

		kc.Status = keycloak.KeycloakStatus{
			Instances: 1,
			Conditions: []keycloak.KeycloakCondition{{
				Type:    keycloak.ConditionReady,
				Status:  "True",
				Message: "Keycloak is ready",
			}},
		}
		require.NoError(t, apiClient.Status().Update(ctx, kc))

		var stored keycloak.Keycloak
		require.NoError(t, apiClient.Get(ctx, client.ObjectKeyFromObject(kc), &stored))
		assert.Equal(t, int32(1), stored.Status.Instances)
		require.Len(t, stored.Status.Conditions, 1)
		assert.Equal(t, keycloak.ConditionReady, stored.Status.Conditions[0].Type)
		assert.Equal(t, "True", stored.Status.Conditions[0].Status)
	})
}

// startControlPlane boots an envtest control plane that serves the vendored
// Keycloak CRD, and returns a client against it.
func startControlPlane(t *testing.T) client.Client {
	t.Helper()

	crdPath, err := utils.KeycloakCRDPath()
	require.NoError(t, err)

	control := &envtest.Environment{
		CRDDirectoryPaths:     []string{crdPath},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: utils.EnvtestBinaryDir(),
		// A full run boots several control planes at once on one machine.
		// test/envtest gives the suites it starts the same two budgets.
		ControlPlaneStartTimeout: 2 * time.Minute,
		CRDInstallOptions:        envtest.CRDInstallOptions{MaxTime: time.Minute},
	}

	cfg, err := control.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, control.Stop())
	})

	// The scheme is private to this test. Registering into the global
	// scheme.Scheme would leak the kind into every other test that shares it.
	testScheme := runtime.NewScheme()
	require.NoError(t, keycloak.AddToScheme(testScheme))

	apiClient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	return apiClient
}
