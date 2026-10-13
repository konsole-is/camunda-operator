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
	"errors"
	"fmt"
	"os"
	"slices"

	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"
)

// schemaVersion is the only version of scenario.yaml that this generator
// reads.
const schemaVersion = 1

// The kinds that a workload step can name, and the outcomes of a Job step.
const (
	kindStatefulSet = "StatefulSet"
	kindDeployment  = "Deployment"
	kindJob         = "Job"

	jobComplete = "Complete"
	jobFailed   = "Failed"
)

// reservedStateNames are the names of the other files beside a
// scenario.yaml: the input, the output of the seed, and the hand-written
// observe.yaml of the Playwright tests.
var reservedStateNames = []string{"scenario", "objects", "observe"}

// scenario is the content of one scenario.yaml. The package doc of main
// describes each key.
type scenario struct {
	SchemaVersion int              `json:"schemaVersion"`
	Description   string           `json:"description"`
	Seed          []map[string]any `json:"seed"`
	Workload      []workloadStep   `json:"workload,omitempty"`
	Overlays      []overlay        `json:"overlays,omitempty"`
	States        []state          `json:"states,omitempty"`
}

// workloadStep changes what the simulator reports for one workload. It holds
// for the rest of the scenario, or until a later state gives a step for the
// same object.
type workloadStep struct {
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	ReadyReplicas *int32 `json:"readyReplicas,omitempty"`
	JobOutcome    string `json:"jobOutcome,omitempty"`
}

// objectRef names one object of a scenario.
type objectRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
}

// overlay is a status that only an API outside the cluster can produce. It
// is written as a JSON merge patch after the reconcilers stop.
type overlay struct {
	Reason string         `json:"reason"`
	Object objectRef      `json:"object"`
	Status map[string]any `json:"status"`
}

// state is a named change on top of the state before it. Its output file is
// <name>.yaml.
type state struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Apply       []map[string]any `json:"apply,omitempty"`
	Delete      []objectRef      `json:"delete,omitempty"`
	Workload    []workloadStep   `json:"workload,omitempty"`
	Overlays    []overlay        `json:"overlays,omitempty"`
}

// readScenario reads and checks the scenario.yaml at path. It refuses a
// schemaVersion other than schemaVersion, a key it does not know, and an
// input that fails validate.
func readScenario(path string) (*scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// The version is read on its own first, so an input of a newer version
	// gets this message and not an unknown-key error.
	var version struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := yaml.Unmarshal(data, &version); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if version.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf(
			"%s: schemaVersion %d is not supported; this generator reads schemaVersion %d",
			path, version.SchemaVersion, schemaVersion,
		)
	}

	var s scenario
	if err := yaml.UnmarshalStrict(data, &s); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &s, nil
}

// validate returns every problem of s, each one naming where it is.
func (s *scenario) validate() error {
	var errs []error
	if s.Description == "" {
		errs = append(errs, errors.New("description is empty"))
	}
	if len(s.Seed) == 0 {
		errs = append(errs, errors.New("seed is empty"))
	}
	errs = append(errs, validateObjects("seed object", s.Seed)...)
	errs = append(errs, validateSteps("the scenario", s.Workload)...)
	errs = append(errs, validateOverlays("the scenario", s.Overlays)...)

	seen := map[string]bool{}
	for i, st := range s.States {
		where := fmt.Sprintf("state %q", st.Name)
		switch {
		case slices.Contains(reservedStateNames, st.Name):
			errs = append(errs, fmt.Errorf(
				"state %d has the name %q, which is the name of another file of the scenario", i+1, st.Name,
			))
		case len(validation.IsDNS1123Label(st.Name)) > 0:
			errs = append(errs, fmt.Errorf("state name %q is not a DNS label", st.Name))
		case seen[st.Name]:
			errs = append(errs, fmt.Errorf("two states have the name %q", st.Name))
		}
		seen[st.Name] = true

		if st.Description == "" {
			errs = append(errs, fmt.Errorf("%s has no description", where))
		}
		errs = append(errs, validateObjects("object to apply of "+where+",", st.Apply)...)
		for j, ref := range st.Delete {
			if ref.APIVersion == "" || ref.Kind == "" || ref.Name == "" {
				errs = append(errs, fmt.Errorf("object to delete %d of %s names no object", j+1, where))
			}
		}
		errs = append(errs, validateSteps(where, st.Workload)...)
		errs = append(errs, validateOverlays(where, st.Overlays)...)
	}

	return errors.Join(errs...)
}

// validateObjects returns an error for each object of objs that has no
// apiVersion, kind, or metadata.name. what names the list in the message.
func validateObjects(what string, objs []map[string]any) []error {
	var errs []error
	for i, obj := range objs {
		apiVersion, _ := obj["apiVersion"].(string)
		kind, _ := obj["kind"].(string)
		metadata, _ := obj["metadata"].(map[string]any)
		name, _ := metadata["name"].(string)
		if apiVersion == "" || kind == "" || name == "" {
			errs = append(errs, fmt.Errorf("%s %d has no apiVersion, kind, or metadata.name", what, i+1))
		}
	}
	return errs
}

// validateSteps returns an error for each step of steps that names no
// object, names a kind the simulator does not run, or gives the field of
// another kind. where names the scenario or the state in the message.
func validateSteps(where string, steps []workloadStep) []error {
	var errs []error
	for i, step := range steps {
		prefix := fmt.Sprintf("workload step %d of %s", i+1, where)
		if step.Namespace == "" || step.Name == "" {
			errs = append(errs, fmt.Errorf("%s names no namespace and name", prefix))
		}

		switch step.Kind {
		case kindStatefulSet, kindDeployment:
			if step.ReadyReplicas == nil || *step.ReadyReplicas < 0 || step.JobOutcome != "" {
				errs = append(errs, fmt.Errorf(
					"%s: a %s step needs readyReplicas of 0 or more, and no jobOutcome", prefix, step.Kind,
				))
			}
		case kindJob:
			if (step.JobOutcome != jobComplete && step.JobOutcome != jobFailed) || step.ReadyReplicas != nil {
				errs = append(errs, fmt.Errorf(
					"%s: a Job step needs jobOutcome Complete or Failed, and no readyReplicas", prefix,
				))
			}
		default:
			errs = append(errs, fmt.Errorf(
				"%s: kind %q is not StatefulSet, Deployment, or Job", prefix, step.Kind,
			))
		}
	}
	return errs
}

// validateOverlays returns an error for each overlay of overlays that has no
// reason, names no object, or holds no status. where names the scenario or
// the state in the message.
func validateOverlays(where string, overlays []overlay) []error {
	var errs []error
	for i, o := range overlays {
		prefix := fmt.Sprintf("overlay %d of %s", i+1, where)
		if o.Reason == "" {
			errs = append(errs, fmt.Errorf("%s has no reason", prefix))
		}
		if o.Object.APIVersion == "" || o.Object.Kind == "" || o.Object.Name == "" {
			errs = append(errs, fmt.Errorf("%s names no object", prefix))
		}
		if len(o.Status) == 0 {
			errs = append(errs, fmt.Errorf("%s has no status", prefix))
		}
	}
	return errs
}
