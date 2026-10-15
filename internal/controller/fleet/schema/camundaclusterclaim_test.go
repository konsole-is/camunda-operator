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

package schema

import (
	"fmt"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// clusterOverrideMessage is the message of the rule on spec.cluster.
const clusterOverrideMessage = "spec.cluster must not set presetRef, releaseRef, platformConfigRef, suspend, " +
	"version, connectors.version, serviceAccount.name, connectors.serviceAccount.name, storageRef, or backupStorageRef"

// clusterNameMessage is the message of the rule on metadata.name.
const clusterNameMessage = "metadata.name must be a DNS-1035 label of at most 40 characters"

// clusterSectionsMessage is the message of the rule on spec.plugins.
const clusterSectionsMessage = "sections and their registrations are fixed after creation"

// validClusterClaim returns a cluster claim with a unique name that sets every
// field that a rule reads.
func validClusterClaim() *fleetv1alpha1.CamundaClusterClaim {
	return &fleetv1alpha1.CamundaClusterClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orders-" + utilrand.String(8),
			Namespace: metav1.NamespaceDefault,
		},
		Spec: fleetv1alpha1.CamundaClusterClaimSpec{
			PresetRef:         "small",
			ReleaseRef:        "camunda-8-9",
			PlatformConfigRef: "production",
			Cluster: &v1.CamundaClusterSpec{
				Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{Replicas: ptr.To[int32](3)}},
			},
			Plugins: []fleetv1alpha1.PluginSelection{
				clusterClaimSelection("backup-storage", "s3-backups"),
				clusterClaimSelection("secondary-storage", "elasticsearch"),
			},
			Attachments: &fleetv1alpha1.ClusterAttachments{
				Optimize: &fleetv1alpha1.OptimizeAttachment{
					ManagementAuthConfigRef: "platform",
					ExternalURL:             "https://orders.example.com/optimize",
					Importer:                &v1.WorkloadSpec{Replicas: ptr.To[int32](1)},
				},
				BackupSchedule: &fleetv1alpha1.BackupScheduleAttachment{Schedule: "0 2 * * *"},
			},
			DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan,
		},
	}
}

func clusterClaimSelection(section, registration string) fleetv1alpha1.PluginSelection {
	return fleetv1alpha1.PluginSelection{Section: section, Registration: registration}
}

var _ = Describe("CamundaClusterClaim schema", func() {
	DescribeTable(
		"admission of the fields",
		func(mutate func(*fleetv1alpha1.CamundaClusterClaim), wantErr string) {
			obj := validClusterClaim()
			mutate(obj)

			expectAdmission(obj, wantErr)
		},
		Entry("admits a claim that sets every rule", func(*fleetv1alpha1.CamundaClusterClaim) {}, ""),
		// No required field of the CamundaCluster kind comes along with the
		// embedded spec type.
		Entry(
			"admits a claim whose cluster sets only zeebe.replicas", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec = fleetv1alpha1.CamundaClusterClaimSpec{
					PresetRef: "small",
					Cluster: &v1.CamundaClusterSpec{
						Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{Replicas: ptr.To[int32](3)}},
					},
				}
			}, "",
		),

		Entry(
			"refuses an empty presetRef", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.PresetRef = ""
			}, "spec.presetRef in body should be at least 1 chars long",
		),
		Entry(
			"refuses a presetRef that is not a DNS name", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.PresetRef = "Small"
			}, "spec.presetRef in body should match",
		),
		Entry(
			"refuses a releaseRef that is not a DNS name", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.ReleaseRef = "camunda_8_9"
			}, "spec.releaseRef in body should match",
		),
		Entry(
			"refuses a platformConfigRef that is not a DNS name", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.PlatformConfigRef = "-production"
			}, "spec.platformConfigRef in body should match",
		),

		Entry(
			"admits a full version in allowVersionDowngrade", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.AllowVersionDowngrade = "8.9.3"
			}, "",
		),
		Entry(
			"refuses an allowVersionDowngrade that is not a full version", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.AllowVersionDowngrade = "8.9"
			}, "spec.allowVersionDowngrade in body should match",
		),
		Entry(
			"refuses a deletion policy outside Delete and Orphan", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.DeletionPolicy = "Retain"
			}, "spec.deletionPolicy: Unsupported value",
		),

		Entry(
			"admits 32 selections", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Plugins = clusterClaimSelections(32)
			}, "",
		),
		Entry(
			"refuses 33 selections", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Plugins = clusterClaimSelections(33)
			}, "spec.plugins: Too many",
		),
		Entry(
			"refuses one selection twice", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Plugins = append(c.Spec.Plugins, c.Spec.Plugins[0])
			}, "Duplicate value",
		),

		Entry(
			"refuses an empty managementAuthConfigRef", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ManagementAuthConfigRef = ""
			}, "spec.attachments.optimize.managementAuthConfigRef in body should be at least 1 chars long",
		),
		Entry(
			"refuses an Optimize URL with another scheme", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ExternalURL = "ftp://orders.example.com"
			}, "externalUrl must be a valid http or https URL",
		),
		Entry(
			"refuses an Optimize URL with a comma", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ExternalURL = "https://orders.example.com/a,b"
			}, "externalUrl must carry no comma",
		),
		Entry(
			"refuses an Optimize URL with a percent-encoded character", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ExternalURL = "https://orders.example.com/a%20b"
			}, "externalUrl must carry no percent-encoded character",
		),
		Entry(
			"refuses an Optimize URL with a backslash", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ExternalURL = `https://orders.example.com/a\b`
			}, "externalUrl must carry no backslash",
		),
		Entry(
			"refuses an Optimize URL with a dot segment", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ExternalURL = "https://orders.example.com/a/../optimize"
			}, "externalUrl must carry no . or .. path segment",
		),
		Entry(
			"refuses an Optimize URL with a trailing slash", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ExternalURL = "https://orders.example.com/optimize/"
			}, "externalUrl must not end with a slash",
		),
		Entry(
			"refuses an Optimize URL with a query", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ExternalURL = "https://orders.example.com/optimize?a=b"
			}, "externalUrl must carry no query and no fragment",
		),
		Entry(
			"refuses an Optimize URL with whitespace", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.ExternalURL = "https://orders.example.com/opti mize"
			}, "externalUrl must carry no whitespace",
		),
		Entry(
			"refuses two Optimize importers", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.Importer.Replicas = ptr.To[int32](2)
			}, "importer.replicas must be 0 or 1",
		),
		Entry(
			"refuses a negative Optimize indexReplicas", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.Optimize.IndexReplicas = ptr.To[int32](-1)
			}, "spec.attachments.optimize.indexReplicas",
		),

		Entry(
			"refuses a backup schedule that is not a cron expression", func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Attachments.BackupSchedule.Schedule = "every night"
			}, "spec.attachments.backupSchedule.schedule in body should match",
		),
	)

	DescribeTable(
		"admission of a claim without a required field, or with an explicit zero value",
		func(value any, wantErr string, path ...string) {
			obj := toUnstructured(validClusterClaim(), "CamundaClusterClaim")
			if value == nil {
				unstructured.RemoveNestedField(obj.Object, path...)
			} else {
				Expect(unstructured.SetNestedField(obj.Object, value, path...)).To(Succeed())
			}

			expectAdmission(obj, wantErr)
		},
		Entry(
			"refuses a claim without presetRef",
			nil, "spec.presetRef: Required value", "spec", "presetRef",
		),
		Entry(
			"refuses an Optimize attachment without managementAuthConfigRef",
			nil, "spec.attachments.optimize.managementAuthConfigRef: Required value",
			"spec", "attachments", "optimize", "managementAuthConfigRef",
		),
		Entry(
			"refuses a backup schedule without schedule",
			nil, "spec.attachments.backupSchedule.schedule: Required value",
			"spec", "attachments", "backupSchedule", "schedule",
		),
		Entry(
			"admits an explicit empty allowVersionDowngrade",
			"", "", "spec", "allowVersionDowngrade",
		),
	)

	// Empty means the deletion policy of the environment, so the API server
	// must not fill in a value.
	It("keeps an unset deletion policy unset", func() {
		obj := validClusterClaim()
		obj.Spec.DeletionPolicy = ""
		expectAdmission(obj, "")

		var got fleetv1alpha1.CamundaClusterClaim
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &got)).To(Succeed())
		Expect(got.Spec.DeletionPolicy).To(BeEmpty())
	})

	DescribeTable(
		"admission of the name",
		func(name, wantErr string) {
			obj := validClusterClaim()
			obj.Name = name

			expectAdmission(obj, wantErr)
		},
		Entry("admits a name of 40 characters", "o"+strings.Repeat("a", 39), ""),
		Entry("refuses a name of 41 characters", "o"+strings.Repeat("a", 40), clusterNameMessage),
		Entry("refuses a name that starts with a digit", "1orders", clusterNameMessage),
		Entry("refuses a name with a dot", "orders.eu", clusterNameMessage),
	)

	DescribeTable(
		"admission of the cluster overrides",
		func(mutate func(*v1.CamundaClusterSpec), wantErr string) {
			obj := validClusterClaim()
			mutate(obj.Spec.Cluster)

			expectAdmission(obj, wantErr)
		},
		Entry("refuses presetRef", func(s *v1.CamundaClusterSpec) { s.PresetRef = "large" }, clusterOverrideMessage),
		Entry(
			"refuses releaseRef",
			func(s *v1.CamundaClusterSpec) { s.ReleaseRef = "camunda-8-8" }, clusterOverrideMessage,
		),
		Entry(
			"refuses platformConfigRef",
			func(s *v1.CamundaClusterSpec) { s.PlatformConfigRef = "staging" }, clusterOverrideMessage,
		),
		Entry("refuses suspend", func(s *v1.CamundaClusterSpec) { s.Suspend = true }, clusterOverrideMessage),
		Entry("refuses version", func(s *v1.CamundaClusterSpec) { s.Version = "8.9.3" }, clusterOverrideMessage),
		Entry(
			"refuses connectors.version",
			func(s *v1.CamundaClusterSpec) { s.Connectors = &v1.ConnectorsSpec{Version: "8.9.1"} },
			clusterOverrideMessage,
		),
		Entry(
			"refuses serviceAccount.name",
			func(s *v1.CamundaClusterSpec) { s.ServiceAccount = &v1.ServiceAccountSpec{Name: "orders"} },
			clusterOverrideMessage,
		),
		Entry(
			"refuses connectors.serviceAccount.name",
			func(s *v1.CamundaClusterSpec) {
				s.Connectors = &v1.ConnectorsSpec{ServiceAccount: &v1.ServiceAccountSpec{Name: "orders-c"}}
			},
			clusterOverrideMessage,
		),
		Entry(
			"refuses storageRef",
			func(s *v1.CamundaClusterSpec) { s.StorageRef = "orders-es" },
			clusterOverrideMessage,
		),
		Entry(
			"refuses backupStorageRef",
			func(s *v1.CamundaClusterSpec) { s.BackupStorageRef = "orders-backups" }, clusterOverrideMessage,
		),
		// Contract 3.2 lets a claim set these fields. A slot conflict is the
		// claim controller's refusal, not the API server's.
		Entry(
			"admits externalUrl",
			func(s *v1.CamundaClusterSpec) { s.ExternalURL = "https://orders.example.com" }, "",
		),
		Entry(
			"admits serviceAccount.annotations",
			func(s *v1.CamundaClusterSpec) {
				s.ServiceAccount = &v1.ServiceAccountSpec{Annotations: map[string]string{"team": "orders"}}
			}, "",
		),
		Entry(
			"admits connectors.serviceAccount.annotations",
			func(s *v1.CamundaClusterSpec) {
				s.Connectors = &v1.ConnectorsSpec{
					ServiceAccount: &v1.ServiceAccountSpec{Annotations: map[string]string{"team": "orders"}},
				}
			}, "",
		),
		Entry(
			"admits documentStorageRef",
			func(s *v1.CamundaClusterSpec) { s.DocumentStorageRef = "orders-documents" }, "",
		),
		Entry(
			"admits monitoring",
			func(s *v1.CamundaClusterSpec) {
				s.Monitoring = &v1.ClusterMonitoringSpec{ServiceMonitor: &v1.ServiceMonitorSpec{Enabled: true}}
			}, "",
		),
		Entry("admits pause", func(s *v1.CamundaClusterSpec) { s.Pause = true }, ""),
	)

	// Templated YAML often renders an unset field as its zero value.
	DescribeTable(
		"admission of an explicit zero value in the cluster overrides",
		func(value any, field string) {
			obj := toUnstructured(validClusterClaim(), "CamundaClusterClaim")
			Expect(unstructured.SetNestedField(obj.Object, value, "spec", "cluster", field)).To(Succeed())

			expectAdmission(obj, "")
		},
		Entry("admits suspend: false", false, "suspend"),
		Entry("admits an empty releaseRef", "", "releaseRef"),
		Entry("admits an empty presetRef", "", "presetRef"),
		Entry("admits an empty storageRef", "", "storageRef"),
		Entry("admits an empty backupStorageRef", "", "backupStorageRef"),
	)

	DescribeTable(
		"update of the selections",
		func(create, change func(*fleetv1alpha1.CamundaClusterClaim), wantErr string) {
			obj := validClusterClaim()
			create(obj)
			expectAdmission(obj, "")

			change(obj)

			expectUpdate(obj, wantErr)
		},
		Entry(
			"admits a change of only the parameters",
			func(*fleetv1alpha1.CamundaClusterClaim) {},
			func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Plugins[1].Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"preset":"large"}`)}
			},
			"",
		),
		Entry(
			"admits the same selections in another order",
			func(*fleetv1alpha1.CamundaClusterClaim) {},
			func(c *fleetv1alpha1.CamundaClusterClaim) { slices.Reverse(c.Spec.Plugins) },
			"",
		),
		Entry(
			"admits a change of another field",
			func(*fleetv1alpha1.CamundaClusterClaim) {},
			func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = true },
			"",
		),
		Entry(
			"admits an update of a claim that has no selection and keeps none",
			func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Plugins = nil },
			func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = true },
			"",
		),
		Entry(
			"refuses a change of a registration",
			func(*fleetv1alpha1.CamundaClusterClaim) {},
			func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Plugins[0].Registration = "gcs-backups" },
			clusterSectionsMessage,
		),
		Entry(
			"refuses an added selection",
			func(*fleetv1alpha1.CamundaClusterClaim) {},
			func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Plugins = append(c.Spec.Plugins, clusterClaimSelection("access", "gateway-access"))
			},
			clusterSectionsMessage,
		),
		Entry(
			"refuses a removed selection",
			func(*fleetv1alpha1.CamundaClusterClaim) {},
			func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Plugins = c.Spec.Plugins[:1] },
			clusterSectionsMessage,
		),
		Entry(
			"refuses the removal of every selection",
			func(*fleetv1alpha1.CamundaClusterClaim) {},
			func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Plugins = nil },
			clusterSectionsMessage,
		),
		Entry(
			"refuses a first selection on a claim created without one",
			func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Plugins = nil },
			func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.Plugins = []fleetv1alpha1.PluginSelection{clusterClaimSelection("backup-storage", "s3-backups")}
			},
			clusterSectionsMessage,
		),
	)
})

// clusterClaimSelections returns n selections with distinct sections.
func clusterClaimSelections(n int) []fleetv1alpha1.PluginSelection {
	selections := make([]fleetv1alpha1.PluginSelection, n)
	for i := range selections {
		selections[i] = clusterClaimSelection(fmt.Sprintf("section-%d", i), "registration")
	}

	return selections
}
