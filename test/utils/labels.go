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

package utils

import (
	"fmt"
	"slices"
	"strings"
)

// The Ginkgo label of each top-level container of the e2e suite. A label
// filter selects flows by these names, and every container carries one.
const (
	LabelManager              = "manager"
	LabelCamundaCluster       = "camundacluster"
	LabelCamundaClusterRDBMS  = "camundacluster-rdbms"
	LabelCamundaClusterOIDC   = "camundacluster-oidc"
	LabelCamundaOptimize      = "camundaoptimize"
	LabelElasticsearchCluster = "elasticsearchcluster"
	LabelDatabase             = "database"
	LabelDatabaseServer       = "databaseserver"
	LabelManagementKeycloak   = "management-keycloak"
	LabelManagementOIDC       = "management-oidc"
	LabelExample              = "example"
)

// AllLabels is every label above. A name outside it names no flow, and both
// the label list of a matrix entry and the e2e workflow are read against it.
var AllLabels = []string{
	LabelManager,
	LabelCamundaCluster,
	LabelCamundaClusterRDBMS,
	LabelCamundaClusterOIDC,
	LabelCamundaOptimize,
	LabelElasticsearchCluster,
	LabelDatabase,
	LabelDatabaseServer,
	LabelManagementKeycloak,
	LabelManagementOIDC,
	LabelExample,
}

// LabelFilter turns a comma-separated list of Ginkgo labels into a label
// filter. A plain entry selects the specs of its label, a "!" prefix excludes
// them, and an empty list selects every spec. An exclusion applies on top of
// the selection: "a,!b" runs the specs of a that are not also b. An entry
// outside known is an error, so a misspelled label cannot skip a flow in
// silence.
func LabelFilter(list string, known []string) (string, error) {
	var selected, excluded []string
	for entry := range strings.SplitSeq(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		name := strings.TrimPrefix(entry, "!")
		if !slices.Contains(known, name) {
			return "", fmt.Errorf("unknown label %q, the labels are %s", name, strings.Join(known, ", "))
		}

		if name == entry {
			selected = append(selected, name)
		} else {
			excluded = append(excluded, "!"+name)
		}
	}

	var terms []string
	if len(selected) > 0 {
		terms = append(terms, "("+strings.Join(selected, " || ")+")")
	}
	terms = append(terms, excluded...)

	return strings.Join(terms, " && "), nil
}
