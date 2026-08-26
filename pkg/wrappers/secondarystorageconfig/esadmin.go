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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// ElasticsearchAdmin builds the Elasticsearch admin client of the contract:
// the endpoint, the Camunda user from the credentials Secret, and the CA
// bundle from the CA Secret when the contract names one. Every consumer of a
// SecondaryStorageConfig that talks to Elasticsearch builds its client here,
// so one missing Secret reports the same reason on all of them.
//
// A state that the user must correct comes back as a
// *conditions.PreCheckFailure. ReasonInvalidReference means that the
// contract has no elasticsearch block, or that the CA Secret holds no usable
// certificate. ReasonMissingSecret means that a Secret or a key is absent.
// An error is a transient read of a Secret. A caller retries an error and
// reports a failure.
func ElasticsearchAdmin(
	ctx context.Context,
	reader client.Reader,
	storage *v1.SecondaryStorageConfig,
) (*esadmin.Client, *conditions.PreCheckFailure, error) {
	es := storage.Spec.Elasticsearch
	if es == nil {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"SecondaryStorageConfig %s/%s has no elasticsearch block",
				storage.Namespace, storage.Name,
			),
		}, nil
	}

	creds := es.CredentialsSecretRef
	secret, message, err := secretref.Get(
		ctx,
		reader,
		types.NamespacedName{Namespace: storage.Namespace, Name: creds.Name},
		creds.UsernameKey,
		creds.PasswordKey,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the Elasticsearch credentials: %w", err)
	}
	if message != "" {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: message}, nil
	}

	var ca []byte
	if es.CASecretRef != nil {
		caSecret, message, err := secretref.Get(
			ctx,
			reader,
			types.NamespacedName{Namespace: storage.Namespace, Name: es.CASecretRef.Name},
			es.CASecretRef.Key,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("reading the Elasticsearch CA: %w", err)
		}
		if message != "" {
			return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: message}, nil
		}
		ca = caSecret.Data[es.CASecretRef.Key]
	}

	admin, err := esadmin.New(
		es.Endpoint,
		string(secret.Data[creds.UsernameKey]),
		string(secret.Data[creds.PasswordKey]),
		ca,
	)
	if err != nil {
		// The constructor fails only on the CA bundle. The Secret exists and
		// the key is present, so the data itself is unusable. That is a
		// state the user corrects, not one a retry fixes.
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"Secret %s/%s key %q of SecondaryStorageConfig %s/%s is not a usable CA bundle: %v",
				storage.Namespace, es.CASecretRef.Name, es.CASecretRef.Key,
				storage.Namespace, storage.Name, err,
			),
		}, nil
	}

	return admin, nil, nil
}
