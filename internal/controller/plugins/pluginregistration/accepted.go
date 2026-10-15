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

package pluginregistration

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/version"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// facts is what the controller read from the cluster for one registration.
type facts struct {
	// missingSections are the names in spec.sections that name no
	// PluginSection, in the order of spec.sections.
	missingSections []string
	// defaultPeers maps a section to the sorted names of the other
	// registrations of that section that have mode Default.
	defaultPeers map[string][]string
}

// accepted returns the Accepted condition of reg. The checks run in this
// order, and the first failure decides the reason: every section exists,
// operatorVersion is inside spec.operatorVersions, and no other registration
// of a section has mode Default when reg has it. An empty
// spec.operatorVersions holds every version.
func accepted(reg *pluginsv1alpha1.PluginRegistration, operatorVersion string, f facts) metav1.Condition {
	reason, message := check(reg, operatorVersion, f)

	status := metav1.ConditionFalse
	if reason == v1.ReasonHealthy {
		status = metav1.ConditionTrue
	}

	return metav1.Condition{
		Type:               pluginsv1alpha1.ConditionAccepted,
		Status:             status,
		Reason:             reason,
		Message:            conditions.BoundMessage(message),
		ObservedGeneration: reg.Generation,
	}
}

func check(reg *pluginsv1alpha1.PluginRegistration, operatorVersion string, f facts) (reason, message string) {
	if len(f.missingSections) > 0 {
		return pluginsv1alpha1.ReasonSectionMissing,
			"These sections do not exist: " + strings.Join(f.missingSections, ", ")
	}

	if msg := versionProblem(operatorVersion, reg.Spec.OperatorVersions); msg != "" {
		return pluginsv1alpha1.ReasonOperatorVersionUnsupported, msg
	}

	if msg := defaultConflict(reg, f.defaultPeers); msg != "" {
		return pluginsv1alpha1.ReasonDefaultConflict, msg
	}

	return v1.ReasonHealthy, "All checks passed"
}

func versionProblem(operatorVersion, rangeExpr string) string {
	if rangeExpr == "" {
		return ""
	}

	// SetupWithManager refuses an operator version that is not semver, so an
	// error here comes from the range.
	ok, err := version.Satisfies(operatorVersion, rangeExpr)
	if err != nil {
		return "operatorVersions is not valid: " + err.Error()
	}
	if !ok {
		return fmt.Sprintf("Operator version %s is outside operatorVersions %q", operatorVersion, rangeExpr)
	}

	return ""
}

func defaultConflict(reg *pluginsv1alpha1.PluginRegistration, peers map[string][]string) string {
	if reg.Spec.Mode != pluginsv1alpha1.ModeDefault {
		return ""
	}

	var sentences []string
	for _, section := range reg.Spec.Sections {
		if others := peers[section]; len(others) > 0 {
			sentences = append(sentences, fmt.Sprintf(
				"In section %s, these registrations also have mode Default: %s",
				section, strings.Join(others, ", "),
			))
		}
	}

	return strings.Join(sentences, ". ")
}
