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

package restore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

func TestWriterRateLimiterRetriesBeforeARegistrationExpires(t *testing.T) {
	limiter := WriterRateLimiter()
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "apps", Name: "restore"}}

	for range 40 {
		assert.LessOrEqual(t, limiter.When(req), storagewriter.RenewInterval)
	}
	assert.Less(t, storagewriter.RenewInterval, storagewriter.Duration)
}
