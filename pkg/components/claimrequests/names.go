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

package claimrequests

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// Name returns the name of the request of claim for selection s in a
// section with mode: <claim>-<section> for One, and
// <claim>-<section>-<registration> for Many. A name over 63 characters is
// cut to 54, without a trailing "-" or ".", and gets "-" and the first 8
// hex characters of the SHA-256 of the full name.
func Name(claim string, s fleetv1alpha1.PluginSelection, mode pluginsv1alpha1.Selection) string {
	name := claim + "-" + s.Section
	if mode == pluginsv1alpha1.SelectionMany {
		name += "-" + s.Registration
	}

	// The name is the value of the plugins.camunda.io/request label.
	if len(name) <= validation.LabelValueMaxLength {
		return name
	}
	sum := sha256.Sum256([]byte(name))

	// A "." or "-" before the hash would end a DNS label of the name.
	return strings.TrimRight(name[:54], "-.") + "-" + hex.EncodeToString(sum[:4])
}

// modeOf returns the selection mode of section, or "" when sections does
// not hold it.
func modeOf(sections []pluginsv1alpha1.PluginSection, section string) pluginsv1alpha1.Selection {
	if s := find(sections, section); s != nil {
		return s.Spec.Selection
	}

	return ""
}
