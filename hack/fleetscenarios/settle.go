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
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// settleTimeout bounds each settle of a scenario.
const settleTimeout = 60 * time.Second

// errNotSettled is the cause of the context of settle when its timeout ends.
var errNotSettled = errors.New("the scenario did not settle")

const (
	settleQuiet = 2 * time.Second
	settleTick  = 250 * time.Millisecond
)

// settle runs the simulator until the scenario is quiet, and returns nil
// then. A scenario is quiet when, for settleQuiet without a break, no
// controller runs a reconcile or finishes one, the last reconcile that each
// controller finished did not fail, no object of the scope changes, the
// simulator has nothing to write, and each object that must observe its
// generation has observed it. A reconcile that a controller queued for later
// with RequeueAfter does not keep a scenario busy. When the scenario is not
// quiet within timeout, settle returns an error. before holds the counts of
// controllerActivity from before the reconcilers started, because the counts
// live as long as the process.
func settle(
	ctx context.Context,
	sim *simulator,
	sc *scope,
	timeout time.Duration,
	before map[string]float64,
) error {
	parent := ctx
	// A timer cancels ctx, because a deadline makes the client rate limiter
	// fail early, with an error that is not the error of ctx.
	ctx, cancel := context.WithCancelCause(parent)
	timer := time.AfterFunc(timeout, func() { cancel(errNotSettled) })
	defer timer.Stop()
	defer cancel(nil)

	quietSince := time.Now()
	objectChanged := map[string]time.Time{}
	controllerChanged := map[string]time.Time{}
	var versions map[string]string
	totals := before
	var unobserved []string
	retrying := map[string]bool{}
	var failing []string
	var seen time.Time

	// notSettled reports the state of the last tick that read the whole
	// scenario, because a tick that the timeout stops read only a part.
	notSettled := func(err error) error {
		if !errors.Is(context.Cause(ctx), errNotSettled) || parent.Err() != nil {
			return err
		}
		return fmt.Errorf(
			"the scenario did not settle within %s: objects that kept changing: %s; "+
				"objects that did not observe their generation: %s; controllers that kept reconciling: %s; "+
				"controllers whose last reconcile failed: %s",
			timeout,
			listOrNone(recentlyChanged(objectChanged, seen), true),
			listOrNone(unobserved, false),
			listOrNone(recentlyChanged(controllerChanged, seen), false),
			listOrNone(failing, false),
		)
	}

	for {
		now := time.Now()
		writes, err := sim.pass(ctx)
		if err != nil {
			return notSettled(err)
		}
		busy := writes > 0

		objs, err := sc.list(ctx)
		if err != nil {
			return notSettled(err)
		}
		current := map[string]string{}
		for i := range objs {
			current[objs[i].GetAPIVersion()+" "+describe(&objs[i])] = objs[i].GetResourceVersion()
		}
		for _, key := range changedKeys(versions, current) {
			objectChanged[key] = now
			busy = true
		}
		versions = current

		reconciles, active, err := controllerActivity()
		if err != nil {
			return err
		}
		changed := rose(totals, reconciles)
		for _, key := range changed {
			name, _, _ := strings.Cut(key, " ")
			controllerChanged[name] = now
			busy = true
		}
		updateRetrying(retrying, changed)
		totals = reconciles

		unobserved = sc.unobserved(objs)
		failing = slices.Sorted(maps.Keys(retrying))
		seen = now
		if ctx.Err() != nil {
			return notSettled(ctx.Err())
		}
		if busy || active > 0 || len(unobserved) > 0 || len(failing) > 0 {
			quietSince = now
		} else if now.Sub(quietSince) >= settleQuiet {
			return nil
		}

		select {
		case <-ctx.Done():
			return notSettled(ctx.Err())
		case <-time.After(settleTick):
		}
	}
}

// changedKeys returns each key that is in only one of before and after, or
// whose value differs.
func changedKeys(before, after map[string]string) []string {
	var keys []string
	for key, value := range after {
		if old, ok := before[key]; !ok || old != value {
			keys = append(keys, key)
		}
	}
	for key := range before {
		if _, ok := after[key]; !ok {
			keys = append(keys, key)
		}
	}
	return keys
}

// rose returns each key whose count in after is higher than in before. A key
// that before does not hold counts from 0, because controller-runtime adds the
// counters of a controller at 0 when the controller starts.
func rose(before, after map[string]float64) []string {
	var keys []string
	for key, value := range after {
		if value > before[key] {
			keys = append(keys, key)
		}
	}
	return keys
}

// updateRetrying records in retrying each controller whose last reconcile
// left a rate-limited retry in its queue: an error, or a requeue without a
// delay. changed holds the keys of controllerActivity whose count rose since
// the last tick. A controller whose retry and another result rose in one
// tick counts as retrying, because the two can be of different objects.
func updateRetrying(retrying map[string]bool, changed []string) {
	retried, finished := map[string]bool{}, map[string]bool{}
	for _, key := range changed {
		name, result, _ := strings.Cut(key, " ")
		switch result {
		case "error", "requeue":
			retried[name] = true
		default:
			finished[name] = true
		}
	}

	for name := range retried {
		retrying[name] = true
	}
	for name := range finished {
		if !retried[name] {
			delete(retrying, name)
		}
	}
}

// controllerActivity returns, for each controller of this process and each
// result, the number of reconciles that the controller finished with that
// result, keyed by "<controller> <result>". It also returns the number of
// reconciles that run now across all controllers.
func controllerActivity() (map[string]float64, float64, error) {
	families, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		return nil, 0, fmt.Errorf("reading the controller metrics: %w", err)
	}

	totals := map[string]float64{}
	active := 0.0
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			switch family.GetName() {
			case "controller_runtime_reconcile_total":
				var name, result string
				for _, label := range metric.GetLabel() {
					switch label.GetName() {
					case "controller":
						name = label.GetValue()
					case "result":
						result = label.GetValue()
					}
				}
				totals[name+" "+result] += metric.GetCounter().GetValue()
			case "controller_runtime_active_workers":
				active += metric.GetGauge().GetValue()
			}
		}
	}
	return totals, active, nil
}

// recentlyChanged returns the keys of changed that changed within
// settleQuiet before now, sorted.
func recentlyChanged(changed map[string]time.Time, now time.Time) []string {
	var keys []string
	for _, key := range slices.Sorted(maps.Keys(changed)) {
		if now.Sub(changed[key]) <= settleQuiet {
			keys = append(keys, key)
		}
	}
	return keys
}

// listOrNone joins items, or returns "none" when there are none. With
// dropVersion, it removes the apiVersion that opens each key of an object.
func listOrNone(items []string, dropVersion bool) string {
	if len(items) == 0 {
		return "none"
	}
	if dropVersion {
		for i, item := range items {
			_, rest, _ := strings.Cut(item, " ")
			items[i] = rest
		}
	}
	return strings.Join(items, ", ")
}
