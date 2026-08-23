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

package secondarystorageconfig

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The annotations of a claimed contract carry the exact identity of the
// CamundaCluster that holds it. Every decision about the holder reads them.
const (
	// ClaimHolderAnnotation names the CamundaCluster that holds the contract,
	// as "<namespace>/<name>".
	ClaimHolderAnnotation = "camunda.io/claim-holder"
	// ClaimHolderUIDAnnotation carries the UID of that CamundaCluster. It
	// tells the holder apart from a later cluster with the same name.
	ClaimHolderUIDAnnotation = "camunda.io/claim-holder-uid"
)

// Holder is the CamundaCluster that holds a contract.
type Holder struct {
	// Cluster is the namespace and name of the holder.
	Cluster types.NamespacedName
	// UID is the UID of the holder.
	UID types.UID
}

// HolderOf returns the holder that the annotations of contract name, and
// false when the contract is unclaimed or the annotations are incomplete.
func HolderOf(contract *v1.SecondaryStorageConfig) (Holder, bool) {
	namespace, name, ok := strings.Cut(contract.Annotations[ClaimHolderAnnotation], "/")
	uid := contract.Annotations[ClaimHolderUIDAnnotation]
	if !ok || namespace == "" || name == "" || uid == "" {
		return Holder{}, false
	}

	return Holder{Cluster: types.NamespacedName{Namespace: namespace, Name: name}, UID: types.UID(uid)}, true
}

// Claim writes holder onto contract as its claim. The patch is conditioned
// on the resourceVersion of contract as the caller read it, so two clusters
// that claim one unclaimed contract at once cannot both win: the later patch
// fails with a conflict, and its caller reads the winner on the next
// reconcile. The error of a failed patch is the API error, wrapped.
func Claim(
	ctx context.Context,
	writer client.Writer,
	contract *v1.SecondaryStorageConfig,
	holder Holder,
) error {
	claimed := contract.DeepCopy()
	if claimed.Annotations == nil {
		claimed.Annotations = map[string]string{}
	}
	claimed.Annotations[ClaimHolderAnnotation] = holder.Cluster.String()
	claimed.Annotations[ClaimHolderUIDAnnotation] = string(holder.UID)

	patch := client.MergeFromWithOptions(contract, client.MergeFromWithOptimisticLock{})
	if err := writer.Patch(ctx, claimed, patch); err != nil {
		return fmt.Errorf(
			"claiming SecondaryStorageConfig %s for CamundaCluster %s: %w",
			client.ObjectKeyFromObject(contract), holder.Cluster, err,
		)
	}

	return nil
}
