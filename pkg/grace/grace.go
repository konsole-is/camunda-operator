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
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultWorkload is the default of Periods.Workload.
	DefaultWorkload = 15 * time.Minute
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
// or Down in place of Creating, Updating, or Scaling. Zero turns the grading
// off for that class: the component keeps its progress reason.
type Periods struct {
	// Workload is the grace period of a component that runs a Deployment, a
	// StatefulSet, or a Keycloak.
	Workload time.Duration
	// Datastore is the grace period of a component that runs an
	// Elasticsearch cluster or a PostgreSQL cluster.
	Datastore time.Duration
}

// BindFlags registers WorkloadFlag and DatastoreFlag on fs. The default of
// each flag is its environment variable, read through getenv, and then
// DefaultWorkload or DefaultDatastore. It returns an error when an environment
// variable is set to a value that is not a duration. Call Validate after
// fs.Parse.
func (p *Periods) BindFlags(fs *flag.FlagSet, getenv func(string) string) error {
	workload, err := durationEnv(getenv, WorkloadEnv, DefaultWorkload)
	if err != nil {
		return err
	}

	datastore, err := durationEnv(getenv, DatastoreEnv, DefaultDatastore)
	if err != nil {
		return err
	}

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

	return nil
}

// durationEnv returns the duration in the environment variable name, or
// fallback when the variable is unset or empty.
func durationEnv(getenv func(string) string, name string, fallback time.Duration) (time.Duration, error) {
	value := getenv(name)
	if value == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", name, err)
	}

	return d, nil
}

// Validate returns an error when a grace period is negative.
func (p Periods) Validate() error {
	if p.Workload < 0 {
		return fmt.Errorf("--%s must not be negative, got %s", WorkloadFlag, p.Workload)
	}

	if p.Datastore < 0 {
		return fmt.Errorf("--%s must not be negative, got %s", DatastoreFlag, p.Datastore)
	}

	return nil
}

// Remaining returns the time until the grace period of the first of comps
// runs out on owner, or 0 when none of them waits on one. Call it after the
// components reconciled, with the period they were built with. A controller
// requeues after the result, because no watch event arrives when a grace
// period runs out.
//
// A component waits on its grace period while its condition is False and not
// yet graded: a reason of Down or Degraded is final, and Unknown or
// PrerequisiteNotMet has not started the grace period yet.
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
		case component.Unknown, component.PrerequisiteNotMet, component.Down, component.Degraded:
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
