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

// grant is one resource of one API group, with the verbs that the app needs.
type grant struct {
	group    string
	resource string
	verbs    []string
}

var (
	readVerbs  = []string{"get", "list", "watch"}
	writeVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete"}
)

// appRules is the RBAC list of the app in contract section 6.2, without the
// read of each operator kind. checkClusterRole adds that read from the CRD
// bases, so a new kind without a read rule fails helm-verify.
var appRules = []grant{
	{"fleet.camunda.io", "camundaclusterclaims", writeVerbs},
	{"fleet.camunda.io", "camundamanagementclusterclaims", writeVerbs},
	{"fleet.camunda.io", "environments", writeVerbs},
	{"core.camunda.io", "camundaclusterpresets", writeVerbs},
	{"core.camunda.io", "elasticsearchclusterpresets", writeVerbs},
	{"core.camunda.io", "databaseserverpresets", writeVerbs},
	{"core.camunda.io", "camundareleases", writeVerbs},
	{"core.camunda.io", "camundaplatformconfigs", writeVerbs},
	{"core.camunda.io", "objectstorageconfigs", writeVerbs},
	{"core.camunda.io", "secondarystorageconfigs", writeVerbs},
	{"core.camunda.io", "databaseserverconfigs", writeVerbs},
	{"core.camunda.io", "databaseconfigs", writeVerbs},
	{"core.camunda.io", "managementauthconfigs", writeVerbs},
	{"core.camunda.io", "logicalbackupelasticsearches", writeVerbs},
	{"core.camunda.io", "logicalbackuprdbmses", writeVerbs},
	{"core.camunda.io", "logicalrestoreelasticsearches", writeVerbs},
	{"core.camunda.io", "logicalrestorerdbmses", writeVerbs},
	{"core.camunda.io", "pointintimerestores", writeVerbs},
	{"core.camunda.io", "backupschedules", writeVerbs},
	{"", "namespaces", []string{"get", "list", "watch", "create", "patch"}},
	{"", "secrets", []string{"get", "list", "watch", "create", "update", "patch"}},
	{"external-secrets.io", "externalsecrets", readVerbs},
	{"plugins.camunda.io", "pluginsections", readVerbs},
	{"plugins.camunda.io", "pluginregistrations", readVerbs},
	{"plugins.camunda.io", "pluginrequests", readVerbs},
	{"plugins.camunda.io", "grants", readVerbs},
	{"", "pods", readVerbs},
	{"", "pods/log", []string{"get"}},
	{"", "events", readVerbs},
	{"events.k8s.io", "events", readVerbs},
	{"", "services", readVerbs},
	{"", "persistentvolumeclaims", readVerbs},
	{"metrics.k8s.io", "pods", []string{"get", "list"}},
	{"gateway.networking.k8s.io", "httproutes", readVerbs},
	{"gateway.networking.k8s.io", "grpcroutes", readVerbs},
}
