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

package camundamanagementcluster

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// realmClaimHolder is a management cluster that claims a realm in these tests.
func realmClaimHolder(namespace, name string, uid types.UID) *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid},
	}
}

func TestRealmClaimLeaseName(t *testing.T) {
	identity := RealmIdentity(v1.KeycloakRealmTarget{
		URL: "https://keycloak.example.com/auth", Realm: "camunda-platform",
	})

	name := RealmClaimLeaseName(identity)

	assert.True(t, strings.HasPrefix(name, "camunda-realm-"))
	assert.Empty(t, validation.IsDNS1123Subdomain(name))
	assert.Equal(t, name, RealmClaimLeaseName(identity))
	assert.NotEqual(t, name, RealmClaimLeaseName(RealmIdentity(v1.KeycloakRealmTarget{
		URL: "https://keycloak.example.com/auth", Realm: "other",
	})))
	assert.NotEqual(t, name, RealmClaimLeaseName(RealmIdentity(v1.KeycloakRealmTarget{
		URL: "https://other.example.com/auth", Realm: "camunda-platform",
	})))
}

func TestNewRealmClaimLease(t *testing.T) {
	target := v1.KeycloakRealmTarget{URL: "https://keycloak.example.com/auth/", Realm: "camunda-platform"}
	holder := realmClaimHolder("apps", "plane", "uid-1")

	lease := NewRealmClaimLease("camunda-system", target, holder)

	assert.Equal(t, RealmClaimLeaseName(RealmIdentity(target)), lease.Name)
	assert.Equal(t, "camunda-system", lease.Namespace)
	assert.Equal(t, RealmClaimLeaseLabels("plane"), lease.Labels)
	assert.Equal(t, labels.ManagedBy, lease.Labels[labels.ManagedByKey])
	assert.Equal(t, ComponentRealmClaim, lease.Labels[labels.ComponentKey])
	assert.Equal(
		t, "https://keycloak.example.com/auth/realms/camunda-platform",
		lease.Annotations[RealmClaimRealmAnnotation],
	)
	require.NotNil(t, lease.Spec.HolderIdentity)
	assert.Equal(t, "apps/plane", *lease.Spec.HolderIdentity)
	require.NotNil(t, lease.Spec.AcquireTime)

	recorded, ours := RealmClaimHolderOf(lease)
	assert.True(t, ours)
	assert.Equal(
		t, RealmClaimHolder{
			NamespacedName: types.NamespacedName{Namespace: "apps", Name: "plane"},
			UID:            "uid-1",
		}, recorded,
	)
}

func TestNewRealmClaimLeaseBoundsTheHolderIdentity(t *testing.T) {
	long := strings.Repeat("n", 200)
	holder := realmClaimHolder("apps", long, "uid-1")

	lease := NewRealmClaimLease("camunda-system", v1.KeycloakRealmTarget{URL: "https://k", Realm: "r"}, holder)

	require.NotNil(t, lease.Spec.HolderIdentity)
	assert.LessOrEqual(t, len(*lease.Spec.HolderIdentity), maxHolderIdentityLength)
	assert.Equal(t, long, lease.Annotations[RealmClaimHolderNameAnnotation])
}

func TestRealmClaimHolderOfRefusesAForeignLease(t *testing.T) {
	cases := map[string]map[string]string{
		"no annotations": nil,
		"no uid": {
			RealmClaimHolderNamespaceAnnotation: "apps",
			RealmClaimHolderNameAnnotation:      "plane",
		},
		"no name": {
			RealmClaimHolderNamespaceAnnotation: "apps",
			RealmClaimHolderUIDAnnotation:       "uid-1",
		},
		"no namespace": {
			RealmClaimHolderNameAnnotation: "plane",
			RealmClaimHolderUIDAnnotation:  "uid-1",
		},
	}
	for name, annotations := range cases {
		t.Run(name, func(t *testing.T) {
			lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}}

			_, ours := RealmClaimHolderOf(lease)

			assert.False(t, ours)
		})
	}
}
