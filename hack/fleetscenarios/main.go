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

// Command fleetscenarios generates the page scenarios of the fleet manager.
// It runs every reconciler of the operator on an envtest control plane, so
// the pages are tested against the objects that the operator writes.
//
// # Input
//
// Each directory under -dir that holds a scenario.yaml is one scenario. A
// person writes scenario.yaml. Its keys:
//
//   - schemaVersion: required. Only 1. The generator refuses another value.
//   - description: required. What the scenario shows, and each change it
//     makes to the samples it copies.
//   - seed: required. The Kubernetes objects that a user applies. The
//     generator applies them with server-side apply before the reconcilers
//     start, the Namespaces first.
//   - workload: optional. Each step changes what the workload simulator
//     reports for one object, for the rest of the scenario. A step has kind
//     (StatefulSet, Deployment, or Job), namespace, and name, and then
//     readyReplicas (StatefulSet and Deployment, 0 or more) or jobOutcome
//     (Job, Complete or Failed).
//   - overlays: optional. A status that only an API outside the cluster can
//     produce, such as the user API of Camunda. Each overlay has a reason,
//     the object (apiVersion, kind, namespace, name), and a status that is
//     written as a JSON merge patch after the reconcilers stop. It is the
//     only status that a person writes.
//   - states: optional. Named changes, each on top of the one before. A
//     state has a name (a DNS label other than objects, scenario, and
//     observe), a description, and optional apply (objects), workload
//     (steps that replace the step of the same object; a step that gives a
//     finished Job another outcome fails the run), and overlays (applied
//     after the overlays of the scenario and of the states before it).
//
// For example:
//
//	schemaVersion: 1
//	description: A cluster with 1 of 3 brokers ready, then all 3.
//	seed:
//	  - apiVersion: v1
//	    kind: Namespace
//	    metadata:
//	      name: team-a
//	  - <the CamundaCluster and the objects it references>
//	workload:
//	  - kind: StatefulSet
//	    namespace: team-a
//	    name: orders-zeebe
//	    readyReplicas: 1
//	states:
//	  - name: brokers-ready
//	    description: All 3 brokers are ready.
//	    workload:
//	      - kind: StatefulSet
//	        namespace: team-a
//	        name: orders-zeebe
//	        readyReplicas: 3
//
// # Output
//
// Beside scenario.yaml the generator writes objects.yaml, the objects after
// the seed, and <state>.yaml for each state. Each file opens with a line
// that says it is generated, and holds every object of the namespaces of
// the scenario and of the operator namespace camunda-system, with its
// status and its managed fields, and every cluster-scoped object of the
// operator. Events are of the events.k8s.io API. The generator removes a
// generated file of a state that is gone, and never a file that a person
// wrote. A scenario writes its files only after each of its states settled.
//
// The outputs are normalized, so the same input always writes the same
// bytes: UIDs come from the kind, namespace, and name of an object, there is
// no resource version, every timestamp is 2026-01-01T00:00:00Z, every value
// of a Secret is "scenario", and each Event is written once, named after
// the object it is about, its reason, and its note, with no count.
//
// # Writers
//
// The managed fields name these field managers: kubectl for the seed and
// the objects of a state, the field managers of the reconcilers,
// kube-controller-manager and kubelet for the workload simulator, and
// fleetscenarios-overlay for the overlays.
//
// # Settle
//
// The reconcilers and the simulator run until the scenario is quiet for 2
// seconds: no controller reconciles, the last reconcile that each
// controller finished did not fail, no object changes, the simulator has
// nothing to write, and each object of the operator and each StatefulSet,
// Deployment, and ReplicaSet has observed its generation. A reconcile that
// waits for a later time does not keep a scenario busy. A scenario that is
// not quiet within 60 seconds fails, and the error names what kept it busy.
//
// Usage: fleetscenarios -dir <scenarios dir> [-v]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	fs := flag.NewFlagSet("fleetscenarios", flag.ExitOnError)
	dir := fs.String("dir", "", "the directory of the scenarios (required)")
	verbose := fs.Bool("v", false, "write the logs of the reconcilers to stderr")
	_ = fs.Parse(os.Args[1:])

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "fleetscenarios: -dir is required")
		os.Exit(2)
	}

	logger := logr.Discard()
	if *verbose {
		logger = zap.New(zap.WriteTo(os.Stderr))
	}
	ctrl.SetLogger(logger)

	if err := run(context.Background(), *dir); err != nil {
		fmt.Fprintf(os.Stderr, "fleetscenarios: %v\n", err)
		os.Exit(1)
	}
}

// run generates every scenario under dir: each directory that holds a
// scenario.yaml. A directory without one is skipped. A scenario writes its
// files only after every one of its states settled, so a failure leaves the
// files of that scenario as they were. run stops at the first failure.
func run(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		scenarioDir := filepath.Join(dir, entry.Name())
		path := filepath.Join(scenarioDir, "scenario.yaml")
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		}

		sc, err := readScenario(path)
		if err != nil {
			return err
		}
		outputs, err := generate(ctx, sc)
		if err != nil {
			return fmt.Errorf("scenario %s: %w", entry.Name(), err)
		}
		if err := writeOutputs(scenarioDir, outputs); err != nil {
			return fmt.Errorf("scenario %s: %w", entry.Name(), err)
		}
	}
	return nil
}
