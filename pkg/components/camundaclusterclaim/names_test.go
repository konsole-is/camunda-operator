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

package camundaclusterclaim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
)

func TestServiceAccountNamesMatchTheTargetOfTheRequests(t *testing.T) {
	connectors := &pluginsv1alpha1.PluginSection{
		Spec: pluginsv1alpha1.PluginSectionSpec{Slot: pluginsv1alpha1.SlotConnectorsIdentity},
	}

	assert.Equal(t, "orders-camunda", ServiceAccountName("orders"))
	assert.Equal(t, claimpolicy.TargetServiceAccount("orders", nil), ServiceAccountName("orders"))
	assert.Equal(t, "orders-connectors", ConnectorsServiceAccountName("orders"))
	assert.Equal(t, claimpolicy.TargetServiceAccount("orders", connectors), ConnectorsServiceAccountName("orders"))
}

func TestAdditionalServiceAccounts(t *testing.T) {
	tests := []struct {
		name       string
		selections []fleetv1alpha1.PluginSelection
		want       []string
	}{
		{
			name:       "Elasticsearch",
			selections: []fleetv1alpha1.PluginSelection{{Section: "secondary-storage", Registration: "elasticsearch"}},
			want:       []string{"orders-es-es"},
		},
		{
			name: "a database server",
			selections: []fleetv1alpha1.PluginSelection{
				{Section: "secondary-storage", Registration: "database-server"},
			},
			want: []string{"orders-pg-postgres"},
		},
		{
			name: "existing secondary storage",
			selections: []fleetv1alpha1.PluginSelection{
				{Section: "secondary-storage", Registration: "existing-secondary-storage"},
			},
		},
		{name: "no selection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AdditionalServiceAccounts("orders", tt.selections, storage))
		})
	}
}

func TestDeletionPolicy(t *testing.T) {
	orphan := pluginsv1alpha1.DeletionPolicyOrphan
	tests := []struct {
		name     string
		spec     pluginsv1alpha1.DeletionPolicy
		recorded *fleetv1alpha1.ResolvedPolicy
		want     pluginsv1alpha1.DeletionPolicy
	}{
		{
			name:     "the claim value wins",
			spec:     orphan,
			recorded: &fleetv1alpha1.ResolvedPolicy{DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete},
			want:     orphan,
		},
		{
			name:     "the snapshot default applies without a claim value",
			recorded: &fleetv1alpha1.ResolvedPolicy{DeletionPolicy: orphan},
			want:     orphan,
		},
		{
			name: "Delete applies without either",
			want: pluginsv1alpha1.DeletionPolicyDelete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claim := &fleetv1alpha1.CamundaClusterClaim{
				Spec:   fleetv1alpha1.CamundaClusterClaimSpec{DeletionPolicy: tt.spec},
				Status: fleetv1alpha1.CamundaClusterClaimStatus{ResolvedPolicy: tt.recorded},
			}

			assert.Equal(t, tt.want, DeletionPolicy(claim))
		})
	}
}

func TestReferences(t *testing.T) {
	environment := &fleetv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "prod"},
		Spec: fleetv1alpha1.EnvironmentSpec{
			PlatformConfigRef: "prod-config",
			Releases:          &fleetv1alpha1.EnvironmentReleases{Default: "camunda-8-9"},
		},
	}
	tests := []struct {
		name                      string
		spec                      fleetv1alpha1.CamundaClusterClaimSpec
		environment               *fleetv1alpha1.Environment
		wantRelease, wantPlatform string
	}{
		{
			name:        "the claim values win",
			spec:        fleetv1alpha1.CamundaClusterClaimSpec{ReleaseRef: "camunda-8-8", PlatformConfigRef: "own"},
			environment: environment,
			wantRelease: "camunda-8-8", wantPlatform: "own",
		},
		{
			name:        "the defaults of the Environment apply without a claim value",
			environment: environment,
			wantRelease: "camunda-8-9", wantPlatform: "prod-config",
		},
		{name: "no Environment gives empty names"},
		{
			name: "an Environment without releases gives no release",
			environment: &fleetv1alpha1.Environment{
				Spec: fleetv1alpha1.EnvironmentSpec{PlatformConfigRef: "prod-config"},
			},
			wantPlatform: "prod-config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release, platform := References(&fleetv1alpha1.CamundaClusterClaim{Spec: tt.spec}, tt.environment)

			assert.Equal(t, tt.wantRelease, release)
			assert.Equal(t, tt.wantPlatform, platform)
		})
	}
}
