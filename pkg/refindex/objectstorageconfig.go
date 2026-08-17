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

package refindex

import (
	"context"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// ObjectStorageConfigSecretField indexes ObjectStorageConfigs by the Secret
// that holds their static credentials, keyed as "<namespace>/<name>". Every
// controller that maps a Secret event to the bucket contracts naming it lists
// with this field, so there is exactly one definition of that mapping.
const ObjectStorageConfigSecretField = "objectstorageconfig.spec.credentialsSecretRef"

// ensured records which managers already carry the index, because
// controller-runtime rejects a second registration of the same field and
// several controllers of one manager need it.
var (
	ensuredMu sync.Mutex
	ensured   = map[ctrl.Manager]bool{}
)

// EnsureObjectStorageConfigSecretIndex registers the
// ObjectStorageConfigSecretField index on mgr, once. Every caller of the
// field calls it in its SetupWithManager; the first one registers, the rest
// are no-ops.
func EnsureObjectStorageConfigSecretIndex(mgr ctrl.Manager) error {
	ensuredMu.Lock()
	defer ensuredMu.Unlock()
	if ensured[mgr] {
		return nil
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.ObjectStorageConfig{},
		ObjectStorageConfigSecretField, func(o client.Object) []string {
			creds := o.(*v1.ObjectStorageConfig).CredentialsSecret()
			if creds == nil {
				return nil
			}
			return []string{NamespacedKey(creds.Namespace, creds.Name)}
		},
	); err != nil {
		return err
	}
	ensured[mgr] = true

	return nil
}
