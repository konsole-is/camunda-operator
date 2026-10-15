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

package pluginstartup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// The interceptors never call through, so the fake client needs no plugin
// types in its scheme.

func TestStartTriesAgainAfterAFailedApply(t *testing.T) {
	var calls int
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
			calls++
			if calls == 1 {
				return errors.New("the server could not find the requested resource")
			}
			return nil
		},
	}).Build()

	w := New(c, "0.3.0", testServiceAccount)
	w.retryDelay = time.Millisecond

	require.NoError(t, w.Start(t.Context()))

	// One failed call, then one call for each object.
	assert.Equal(t, 1+len(Sections())+len(Registrations("0.3.0", testServiceAccount)), calls)
}

func TestStartReturnsWhenTheContextEnds(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
			return errors.New("refused")
		},
	}).Build()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	w := New(c, "0.3.0", testServiceAccount)
	w.retryDelay = time.Millisecond

	assert.NoError(t, w.Start(ctx))
}

func TestWriterNeedsLeaderElection(t *testing.T) {
	assert.True(t, New(nil, "0.3.0", testServiceAccount).NeedLeaderElection())
}
