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

// Package grace holds the grace periods of the components that run a
// workload, and the wait until one of them runs out.
package grace

import (
	"flag"
	"fmt"
	"slices"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultWorkload is the default of Periods.Workload.
	DefaultWorkload = 30 * time.Minute
	// DefaultDatastore is the default of Periods.Datastore.
	DefaultDatastore = 30 * time.Minute
)

const (
	// WorkloadFlag is the manager flag that sets Periods.Workload.
	WorkloadFlag = "workload-grace-period"
	// WorkloadEnv is the environment variable that defaults WorkloadFlag.
	WorkloadEnv = "CAMUNDA_OPERATOR_WORKLOAD_GRACE_PERIOD"
	// DatastoreFlag is the manager flag that sets Periods.Datastore.
	DatastoreFlag = "datastore-grace-period"
	// DatastoreEnv is the environment variable that defaults DatastoreFlag.
	DatastoreEnv = "CAMUNDA_OPERATOR_DATASTORE_GRACE_PERIOD"
)

// Periods are the grace periods of the components that run a workload. A
// component that is not ready when its grace period runs out reports Degraded
// or Down in place of a not-ready reason, such as Creating, Failing, or
// Blocked. Zero turns the grading off for that class.
type Periods struct {
	// Workload is the grace period of a component that runs a Deployment, a
	// StatefulSet, or a Keycloak.
	Workload time.Duration
	// Datastore is the grace period of a component that runs an
	// Elasticsearch cluster or a PostgreSQL cluster.
	Datastore time.Duration
}

// BindFlags registers WorkloadFlag and DatastoreFlag on fs and returns the
// check to call after fs.Parse. The default of each flag is its environment
// variable, read through getenv, and then DefaultWorkload or DefaultDatastore.
// The check returns an error when a period is negative, or when an
// environment variable holds a value that is not a duration and its flag was
// not set.
func (p *Periods) BindFlags(fs *flag.FlagSet, getenv func(string) string) func() error {
	workload, workloadErr := durationEnv(getenv, WorkloadEnv, DefaultWorkload)
	datastore, datastoreErr := durationEnv(getenv, DatastoreEnv, DefaultDatastore)

	fs.DurationVar(
		&p.Workload, WorkloadFlag, workload,
		"How long a Deployment, a StatefulSet, or a Keycloak may take to become ready "+
			"before its condition reports Degraded or Down. 0 turns this off. "+
			"Defaults to the "+WorkloadEnv+" environment variable.",
	)
	fs.DurationVar(
		&p.Datastore, DatastoreFlag, datastore,
		"How long an Elasticsearch or a PostgreSQL cluster may take to become ready "+
			"before its condition reports Degraded or Down. 0 turns this off. "+
			"Defaults to the "+DatastoreEnv+" environment variable.",
	)

	return func() error {
		set := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

		if workloadErr != nil && !set[WorkloadFlag] {
			return workloadErr
		}

		if datastoreErr != nil && !set[DatastoreFlag] {
			return datastoreErr
		}

		return p.Validate()
	}
}

// durationEnv returns the duration in the environment variable name, or
// fallback when it is unset or empty, or fallback and an error when it is bad.
func durationEnv(getenv func(string) string, name string, fallback time.Duration) (time.Duration, error) {
	value := getenv(name)
	if value == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback, fmt.Errorf("reading %s: %w", name, err)
	}

	return d, nil
}

// Validate returns an error when a grace period is negative.
func (p Periods) Validate() error {
	if p.Workload < 0 {
		return fmt.Errorf("--%s (%s) must not be negative, got %s", WorkloadFlag, WorkloadEnv, p.Workload)
	}

	if p.Datastore < 0 {
		return fmt.Errorf("--%s (%s) must not be negative, got %s", DatastoreFlag, DatastoreEnv, p.Datastore)
	}

	return nil
}

// Remaining returns the time until the grace period of the first of comps
// runs out on owner, or 0 when none of them waits on one. Call it after the
// components reconciled, with the period they were built with. A controller
// requeues after the result, because no watch event arrives when a grace
// period runs out.
//
// A component waits on its grace period while its condition is False, not yet
// Down or Degraded, and not in a reason that ocf keeps outside the grace
// period, such as PrerequisiteNotMet or a suspension.
func Remaining(
	owner component.OperatorCRD, period time.Duration, now time.Time, comps ...*component.Component,
) time.Duration {
	if period <= 0 {
		return 0
	}

	var soonest time.Duration
	for _, comp := range comps {
		cond := comp.GetCondition(owner)
		if cond.Status != metav1.ConditionFalse {
			continue
		}

		switch cond.ComponentStatus() {
		case component.Down, component.Degraded,
			component.Unknown, component.PrerequisiteNotMet, component.FeatureGateError,
			component.PendingSuspension, component.Suspending:
			continue
		}

		// ocf grades a condition only once more than the period has passed,
		// and lastTransitionTime keeps whole seconds on the server.
		left := cond.LastTransitionTime.Add(period + time.Second).Sub(now)
		if left <= 0 {
			// The grace period ran out and ocf graded the component
			// Healthy: it keeps its progress reason until an event comes.
			continue
		}

		soonest = Sooner(soonest, left)
	}

	return soonest
}

// Restart removes the condition of conditionType from owner when its reason
// is one of reasons, so the component starts a new grace period when it
// reconciles next. ocf counts the grace period from the lastTransitionTime of
// the condition it finds. A controller calls Restart before the reconcile
// when a reason that holds the component back, such as one that the
// controller staged itself, no longer applies.
func Restart(owner component.OperatorCRD, conditionType string, reasons ...string) {
	cond := meta.FindStatusCondition(*owner.GetStatusConditions(), conditionType)
	if cond != nil && slices.Contains(reasons, cond.Reason) {
		meta.RemoveStatusCondition(owner.GetStatusConditions(), conditionType)
	}
}

// Sooner returns the shorter of two waits. Zero means no wait, so it returns
// the other one.
func Sooner(a, b time.Duration) time.Duration {
	switch {
	case a == 0:
		return b
	case b == 0:
		return a
	default:
		return min(a, b)
	}
}
