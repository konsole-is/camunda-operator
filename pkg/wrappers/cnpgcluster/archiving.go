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

package cnpgcluster

import (
	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContinuousArchiving returns what CloudNativePG reports about the write-ahead
// log uploads of the cluster, or nil when it reports nothing. A cluster that
// archives nowhere carries no condition, and neither does one that has not
// uploaded a segment yet. A False condition names one failed upload, not a
// stopped archive. LastTransitionTime is when the uploads stopped arriving.
func ContinuousArchiving(cluster *cnpgv1.Cluster) *metav1.Condition {
	return meta.FindStatusCondition(
		cluster.Status.Conditions, string(cnpgv1.ConditionContinuousArchiving),
	)
}
