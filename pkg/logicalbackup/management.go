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

package logicalbackup

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// SiblingInProgress reports the name of a non-terminal backup of the other
// backup kind for the cluster, or the empty string when none runs. Each
// backup controller checks its own kind itself; the manager wires this seam
// between the two kinds, so backups of one cluster run one at a time across
// kinds. Both controllers use this one signature, so the wiring needs no
// adapters.
type SiblingInProgress func(ctx context.Context, cluster types.NamespacedName) (string, error)

// ManagementClient builds the management API client of cluster from the
// binding it publishes, resolving the credentials the binding names. A state
// the user must see comes back as a *conditions.PreCheckFailure:
// ReasonProgressing when the cluster has not published its binding,
// ReasonMissingSecret when the binding names basic auth without a resolvable
// credentials Secret, and ReasonInvalidReference when the binding is
// unusable, for example a Camunda version the client does not support. An
// error is transient. Both backup controllers build their client through
// this one constructor, so the same broken binding reports the same reason
// on either kind.
func ManagementClient(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (*camundaadmin.Client, *conditions.PreCheckFailure, error) {
	binding := cluster.Status.Management
	if binding == nil || binding.Endpoint == "" {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonProgressing,
			Message: fmt.Sprintf(
				"CamundaCluster %s/%s has not published its management binding yet",
				cluster.Namespace, cluster.Name,
			),
		}, nil
	}

	auth := camundaadmin.Auth{}
	if binding.Auth.Method == v1.ManagementAuthMethodBasic {
		ref := binding.Auth.CredentialsSecretRef
		if ref == nil {
			return nil, &conditions.PreCheckFailure{
				Reason: v1.ReasonMissingSecret,
				Message: fmt.Sprintf(
					"CamundaCluster %s/%s publishes basic auth for its management port but names no credentials Secret",
					cluster.Namespace, cluster.Name,
				),
			}, nil
		}

		secret, message, err := secretref.Get(
			ctx,
			reader,
			types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
			ref.UsernameKey,
			ref.PasswordKey,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("reading the management credentials: %w", err)
		}
		if message != "" {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonMissingSecret,
				Message: message,
			}, nil
		}

		auth.Username = string(secret.Data[ref.UsernameKey])
		auth.Password = string(secret.Data[ref.PasswordKey])
	}

	admin, err := camundaadmin.New(camundaadmin.Binding{
		Endpoint: binding.Endpoint,
		Version:  binding.Version,
		Auth:     auth,
	})
	if err != nil {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"the management binding of CamundaCluster %s/%s is unusable: %v",
				cluster.Namespace, cluster.Name, err,
			),
		}, nil
	}

	return admin, nil, nil
}
