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

package main

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/test/envtest"
)

func TestSettle(t *testing.T) {
	plane, err := envtest.ControlPlane(envtest.Options{})
	require.NoError(t, err)
	cfg, err := plane.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = plane.Stop() })

	ctx := t.Context()
	c, err := client.New(cfg, client.Options{})
	require.NoError(t, err)
	sc, err := newScope(ctx, cfg)
	require.NoError(t, err)
	require.NoError(t, c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "fight"}}))
	sc.add("fight")
	sim := newSimulator(c)
	sim.watch("fight")
	before, _, err := controllerActivity()
	require.NoError(t, err)

	t.Run("returns once the scenario is quiet", func(t *testing.T) {
		start := time.Now()
		require.NoError(t, settle(ctx, sim, sc, 10*time.Second, before))
		assert.Less(t, time.Since(start), 10*time.Second)
	})

	t.Run("fails within its timeout and names an object that keeps changing", func(t *testing.T) {
		flapping := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "fight", Name: "flapping"}}
		require.NoError(t, c.Create(ctx, flapping))
		writerCtx, stop := context.WithCancel(ctx)
		defer stop()
		go keepChanging(writerCtx, c, flapping)

		const timeout = 3 * time.Second
		start := time.Now()
		err := settle(ctx, sim, sc, timeout, before)
		require.Error(t, err)
		assert.ErrorContains(t, err, "ConfigMap fight/flapping")
		assert.Less(t, time.Since(start), timeout+time.Second)
	})

	t.Run("fails within its timeout when one request is slower than the timeout", func(t *testing.T) {
		slow := rest.CopyConfig(cfg)
		slow.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return delayed{next: rt, delay: 5 * time.Second}
		}
		// The discovery of the REST mapper takes no context, so it goes
		// through the client of c.
		slowClient, err := client.New(slow, client.Options{Mapper: c.RESTMapper()})
		require.NoError(t, err)
		slowSim := newSimulator(slowClient)
		slowSim.watch("fight")

		const timeout = time.Second
		start := time.Now()
		err = settle(ctx, slowSim, sc, timeout, before)
		require.Error(t, err)
		assert.ErrorContains(t, err, "did not settle within 1s")
		assert.Less(t, time.Since(start), timeout+time.Second)
	})

	t.Run("fails with its timeout error when the rate limiter cannot wait until the timeout", func(t *testing.T) {
		// One token, then one token every 5 seconds. The second request of
		// settle has to wait past the timeout for its token.
		throttled := rest.CopyConfig(cfg)
		throttled.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(0.2, 1)
		throttledClient, err := client.New(throttled, client.Options{Mapper: c.RESTMapper()})
		require.NoError(t, err)
		throttledSim := newSimulator(throttledClient)
		throttledSim.watch("fight")

		const timeout = time.Second
		start := time.Now()
		err = settle(ctx, throttledSim, sc, timeout, before)
		require.Error(t, err)
		assert.ErrorContains(t, err, "did not settle within 1s")
		assert.Less(t, time.Since(start), timeout+time.Second)
	})
}

// delayed holds each request for delay before it sends it, or until the
// context of the request ends.
type delayed struct {
	next  http.RoundTripper
	delay time.Duration
}

func (d delayed) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	case <-time.After(d.delay):
	}
	return d.next.RoundTrip(req)
}

// keepChanging writes a new label value on obj every 100 ms until ctx ends,
// like two writers that fight over one field.
func keepChanging(ctx context.Context, c client.Client, obj *corev1.ConfigMap) {
	for i := 0; ; i++ {
		var latest corev1.ConfigMap
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), &latest); err == nil {
			latest.Labels = map[string]string{"round": strconv.Itoa(i)}
			_ = c.Update(ctx, &latest)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestRoseIgnoresACounterThatAControllerAddsAtZero(t *testing.T) {
	before := map[string]float64{"database success": 2}
	after := map[string]float64{"database success": 2, "camundacluster error": 0, "camundacluster success": 1}

	assert.Equal(t, []string{"camundacluster success"}, rose(before, after))
}

func TestUpdateRetryingKeepsAControllerWhoseLastReconcileFailed(t *testing.T) {
	retrying := map[string]bool{}

	updateRetrying(retrying, []string{"camundacluster error", "database success"})
	assert.Equal(t, map[string]bool{"camundacluster": true}, retrying, "an error leaves a retry queued")

	updateRetrying(retrying, []string{"database requeue_after"})
	assert.Equal(t, map[string]bool{"camundacluster": true}, retrying, "another controller does not clear it")

	updateRetrying(retrying, []string{"camundacluster error", "camundacluster success"})
	assert.Equal(t, map[string]bool{"camundacluster": true}, retrying, "a success can be of another object")

	updateRetrying(retrying, []string{"camundacluster success"})
	assert.Empty(t, retrying, "a later success clears it")

	updateRetrying(retrying, []string{"database requeue"})
	assert.Equal(t, map[string]bool{"database": true}, retrying, "a requeue without a delay leaves a retry queued")

	mixed := map[string]bool{}
	updateRetrying(mixed, []string{"logicalbackupelasticsearch error", "logicalbackupelasticsearch success"})
	assert.Equal(t, map[string]bool{"logicalbackupelasticsearch": true}, mixed, "a first tick with both results")
}

func TestSettleTimeoutIsOneMinute(t *testing.T) {
	assert.Equal(t, time.Minute, settleTimeout, "the issue bounds a settle at 60 seconds")
}
