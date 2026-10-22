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

package camundamanagementclusterclaim

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundamanagementcluster"
)

func TestManagementClusterBuildsTheFiveURLsFromTheHost(t *testing.T) {
	plane := previewManagementCluster(t, fullInput(t))

	assert.Equal(t, "https://plane.example.com/auth", plane.Spec.IdentityProvider.Keycloak.ExternalURL)
	assert.Equal(t, "https://plane.example.com/identity", plane.Spec.Identity.ExternalURL)
	assert.Equal(t, "https://plane.example.com/console", plane.Spec.Console.ExternalURL)
	assert.Equal(t, "https://plane.example.com/modeler", plane.Spec.WebModeler.ExternalURL)
	assert.Equal(t, "https://plane.example.com/modeler-ws", plane.Spec.WebModeler.WebsocketsExternalURL)
}

func TestManagementClusterCarriesTheDatabaseRefsFromTheOutputs(t *testing.T) {
	plane := previewManagementCluster(t, fullInput(t))

	assert.Equal(t, "my-plane-keycloak", plane.Spec.IdentityProvider.Keycloak.DatabaseConfigRef)
	assert.Equal(t, "my-plane-identity", plane.Spec.Identity.DatabaseConfigRef)
	assert.Equal(t, "my-plane-webmodeler", plane.Spec.WebModeler.DatabaseConfigRef)
}

func TestManagementClusterPassesTheURLsOfTheClaimWithoutAccess(t *testing.T) {
	plane := previewManagementCluster(t, withoutAccess(fullInput(t)))

	assert.Equal(t, "https://login.example.com/auth", plane.Spec.IdentityProvider.Keycloak.ExternalURL)
	assert.Equal(t, "https://plane.example.com/identity", plane.Spec.Identity.ExternalURL)
	assert.Equal(t, "https://console.example.com", plane.Spec.Console.ExternalURL)
	assert.Equal(t, "https://modeler.example.com", plane.Spec.WebModeler.ExternalURL)
	assert.Equal(t, "https://modeler-ws.example.com", plane.Spec.WebModeler.WebsocketsExternalURL)
}

func TestManagementClusterPassesTheVersionsThrough(t *testing.T) {
	plane := previewManagementCluster(t, fullInput(t))

	assert.Equal(t, "8.9.10", plane.Spec.Identity.Version)
	assert.Equal(t, "26.3.1", plane.Spec.IdentityProvider.Keycloak.Version)
	assert.Equal(t, "8.9.122", plane.Spec.Console.Version)
	assert.Equal(t, "8.9.11", plane.Spec.WebModeler.Version)
}

func TestManagementClusterSelectsTheNamespacesOfTheEnvironments(t *testing.T) {
	plane := previewManagementCluster(t, fullInput(t))

	want := &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key:      fleetv1alpha1.LabelEnvironment,
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{"dev", "test"},
	}}}
	assert.Equal(t, want, plane.Spec.NamespaceSelector)
}

func TestManagementClusterCarriesTheClaimFields(t *testing.T) {
	in := fullInput(t)
	in.Claim.Spec.Suspend = true
	in.Claim.Spec.ManagementAuthConfigName = "team-auth"
	in.Claim.Spec.ClusterSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}}

	plane := previewManagementCluster(t, in)

	assert.Equal(t, "my-plane", plane.Name)
	assert.Equal(t, "my-plane-ns", plane.Namespace)
	assert.Equal(t, map[string]string{fleetv1alpha1.LabelClaim: "my-plane"}, plane.Labels)
	assert.Equal(t, "default", plane.Spec.PlatformConfigRef)
	assert.True(t, plane.Spec.Suspend)
	assert.Equal(t, "team-auth", plane.Spec.ManagementAuthConfigName)
	assert.Equal(t, in.Claim.Spec.ClusterSelector, plane.Spec.ClusterSelector)
	assert.Equal(t, ptr.To[int32](2), plane.Spec.IdentityProvider.Keycloak.Replicas)
	assert.Equal(t, "admin", plane.Spec.Identity.Admin.Username)
	assert.Equal(t, "smtp.example.com", plane.Spec.WebModeler.Mail.SMTPHost)
}

func TestManagementClusterFollowsTheIdentityMode(t *testing.T) {
	in := fullInput(t)
	in.Claim.Spec.Management.IdentityProvider = fleetv1alpha1.ManagementIdentityProvider{OIDC: &v1.ManagementOIDCSpec{}}

	plane := previewManagementCluster(t, in)

	assert.NotNil(t, plane.Spec.IdentityProvider.OIDC)
	assert.Nil(t, plane.Spec.IdentityProvider.Keycloak)
	assert.Nil(t, plane.Spec.IdentityProvider.ExternalKeycloak)
}

func TestManagementClusterLeavesUnsetComponentsOut(t *testing.T) {
	in := fullInput(t)
	in.Claim.Spec.Management.Console = nil
	in.Claim.Spec.Management.WebModeler = nil

	plane := previewManagementCluster(t, in)

	assert.Nil(t, plane.Spec.Console)
	assert.Nil(t, plane.Spec.WebModeler)
	assert.Nil(t, plane.Spec.ClusterSelector)
}

func TestManagementClusterKeepsTheLiveValueWhenAnOutputIsMissing(t *testing.T) {
	in := fullInput(t)
	notReady(in, databaseSection)
	in.Live = &v1.CamundaManagementCluster{Spec: v1.CamundaManagementClusterSpec{
		IdentityProvider: v1.IdentityProviderSpec{Keycloak: &v1.ManagedKeycloakSpec{DatabaseConfigRef: "old-keycloak"}},
		Identity:         v1.IdentitySpec{DatabaseConfigRef: "old-identity"},
	}}

	plane := previewManagementCluster(t, in)

	assert.Equal(t, "old-keycloak", plane.Spec.IdentityProvider.Keycloak.DatabaseConfigRef)
	assert.Equal(t, "old-identity", plane.Spec.Identity.DatabaseConfigRef)
	assert.Empty(t, plane.Spec.WebModeler.DatabaseConfigRef, "the live plane has no Web Modeler")
}

// A component that the claim turns on after creation has no value from
// the outputs yet, and the API server refuses a plane without it.
func TestManagementClusterWaitsWhileARequiredFieldHasNoValue(t *testing.T) {
	tests := []struct {
		name    string
		missing string
		live    *v1.CamundaManagementCluster
		want    string
	}{
		{name: "every output is there"},
		{
			name:    "the database outputs are missing before the plane exists",
			missing: databaseSection,
			want:    "waiting for the outputs of management-database",
		},
		{
			name:    "web modeler is new and its database output is missing",
			missing: databaseSection,
			live: &v1.CamundaManagementCluster{Spec: v1.CamundaManagementClusterSpec{
				IdentityProvider: v1.IdentityProviderSpec{
					Keycloak: &v1.ManagedKeycloakSpec{DatabaseConfigRef: "my-plane-keycloak"},
				},
				Identity: v1.IdentitySpec{DatabaseConfigRef: "my-plane-identity"},
			}},
			want: "waiting for the outputs of management-database",
		},
		{
			name:    "the live plane keeps every value",
			missing: databaseSection,
			live: &v1.CamundaManagementCluster{Spec: v1.CamundaManagementClusterSpec{
				IdentityProvider: v1.IdentityProviderSpec{
					Keycloak: &v1.ManagedKeycloakSpec{DatabaseConfigRef: "my-plane-keycloak"},
				},
				Identity:   v1.IdentitySpec{DatabaseConfigRef: "my-plane-identity"},
				WebModeler: &v1.WebModelerSpec{DatabaseConfigRef: "my-plane-webmodeler"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := fullInput(t)
			in.Live = tt.live
			if tt.missing != "" {
				notReady(in, tt.missing)
			}

			_, waiting, err := managementCluster(in)

			require.NoError(t, err)
			assert.Equal(t, tt.want, waiting)
		})
	}
}

// Web Modeler turned on after creation has no database until the output
// comes, and the API server refuses a plane without it.
func TestManagementClusterComponentHoldsTheApplyWhileADatabaseIsMissing(t *testing.T) {
	in := fullInput(t)
	in.Claim.UID = "claim-uid"
	notReady(in, databaseSection)
	in.Claim.Status.Conditions = []metav1.Condition{{
		Type:   fleetv1alpha1.ConditionPlaneRequestsReady,
		Status: metav1.ConditionTrue,
		Reason: "Healthy",
	}}
	in.Live = &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      in.Claim.Name,
			Namespace: in.Claim.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: fleetv1alpha1.GroupVersion.String(),
				Kind:       in.Claim.GetKind(),
				Name:       in.Claim.Name,
				UID:        in.Claim.UID,
				Controller: new(true),
			}},
		},
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{
				Keycloak: &v1.ManagedKeycloakSpec{DatabaseConfigRef: "my-plane-keycloak"},
			},
			Identity: v1.IdentitySpec{DatabaseConfigRef: "my-plane-identity"},
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, fleetv1alpha1.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	var writes int
	count := func(obj client.Object) {
		if _, ok := obj.(*v1.CamundaManagementCluster); ok {
			writes++
		}
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme)).
		WithObjects(in.Live.DeepCopy()).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, w client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				count(obj)
				return w.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, w client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				count(obj)
				return w.Update(ctx, obj, opts...)
			},
			Patch: func(
				ctx context.Context, w client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption,
			) error {
				count(obj)
				return w.Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()

	comps, err := Build(in)
	require.NoError(t, err)
	require.NoError(t, comps.ManagementCluster.Reconcile(context.Background(), component.ReconcileContext{
		Client:        c,
		APIReader:     c,
		Scheme:        scheme,
		EventRecorder: events.NewFakeRecorder(100),
		Owner:         in.Claim,
	}))

	condition := comps.ManagementCluster.GetCondition(in.Claim)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, "Blocked", condition.Reason)
	assert.Contains(t, condition.Message, "waiting for the outputs of management-database")
	assert.Zero(t, writes, "the plane is not written")
}

// The apply of the plane sends only the fields that the claim and the
// outputs set.
func TestManagementClusterApplySendsOnlyTheClaimAndOutputFields(t *testing.T) {
	var sent []byte
	recorder := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(
			_ context.Context, _ client.WithWatch, obj client.Object,
			patch client.Patch, _ ...client.PatchOption,
		) error {
			data, err := patch.Data(obj)
			sent = data

			return err
		},
	}).Build()
	in := fullInput(t)
	in.Claim.Spec.Management = fleetv1alpha1.ManagementOverrides{
		IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{OIDC: &v1.ManagementOIDCSpec{}},
		Identity: fleetv1alpha1.IdentityOverrides{
			Admin: v1.IdentityAdminSpec{ClaimName: "groups", ClaimValue: "admins"},
		},
	}

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(
		t,
		camundamanagementcluster.NewApplyClient(recorder).
			Patch(t.Context(), previewManagementCluster(t, in), client.Apply),
	)

	var applied struct {
		Spec map[string]json.RawMessage `json:"spec"`
	}
	require.NoError(t, json.Unmarshal(sent, &applied))
	assert.Equal(
		t,
		[]string{"identity", "identityProvider", "namespaceSelector", "platformConfigRef"},
		slices.Sorted(maps.Keys(applied.Spec)),
	)
	assert.JSONEq(
		t,
		`{"version":"8.9.10","externalUrl":"https://plane.example.com/identity",`+
			`"databaseConfigRef":"my-plane-identity","admin":{"claimName":"groups","claimValue":"admins"}}`,
		string(applied.Spec["identity"]),
	)
	assert.JSONEq(t, `{"oidc":{}}`, string(applied.Spec["identityProvider"]))
}
