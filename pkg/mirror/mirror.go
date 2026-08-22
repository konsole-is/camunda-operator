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

// Package mirror is the reader side of the mirrored Secrets of a
// CamundaCluster. A pod mounts a Secret only from its own namespace. The
// CamundaCluster controller therefore copies every referenced Secret that
// lives elsewhere into the namespace of the cluster, one copy for each
// purpose. This package holds the rule that reads those copies back. A caller
// reads a Secret of the cluster namespace where it is. A caller reads a Secret
// of any other namespace through its purpose-named copy.
//
// LogicalBackupRDBMS and LogicalRestoreRDBMS both mount these Secrets into a
// Job, and both apply this rule. The writer side of the copies is
// pkg/components/camundacluster, and the CamundaCluster controller decides
// what to copy through Needed, so writer and reader share one rule.
package mirror

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// LocalSecretName resolves where a referenced Secret is reachable from the
// namespace of the cluster. A source in that namespace keeps its own name. A
// source in any other namespace resolves to its purpose-named copy.
func LocalSecretName(
	cluster *v1.CamundaCluster,
	namespace, name string,
	purpose camundacluster.MirrorPurpose,
) string {
	if !Needed(cluster, namespace) {
		return name
	}

	return camundacluster.MirroredSecretName(cluster, purpose)
}

// Needed reports whether a Secret in namespace is copied into the namespace
// of the cluster before a pod of the cluster can mount it.
func Needed(cluster *v1.CamundaCluster, namespace string) bool {
	return namespace != cluster.Namespace
}

// CheckLocalSecret checks that the Secret at namespace/name carries keys. A
// missing Secret or a missing key becomes a pre-check failure with the given
// reason. purpose names the credentials in the message, and the message also
// names the CamundaCluster controller as the keeper of a copy. Pass an
// uncached reader, because a caller that admits a Job must not read stale
// Secret data.
func CheckLocalSecret(
	ctx context.Context,
	reader client.Reader,
	namespace, name, reason, purpose string,
	keys ...string,
) (*conditions.PreCheckFailure, error) {
	message, err := secretref.CheckKeys(
		ctx, reader, types.NamespacedName{Namespace: namespace, Name: name}, keys...,
	)
	if err != nil {
		return nil, fmt.Errorf("checking the %s credentials: %w", purpose, err)
	}
	if message == "" {
		return nil, nil
	}

	return &conditions.PreCheckFailure{
		Reason: reason,
		Message: fmt.Sprintf(
			"%s. The CamundaCluster controller keeps the local copy of %s credentials that live "+
				"outside the cluster namespace",
			message, purpose,
		),
	}, nil
}
