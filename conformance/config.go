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

package conformance

import (
	"errors"
	"fmt"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// Config is the input of a conformance run. Run refuses a config that is not
// valid, and names each problem.
type Config struct {
	// Registration is the name of the PluginRegistration under test.
	Registration string
	// Namespace is the namespace of the requests of the run.
	Namespace string
	// Cases are the requests of the run, one PluginRequest each.
	Cases []Case
	// Timeouts are the longest waits of the run.
	Timeouts Timeouts
	// Skip names the checks that the run does not run. Only a skippable
	// check can be skipped, and each skip needs a reason.
	Skip []Skip
	// RESTConfig is the connection to the cluster of the run. Nil takes the
	// first of the --kubeconfig flag, the KUBECONFIG file, the in-cluster
	// config, and $HOME/.kube/config, as controller-runtime GetConfig does.
	RESTConfig *rest.Config
}

func (c Config) validate() error {
	var errs []error

	if c.Registration == "" {
		errs = append(errs, errors.New("registration is empty"))
	}

	if c.Namespace == "" {
		errs = append(errs, errors.New("namespace is empty"))
	}

	if len(c.Cases) == 0 {
		errs = append(errs, errors.New("a run needs at least one case"))
	}

	names := make(map[string]bool, len(c.Cases))
	for i, tc := range c.Cases {
		switch {
		case tc.Name == "":
			errs = append(errs, fmt.Errorf("case %d has no name", i))
		case names[tc.Name]:
			errs = append(errs, fmt.Errorf("case name %q is used more than once", tc.Name))
		}
		names[tc.Name] = true

		switch tc.DeletionPolicy {
		case "", pluginsv1alpha1.DeletionPolicyDelete, pluginsv1alpha1.DeletionPolicyOrphan:
		default:
			errs = append(errs, fmt.Errorf(
				"case %q has the deletion policy %q, which is not Delete or Orphan",
				tc.Name,
				tc.DeletionPolicy,
			))
		}
	}

	errs = append(errs, c.Timeouts.validate())

	return errors.Join(errs...)
}

// Case is one request of a run, as a claim controller writes it. Each
// deletion policy of a case names a claim of its own, so names that a plugin
// derives from the claim do not collide between the Delete and the Orphan
// request.
type Case struct {
	// Name names the case in the report. It is unique in the config.
	Name string
	// Inputs are the resolved inputs of the request.
	Inputs map[string]string
	// Parameters are the parameters of the request.
	Parameters *apiextensionsv1.JSON
	// Target is the resource that the request serves.
	Target pluginsv1alpha1.TargetRef
	// DeletionPolicy is the deletion policy of the request. It is empty,
	// Delete, or Orphan. An empty policy runs the case once for each policy
	// of the registration. When the registration has two policies, the target
	// name of each of these runs ends in -delete or -orphan, and so does the
	// ServiceAccount name when the case sets one.
	DeletionPolicy pluginsv1alpha1.DeletionPolicy
}

// Skip is a check that a run does not run, with the reason that the report
// shows.
type Skip struct {
	ID     CheckID
	Reason string
}

// Timeouts are the longest waits of a run. A zero field takes its default.
// A negative field is refused.
type Timeouts struct {
	// OutputsReady is the wait for OutputsReady=True on a request. The
	// default is 5 minutes.
	OutputsReady time.Duration
	// Ready is the wait for Ready=True on a request. The default is 10
	// minutes.
	Ready time.Duration
	// Deletion bounds each wait for a deletion: of a request and its objects,
	// of a terminating namespace, and of the cleanups. The default is 5 minutes.
	Deletion time.Duration
}

func (t Timeouts) withDefaults() Timeouts {
	if t.OutputsReady == 0 {
		t.OutputsReady = 5 * time.Minute
	}

	if t.Ready == 0 {
		t.Ready = 10 * time.Minute
	}

	if t.Deletion == 0 {
		t.Deletion = 5 * time.Minute
	}

	return t
}

func (t Timeouts) validate() error {
	var errs []error
	for _, f := range []struct {
		name string
		d    time.Duration
	}{
		{"OutputsReady", t.OutputsReady},
		{"Ready", t.Ready},
		{"Deletion", t.Deletion},
	} {
		if f.d < 0 {
			errs = append(errs, fmt.Errorf("timeout %s is negative: %s", f.name, f.d))
		}
	}

	return errors.Join(errs...)
}
