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

const (
	// cnpgCRDDir holds the CloudNativePG CRDs the envtest suites need. This
	// operator writes two of them, Cluster and ScheduledBackup. The third,
	// Backup, is vendored because a ScheduledBackup produces Backup objects
	// and a later controller reads them.
	//
	// The published Go module github.com/cloudnative-pg/api carries the types
	// alone, so the schemas are vendored instead of resolved from the module
	// cache the way the ECK ones are.
	cnpgCRDDir = "internal/testenv/crds/cnpg"
	// barmanCRDDir holds the ObjectStore CRD of the Barman Cloud plugin.
	barmanCRDDir = "internal/testenv/crds/barmancloud"
)

// CNPGCRDPath returns the directory of the vendored CloudNativePG CRDs, for
// envtest to install. The VERSION file beside them names the CloudNativePG
// release they come from.
func CNPGCRDPath() (string, error) {
	return vendoredCRDPath(cnpgCRDDir)
}

// BarmanCRDPath returns the directory of the vendored ObjectStore CRD of the
// Barman Cloud plugin, for envtest to install. The VERSION file beside it
// names the plugin release it comes from.
func BarmanCRDPath() (string, error) {
	return vendoredCRDPath(barmanCRDDir)
}
