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

package labels_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/components/database"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

// docs/architecture.md promises these component values to selectors outside
// this repository. The values of the Elasticsearch and PostgreSQL objects are
// pinned in their own packages.
func TestDocumentedComponentValuesAreStable(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "zeebe", camundacluster.ComponentZeebe)
	assert.Equal(t, "gateway", camundacluster.ComponentGateway)
	assert.Equal(t, "operate", camundacluster.ComponentOperate)
	assert.Equal(t, "tasklist", camundacluster.ComponentTasklist)
	assert.Equal(t, "admin", camundacluster.ComponentAdmin)
	assert.Equal(t, "connectors", camundacluster.ComponentConnectors)
	assert.Equal(t, "optimize-webapp", camundaoptimize.ComponentWebapp)
	assert.Equal(t, "optimize-importer", camundaoptimize.ComponentImporter)
	assert.Equal(t, "web-modeler-cluster-user", camundamanagementcluster.ComponentWebModelerClusterUser)
	assert.Equal(t, "storage-writer", storagewriter.Component)
	assert.Equal(t, "restore", restore.ComponentRestore)

	db := &v1.Database{ObjectMeta: metav1.ObjectMeta{Name: "my-db"}}
	assert.Equal(t, "database", database.BindingSelector(db)[labels.ComponentKey])
}
