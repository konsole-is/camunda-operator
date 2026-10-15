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

package pluginstartup_test

import (
	"encoding/json"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

// The expected values below are copies of the contract, not calls to
// Sections or Registrations, so that the specs check the code against it.

type wantSection struct {
	label, emptyOption string
	slot               pluginsv1alpha1.Slot
	appliesTo          []pluginsv1alpha1.ClaimKind
	outputs            []string
}

type wantRegistration struct {
	sections         []string
	mode             pluginsv1alpha1.Mode
	required         []string
	properties       []string
	claimEditable    []string
	inputs           map[string]string // name -> from
	outputs          []string
	deletionPolicies []pluginsv1alpha1.DeletionPolicy
}

var (
	clusterClaim = []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim}
	planeClaim   = []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim}
	bothPolicies = []pluginsv1alpha1.DeletionPolicy{
		pluginsv1alpha1.DeletionPolicyDelete,
		pluginsv1alpha1.DeletionPolicyOrphan,
	}
	storageIn = map[string]string{
		"storageClassName":          "volume-encryption/storageClassName",
		"objectStorageConfig":       "backup-storage/objectStorageConfig",
		"serviceAccountAnnotations": "cloud-identity/serviceAccountAnnotations",
	}
	databaseOut = []string{"keycloakDatabaseConfig", "identityDatabaseConfig", "webModelerDatabaseConfig"}
)

var _ = Describe("Writer", Ordered, func() {
	It("writes exactly the eight core sections and the six built-in registrations", func() {
		Eventually(sectionNames, envtest.Timeout, envtest.Interval).Should(ConsistOf(
			"cloud-identity", "encryption-key", "volume-encryption", "backup-storage",
			"secondary-storage", "access", "connectors-permissions", "management-database",
		))
		Eventually(registrationNames, envtest.Timeout, envtest.Interval).Should(ConsistOf(
			"elasticsearch", "database-server", "management-database-server",
			"existing-secondary-storage", "existing-object-storage", "existing-database-configs",
		))
	})

	DescribeTable(
		"writes each core section as the contract gives it",
		func(name string, want wantSection) {
			var s pluginsv1alpha1.PluginSection
			Expect(env.Client.Get(env.Ctx, client.ObjectKey{Name: name}, &s)).To(Succeed())

			Expect(s.Spec.Label).To(Equal(want.label))
			Expect(s.Spec.Selection).To(Equal(pluginsv1alpha1.SelectionOne))
			Expect(s.Spec.EmptyOptionLabel).To(Equal(want.emptyOption))
			Expect(s.Spec.Slot).To(Equal(want.slot))
			Expect(s.Spec.AppliesTo).To(ConsistOf(want.appliesTo))
			Expect(keys(s.Spec.Outputs)).To(ConsistOf(want.outputs))
			Expect(s.ManagedFields).To(ContainElement(HaveField("Manager", pluginstartup.FieldManager)))
		},
		Entry("cloud-identity", "cloud-identity", wantSection{
			"Cloud identity", "No cloud identity",
			pluginsv1alpha1.SlotWorkloadIdentity, clusterClaim, []string{"principal", "serviceAccountAnnotations"},
		}),
		Entry("encryption-key", "encryption-key", wantSection{
			"Encryption key", "No key", "", clusterClaim, []string{"keyRef", "format"},
		}),
		Entry("volume-encryption", "volume-encryption", wantSection{
			"Volume encryption", "Unencrypted volumes",
			pluginsv1alpha1.SlotVolumeEncryption, clusterClaim, []string{"storageClassName"},
		}),
		Entry("backup-storage", "backup-storage", wantSection{
			"Backup storage", "No backups",
			pluginsv1alpha1.SlotBackupStorage, clusterClaim, []string{"objectStorageConfig"},
		}),
		Entry("secondary-storage", "secondary-storage", wantSection{
			"Secondary storage", "",
			pluginsv1alpha1.SlotSecondaryStorage, clusterClaim, []string{"secondaryStorageConfig"},
		}),
		Entry("access", "access", wantSection{
			"Access", "No access from outside",
			pluginsv1alpha1.SlotExternalAccess, append(slices.Clone(clusterClaim), planeClaim...),
			[]string{"externalUrl", "grpcHost", "optimizeUrl", "managementHost"},
		}),
		Entry("connectors-permissions", "connectors-permissions", wantSection{
			"Connectors permissions", "No cloud permissions",
			pluginsv1alpha1.SlotConnectorsIdentity, clusterClaim, []string{"principal", "serviceAccountAnnotations"},
		}),
		Entry("management-database", "management-database", wantSection{
			"Databases", "", pluginsv1alpha1.SlotManagementDatabases, planeClaim, databaseOut,
		}),
	)

	DescribeTable(
		"writes each built-in registration as the contract gives it",
		func(name string, want wantRegistration) {
			var r pluginsv1alpha1.PluginRegistration
			Expect(env.Client.Get(env.Ctx, client.ObjectKey{Name: name}, &r)).To(Succeed())

			Expect(r.Spec.Sections).To(Equal(want.sections))
			Expect(r.Spec.Mode).To(Equal(want.mode))
			Expect(
				r.Spec.Controller,
			).To(Equal(pluginsv1alpha1.ControllerInfo{Name: "camunda-operator", Version: suiteVersion}))
			Expect(r.Spec.ServiceAccount).To(Equal(serviceAccount))
			Expect(r.Spec.ClaimEditable).To(ConsistOf(want.claimEditable))
			Expect(r.Spec.DeletionPolicies).To(ConsistOf(want.deletionPolicies))
			Expect(r.Spec.OperatorVersions).To(BeEmpty())
			Expect(keys(r.Spec.Outputs)).To(ConsistOf(want.outputs))

			inputs := map[string]string{}
			for _, in := range r.Spec.Inputs {
				Expect(in.Required).To(BeFalse(), in.Name)
				inputs[in.Name] = in.From
			}
			Expect(inputs).To(Equal(want.inputs))

			var schema struct {
				Type       string                     `json:"type"`
				Required   []string                   `json:"required"`
				Properties map[string]json.RawMessage `json:"properties"`
			}
			Expect(json.Unmarshal(r.Spec.Parameters.Raw, &schema)).To(Succeed())
			Expect(schema.Type).To(Equal("object"))
			Expect(schema.Required).To(ConsistOf(want.required))
			Expect(schema.Properties).To(HaveLen(len(want.properties)))
			for _, p := range want.properties {
				Expect(schema.Properties).To(HaveKey(p))
			}
		},
		Entry("elasticsearch", "elasticsearch", wantRegistration{
			sections: []string{"secondary-storage"}, mode: pluginsv1alpha1.ModeDefault,
			required: []string{"preset"}, properties: []string{"preset", "overrides"},
			claimEditable: []string{"overrides"}, inputs: storageIn,
			outputs: []string{"secondaryStorageConfig"}, deletionPolicies: bothPolicies,
		}),
		Entry("database-server", "database-server", wantRegistration{
			sections: []string{"secondary-storage"}, mode: pluginsv1alpha1.ModeOptional,
			required: []string{"preset"}, properties: []string{"preset", "overrides"},
			claimEditable: []string{"overrides"}, inputs: storageIn,
			outputs: []string{"secondaryStorageConfig"}, deletionPolicies: bothPolicies,
		}),
		Entry("existing-secondary-storage", "existing-secondary-storage", wantRegistration{
			sections: []string{"secondary-storage"}, mode: pluginsv1alpha1.ModeOptional,
			required: []string{"name"}, properties: []string{"name"}, inputs: map[string]string{},
			outputs: []string{"secondaryStorageConfig"}, deletionPolicies: bothPolicies,
		}),
		Entry("existing-object-storage", "existing-object-storage", wantRegistration{
			sections: []string{"backup-storage"}, mode: pluginsv1alpha1.ModeOptional,
			required: []string{"name"}, properties: []string{"name"}, inputs: map[string]string{},
			outputs: []string{"objectStorageConfig"}, deletionPolicies: bothPolicies,
		}),
		Entry("management-database-server", "management-database-server", wantRegistration{
			sections: []string{"management-database"}, mode: pluginsv1alpha1.ModeDefault,
			required: []string{"preset"}, properties: []string{"preset", "databases"},
			inputs:  map[string]string{"storageClassName": "volume-encryption/storageClassName"},
			outputs: databaseOut, deletionPolicies: bothPolicies,
		}),
		Entry("existing-database-configs", "existing-database-configs", wantRegistration{
			sections: []string{"management-database"}, mode: pluginsv1alpha1.ModeOptional,
			properties: []string{"keycloak", "identity", "webModeler"}, inputs: map[string]string{},
			outputs: databaseOut, deletionPolicies: bothPolicies,
		}),
	)

	It("updates controller.version on the next start and keeps a field that another manager wrote", func() {
		var r pluginsv1alpha1.PluginRegistration
		Expect(env.Client.Get(env.Ctx, client.ObjectKey{Name: "elasticsearch"}, &r)).To(Succeed())
		before := r.DeepCopy()
		r.Spec.DocsURL = "https://example.com/docs/elasticsearch"
		r.Labels = map[string]string{"team": "data"}
		Expect(
			env.Client.Patch(env.Ctx, &r, client.MergeFrom(before), client.FieldOwner("fleet-manager")),
		).To(Succeed())

		Expect(pluginstartup.New(env.Client, "0.4.0", serviceAccount).Start(env.Ctx)).To(Succeed())

		Expect(env.Client.Get(env.Ctx, client.ObjectKey{Name: "elasticsearch"}, &r)).To(Succeed())
		Expect(r.Spec.Controller.Version).To(Equal("0.4.0"))
		Expect(r.Spec.DocsURL).To(Equal("https://example.com/docs/elasticsearch"))
		Expect(r.Labels).To(HaveKeyWithValue("team", "data"))
	})

	It("sets a field that it writes back to its value on the next start", func() {
		var s pluginsv1alpha1.PluginSection
		Expect(env.Client.Get(env.Ctx, client.ObjectKey{Name: "access"}, &s)).To(Succeed())
		before := s.DeepCopy()
		s.Spec.Label = "Ingress"
		Expect(env.Client.Patch(env.Ctx, &s, client.MergeFrom(before), client.FieldOwner("kubectl-edit"))).To(Succeed())

		Expect(pluginstartup.New(env.Client, suiteVersion, serviceAccount).Start(env.Ctx)).To(Succeed())

		Expect(env.Client.Get(env.Ctx, client.ObjectKeyFromObject(&s), &s)).To(Succeed())
		Expect(s.Spec.Label).To(Equal("Access"))
	})

	It("writes a core section again on the next start after an admin deleted it", func() {
		access := &pluginsv1alpha1.PluginSection{ObjectMeta: metav1.ObjectMeta{Name: "access"}}
		Expect(env.Client.Delete(env.Ctx, access)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(env.Client.Get(env.Ctx, client.ObjectKeyFromObject(access), access))
		}, envtest.Timeout, envtest.Interval).Should(BeTrue())

		Expect(pluginstartup.New(env.Client, suiteVersion, serviceAccount).Start(env.Ctx)).To(Succeed())

		Expect(env.Client.Get(env.Ctx, client.ObjectKeyFromObject(access), access)).To(Succeed())
		Expect(access.Spec.Label).To(Equal("Access"))
	})

	It("leaves a section that a plugin chart wrote in place", func() {
		plugin := &pluginsv1alpha1.PluginSection{
			ObjectMeta: metav1.ObjectMeta{Name: "dns-zone"},
			Spec: pluginsv1alpha1.PluginSectionSpec{
				Label:     "DNS zone",
				Selection: pluginsv1alpha1.SelectionOne,
				AppliesTo: clusterClaim,
			},
		}
		Expect(env.Client.Create(env.Ctx, plugin)).To(Succeed())

		Expect(pluginstartup.New(env.Client, suiteVersion, serviceAccount).Start(env.Ctx)).To(Succeed())

		Expect(env.Client.Get(env.Ctx, client.ObjectKeyFromObject(plugin), plugin)).To(Succeed())
		Expect(plugin.Spec.Label).To(Equal("DNS zone"))
		Expect(plugin.ManagedFields).NotTo(ContainElement(HaveField("Manager", pluginstartup.FieldManager)))
	})
})

// sectionNames returns the names of the sections that the writer manages, so
// that a section of another spec does not change the result.
func sectionNames() []string {
	var list pluginsv1alpha1.PluginSectionList
	Expect(env.Client.List(env.Ctx, &list)).To(Succeed())
	names := []string{}
	for _, s := range list.Items {
		if slices.ContainsFunc(s.ManagedFields, func(f metav1.ManagedFieldsEntry) bool {
			return f.Manager == pluginstartup.FieldManager
		}) {
			names = append(names, s.Name)
		}
	}
	return names
}

func registrationNames() []string {
	var list pluginsv1alpha1.PluginRegistrationList
	Expect(env.Client.List(env.Ctx, &list)).To(Succeed())
	names := make([]string, 0, len(list.Items))
	for _, r := range list.Items {
		names = append(names, r.Name)
	}
	return names
}

func keys(outputs []pluginsv1alpha1.OutputDecl) []string {
	out := make([]string, 0, len(outputs))
	for _, o := range outputs {
		out = append(out, o.Key)
	}
	return out
}
