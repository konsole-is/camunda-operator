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

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Existing objects carry these condition types in their status, and users
// match them with kubectl wait. A changed string strands the old condition.
func TestComponentConditionTypesKeepTheirStoredStrings(t *testing.T) {
	cases := []struct{ got, want string }{
		{v1.ConditionCredentialsReady, "CredentialsReady"},
		{v1.ConditionElasticsearchReady, "ElasticsearchReady"},
		{v1.ConditionStorageContractReady, "StorageContractReady"},
		{v1.ConditionKeystoreReady, "KeystoreReady"},
		{v1.ConditionSnapshotRepositoryReady, "SnapshotRepositoryReady"},
		{v1.ConditionMetricsReady, "MetricsReady"},
		{v1.ConditionBindingsReady, "BindingsReady"},
	}

	for _, c := range cases {
		assert.Equal(t, c.want, c.got)
	}
}
