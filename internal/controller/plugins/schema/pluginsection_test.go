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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// validPluginSection returns the core section backup-storage under a unique
// name.
func validPluginSection() *pluginsv1alpha1.PluginSection {
	return &pluginsv1alpha1.PluginSection{
		ObjectMeta: metav1.ObjectMeta{Name: "section-" + utilrand.String(8)},
		Spec: pluginsv1alpha1.PluginSectionSpec{
			Label:            "Backup storage",
			Order:            40,
			Selection:        pluginsv1alpha1.SelectionOne,
			EmptyOptionLabel: "No backups",
			Slot:             pluginsv1alpha1.SlotBackupStorage,
			AppliesTo:        []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim},
			Outputs:          []pluginsv1alpha1.OutputDecl{{Key: "objectStorageConfig"}},
		},
	}
}

var _ = Describe("PluginSection schema", func() {
	DescribeTable(
		"admission",
		func(mutate func(*pluginsv1alpha1.PluginSection), wantErr string) {
			obj := validPluginSection()
			mutate(obj)
			expectAdmission(obj, wantErr)
		},
		Entry(
			"admits the backup-storage section",
			func(*pluginsv1alpha1.PluginSection) {}, "",
		),
		Entry(
			"admits a name of 63 characters",
			func(o *pluginsv1alpha1.PluginSection) { o.Name = nameOfLength(63) }, "",
		),
		Entry(
			"refuses a name of 64 characters",
			func(o *pluginsv1alpha1.PluginSection) { o.Name = nameOfLength(64) },
			"metadata.name must be at most 63 characters",
		),
		Entry(
			"admits a section for both claim kinds",
			func(o *pluginsv1alpha1.PluginSection) {
				o.Spec.AppliesTo = []pluginsv1alpha1.ClaimKind{
					pluginsv1alpha1.ClaimKindCamundaClusterClaim,
					pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim,
				}
			}, "",
		),
		Entry(
			"refuses an empty appliesTo",
			func(o *pluginsv1alpha1.PluginSection) { o.Spec.AppliesTo = []pluginsv1alpha1.ClaimKind{} },
			"appliesTo must list at least one claim kind",
		),
		Entry(
			"admits selection Many without a slot",
			func(o *pluginsv1alpha1.PluginSection) {
				o.Spec.Slot = ""
				o.Spec.Selection = pluginsv1alpha1.SelectionMany
			}, "",
		),
		Entry(
			"refuses a slot with selection Many",
			func(o *pluginsv1alpha1.PluginSection) { o.Spec.Selection = pluginsv1alpha1.SelectionMany },
			"a section with a slot must have selection One",
		),
		Entry(
			"admits an emptyOptionLabel of 64 characters",
			func(o *pluginsv1alpha1.PluginSection) { o.Spec.EmptyOptionLabel = strings.Repeat("x", 64) }, "",
		),
		Entry(
			"refuses an emptyOptionLabel of 65 characters",
			func(o *pluginsv1alpha1.PluginSection) { o.Spec.EmptyOptionLabel = strings.Repeat("x", 65) },
			"emptyOptionLabel must be at most 64 characters",
		),
		Entry(
			"refuses a duplicate output key",
			func(o *pluginsv1alpha1.PluginSection) {
				o.Spec.Outputs = append(o.Spec.Outputs, pluginsv1alpha1.OutputDecl{Key: "objectStorageConfig"})
			},
			"spec.outputs[1]: Duplicate value",
		),
	)
})
