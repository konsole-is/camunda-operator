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

package echo

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// Options configures one echo controller, which serves one
// PluginRegistration.
type Options struct {
	// Registration is the name of the PluginRegistration that the controller
	// writes and serves.
	Registration string
	// Sections are the sections that the registration serves, for example
	// "echo", or "access" and "backup-storage".
	Sections []string
	// ServiceAccount is the ServiceAccount that the registration names.
	ServiceAccount pluginsv1alpha1.ServiceAccountRef
	// Inputs are the inputs that the registration declares.
	Inputs []pluginsv1alpha1.InputDecl
	// Outputs are the outputs that the controller writes for each request.
	Outputs []Output
	// GrantTemplates are the grant templates that the registration
	// publishes. The controller writes one Grant for each template whose
	// required inputs all have a value in the request.
	GrantTemplates []pluginsv1alpha1.GrantTemplate
	// Defect makes the controller break one rule of the contract. The zero
	// value keeps the contract.
	Defect Defect
}

// Output is one output that the controller writes for each request.
type Output struct {
	// Key is the key of the output.
	Key string
	// FromInput names the input of the request whose value the output
	// copies. When it is empty, the output is Value.
	FromInput string
	// Value is the output when FromInput is empty.
	Value string
	// Section is the section whose requests get the output. When it is
	// empty, every request gets it.
	Section string
}

// Defect makes the controller break one rule of the plugin contract.
type Defect string

const (
	// DefectNone keeps the contract.
	DefectNone Defect = ""
	// DefectLateFinalizer writes the objects of a request before it adds the
	// cleanup finalizer.
	DefectLateFinalizer Defect = "LateFinalizer"
	// DefectStaleOutputsReady writes OutputsReady for the generation before
	// the current one.
	DefectStaleOutputsReady Defect = "StaleOutputsReady"
	// DefectMissingOutput leaves the last output out of the status.
	DefectMissingOutput Defect = "MissingOutput"
	// DefectNotReady writes Ready=False with the reason EchoDefect.
	DefectNotReady Defect = "NotReady"
	// DefectNoOwnerReference writes the ConfigMap with no owner reference to
	// the request.
	DefectNoOwnerReference Defect = "NoOwnerReference"
	// DefectClusterOwnerReference gives the cluster-scoped StorageClass an
	// owner reference to the request.
	DefectClusterOwnerReference Defect = "ClusterOwnerReference"
	// DefectUnlistedResource leaves the StorageClass out of
	// status.resources.
	DefectUnlistedResource Defect = "UnlistedResource"
	// DefectNoEvents records no event on the request.
	DefectNoEvents Defect = "NoEvents"
	// DefectRelativeLink writes a link with a relative URL.
	DefectRelativeLink Defect = "RelativeLink"
	// DefectDropAccepted removes the Accepted condition of the registration,
	// which the registry controller owns.
	DefectDropAccepted Defect = "DropAccepted"
	// DefectKeepOnDelete keeps the objects of a request with the deletion
	// policy Delete.
	DefectKeepOnDelete Defect = "KeepOnDelete"
	// DefectKeepOwnerOnOrphan keeps the owner reference of the ConfigMap with
	// the deletion policy Orphan.
	DefectKeepOwnerOnOrphan Defect = "KeepOwnerOnOrphan"
	// DefectKeepRouteOnOrphan keeps the Ingress to a missing Service with the
	// deletion policy Orphan.
	DefectKeepRouteOnOrphan Defect = "KeepRouteOnOrphan"
	// DefectWrongFragment adds the key EchoDefect to the fragment of each
	// Grant, so that it is not the filled-in template.
	DefectWrongFragment Defect = "WrongFragment"
)

// defects are the defects that New accepts.
var defects = []Defect{
	DefectNone,
	DefectLateFinalizer,
	DefectStaleOutputsReady,
	DefectMissingOutput,
	DefectNotReady,
	DefectNoOwnerReference,
	DefectClusterOwnerReference,
	DefectUnlistedResource,
	DefectNoEvents,
	DefectRelativeLink,
	DefectDropAccepted,
	DefectKeepOnDelete,
	DefectKeepOwnerOnOrphan,
	DefectKeepRouteOnOrphan,
	DefectWrongFragment,
}

func (o Options) validate() error {
	var errs []error

	if o.Registration == "" {
		errs = append(errs, errors.New("option Registration is empty"))
	}

	if len(o.Sections) == 0 {
		errs = append(errs, errors.New("option Sections is empty"))
	}

	if o.ServiceAccount.Name == "" {
		errs = append(errs, errors.New("option ServiceAccount.Name is empty"))
	}

	if o.ServiceAccount.Namespace == "" {
		errs = append(errs, errors.New("option ServiceAccount.Namespace is empty"))
	}

	for i, out := range o.Outputs {
		if out.Key == "" {
			errs = append(errs, fmt.Errorf("option Outputs[%d].Key is empty", i))
		}

		declared := slices.ContainsFunc(
			o.Inputs,
			func(in pluginsv1alpha1.InputDecl) bool { return in.Name == out.FromInput },
		)
		if out.FromInput != "" && !declared {
			errs = append(
				errs,
				fmt.Errorf("option Outputs[%d].FromInput names %q, which Inputs does not declare", i, out.FromInput),
			)
		}

		if out.Section != "" && !slices.Contains(o.Sections, out.Section) {
			errs = append(
				errs,
				fmt.Errorf("option Outputs[%d].Section names %q, which Sections does not hold", i, out.Section),
			)
		}
	}

	for i, tmpl := range o.GrantTemplates {
		errs = append(errs, grantTemplateErrors(i, tmpl, o.Inputs)...)
	}

	if !slices.Contains(defects, o.Defect) {
		errs = append(errs, fmt.Errorf("option Defect %q is not a defect of this package", o.Defect))
	}

	return errors.Join(errs...)
}

func grantTemplateErrors(i int, tmpl pluginsv1alpha1.GrantTemplate, inputs []pluginsv1alpha1.InputDecl) []error {
	var errs []error

	if tmpl.Name == "" {
		errs = append(errs, fmt.Errorf("option GrantTemplates[%d].Name is empty", i))
	}

	var object map[string]any
	if tmpl.Template == nil || json.Unmarshal(tmpl.Template.Raw, &object) != nil || object == nil {
		errs = append(errs, fmt.Errorf("option GrantTemplates[%d].Template is not a JSON object", i))
	}

	for _, name := range tmpl.RequiresInputs {
		if !slices.ContainsFunc(inputs, func(in pluginsv1alpha1.InputDecl) bool { return in.Name == name }) {
			errs = append(errs, fmt.Errorf(
				"option GrantTemplates[%d].RequiresInputs names %q, which Inputs does not declare",
				i,
				name,
			))
		}
	}

	return errs
}
