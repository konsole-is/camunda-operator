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
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// settleTimeout bounds each settle of a scenario.
const settleTimeout = 60 * time.Second

const (
	settleQuiet = 2 * time.Second
	settleTick  = 250 * time.Millisecond
)

// settle runs the simulator until the scenario is quiet, and returns nil
// then. A scenario is quiet when, for settleQuiet without a break, no
// controller runs a reconcile or finishes one, no object of the scope
// changes, the simulator has nothing to write, and each object that must
// observe its generation has observed it. A reconcile that a controller
// queued for later with RequeueAfter does not keep a scenario busy. When the
// scenario is not quiet within timeout, settle returns an error that names
// each object that changed in the last settleQuiet, each object that did not
// observe its generation, and each controller that reconciled in that time.
func settle(ctx context.Context, sim *simulator, sc *scope, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	quietSince := time.Now()
	objectChanged := map[string]time.Time{}
	controllerChanged := map[string]time.Time{}
	var versions map[string]string
	var totals map[string]float64
	var unobserved []string

	for {
		now := time.Now()
		writes, err := sim.pass(ctx)
		if err != nil {
			return err
		}
		busy := writes > 0

		objs, err := sc.list(ctx)
		if err != nil {
			return err
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
		for _, name := range changedKeys(totals, reconciles) {
			controllerChanged[name] = now
			busy = true
		}
		totals = reconciles

		unobserved = sc.unobserved(objs)
		if busy || active > 0 || len(unobserved) > 0 {
			quietSince = now
		} else if now.Sub(quietSince) >= settleQuiet {
			return nil
		}

		if now.After(deadline) {
			return fmt.Errorf(
				"the scenario did not settle within %s: objects that kept changing: %s; "+
					"objects that did not observe their generation: %s; controllers that kept reconciling: %s",
				timeout,
				listOrNone(recentlyChanged(objectChanged, now), true),
				listOrNone(unobserved, false),
				listOrNone(recentlyChanged(controllerChanged, now), false),
			)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(settleTick):
		}
	}
}

// changedKeys returns each key that is in only one of before and after, or
// whose value differs.
func changedKeys[V comparable](before, after map[string]V) []string {
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

// controllerActivity returns, for each controller of this process, the
// number of reconciles it finished, and the number of reconciles that run
// now across all controllers.
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
				for _, label := range metric.GetLabel() {
					if label.GetName() == "controller" {
						totals[label.GetValue()] += metric.GetCounter().GetValue()
					}
				}
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
