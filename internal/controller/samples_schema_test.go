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

package controller

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// implementedKindSamples lists the sample manifests of the kinds whose API
// types are implemented and therefore validate a real schema. The samples of
// stub kinds are covered when their batch lands.
var implementedKindSamples = []string{
	"core_v1_databaseserverconfig.yaml",
	"core_v1_databaseconfig.yaml",
	"core_v1_secondarystorageconfig.yaml",
	"core_v1_objectstorageconfig.yaml",
	"core_v1_managementauthconfig.yaml",
	"core_v1_elasticsearchcluster.yaml",
	"core_v1_elasticsearchclusterpreset.yaml",
	"core_v1_database.yaml",
	"core_v1_camundaplatformconfig.yaml",
	"core_v1_camundacluster.yaml",
	"core_v1_camundaclusterpreset.yaml",
	"core_v1_logicalrestore.yaml",
	"core_v1_pointintimerestore.yaml",
}

var _ = Describe("config/samples", func() {
	It("applies every implemented kind's sample against the schema", func() {
		for _, file := range implementedKindSamples {
			data, err := os.ReadFile(filepath.Join("..", "..", "config", "samples", file))
			Expect(err).NotTo(HaveOccurred(), file)

			var obj unstructured.Unstructured
			Expect(yaml.Unmarshal(data, &obj.Object)).To(Succeed(), file)

			gvk := obj.GroupVersionKind()
			mapping, err := k8sClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
			Expect(err).NotTo(HaveOccurred(), file)
			if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
				obj.SetNamespace(fixtures.SchemaTestNamespace)
			}

			Expect(k8sClient.Create(ctx, &obj)).To(Succeed(), file)
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, &obj) })
		}
	})
})
