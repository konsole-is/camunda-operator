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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The fixture identities. Every value is fixed, so the golden manifests stay
// deterministic.
const (
	fixtureName      = "my-optimize"
	fixtureNamespace = "camunda"
	fixtureCluster   = "my-cluster"
	fixtureVersion   = "8.9.4"
)

// newInput returns the minimal render input, with mutate applied to it.
func newInput(t *testing.T, mutate func(in *Input)) Input {
	t.Helper()

	in := Input{
		Optimize: &v1.CamundaOptimize{
			ObjectMeta: metav1.ObjectMeta{Name: fixtureName, Namespace: fixtureNamespace},
			Spec: v1.CamundaOptimizeSpec{
				Version:           fixtureVersion,
				ManagementAuthRef: "management-auth",
				ClusterRef:        v1.ClusterRef{Name: fixtureCluster},
			},
		},
		ClusterName: fixtureCluster,
		Partitions:  1,
		Storage: v1.ElasticsearchStorage{
			Endpoint: "http://elasticsearch.camunda.svc:9200",
			CredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name:        "es-credentials",
				UsernameKey: "username",
				PasswordKey: "password",
			},
		},
		Auth: &v1.ManagementAuthConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "management-auth"},
			Spec: v1.ManagementAuthConfigSpec{
				BaseURL:   "http://identity.camunda.svc:8080",
				IssuerURL: "https://identity.example.com/realms/camunda",
				AuthURL:   "https://identity.example.com/realms/camunda/protocol/openid-connect/auth",
				TokenURL:  "https://identity.example.com/realms/camunda/protocol/openid-connect/token",
				JwksURL:   "https://identity.example.com/realms/camunda/protocol/openid-connect/certs",
				ClientID:  "optimize",
				Audience:  "optimize-api",
				ClientSecretRef: v1.SecretKeyRef{
					Name:      "optimize-client",
					Namespace: fixtureNamespace,
					Key:       "client-secret",
				},
			},
		},
	}
	if mutate != nil {
		mutate(&in)
	}

	return in
}

// fixtureMinimal is a CamundaOptimize with nothing but the required fields.
func fixtureMinimal(t *testing.T) Input {
	t.Helper()

	return newInput(t, nil)
}

// fixtureRealistic exercises every override surface: a TLS endpoint with a CA,
// a platform registry and license, several partitions, per-workload resources,
// scheduling, pod metadata, extra environment, and a ServiceMonitor.
func fixtureRealistic(t *testing.T) Input {
	t.Helper()

	return newInput(t, func(in *Input) {
		in.Partitions = 3
		in.ServiceMonitorSupported = true
		in.Platform = v1.CamundaPlatformConfigSpec{
			Images:           &v1.ImagesSpec{Optimize: "registry.example.com/mirror/camunda/optimize"},
			LicenseSecretRef: &v1.SecretKeyRef{Name: "camunda-license", Namespace: fixtureNamespace, Key: "license"},
		}
		in.Storage.Endpoint = "https://elasticsearch.camunda.svc:9200"
		in.Storage.CASecretRef = &v1.LocalSecretKeyRef{
			Name: "es-ca",
			Key:  "ca.crt",
		}
		in.Auth.Spec.IssuerBackendURL = "http://identity.camunda.svc:8080/realms/camunda"
		in.Optimize.Spec.Webapp = &v1.WorkloadSpec{
			Replicas: new(int32(2)),
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
			ExtraEnv: []corev1.EnvVar{{Name: "OPTIMIZE_JAVA_OPTS", Value: "-Xmx2048m"}},
			ExtraEnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "optimize-extra"},
			}}},
			PodLabels:      map[string]string{"team": "platform"},
			PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
		}
		in.Optimize.Spec.Importer = &v1.WorkloadSpec{
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			},
			Scheduling: &v1.SchedulingSpec{
				Tolerations: []corev1.Toleration{{Key: "camunda", Operator: corev1.TolerationOpExists}},
			},
		}
		in.Optimize.Spec.Monitoring = &v1.OptimizeMonitoringSpec{
			ServiceMonitor: &v1.ServiceMonitorSpec{
				Enabled: true,
				Labels:  map[string]string{"prometheus": "platform"},
			},
		}
	})
}

// goldenFixtures are the fixtures that the golden test renders, by directory
// name.
func goldenFixtures(t *testing.T) map[string]Input {
	t.Helper()

	return map[string]Input{
		"minimal":   fixtureMinimal(t),
		"realistic": fixtureRealistic(t),
	}
}
