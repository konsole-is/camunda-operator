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
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundaoptimize"
)

func TestOptimizeCarriesTheClaimAndReleaseFields(t *testing.T) {
	optimize, _ := previewAttachments(t, fullInput(t))
	require.NotNil(t, optimize)

	assert.Equal(t, "orders-optimize", optimize.Name)
	assert.Equal(t, "team-a", optimize.Namespace)
	assert.Equal(t, map[string]string{fleetv1alpha1.LabelClaim: "orders"}, optimize.Labels)
	assert.Equal(t, v1.ClusterRef{Name: "orders"}, optimize.Spec.ClusterRef)
	assert.Equal(t, "management", optimize.Spec.ManagementAuthRef)
	assert.Equal(t, "8.9.9", optimize.Spec.Version)
	assert.Equal(t, ptr.To[int32](1), optimize.Spec.IndexReplicas)
	assert.Equal(t, "https://orders.example.com/optimize", optimize.Spec.ExternalURL, "from optimizeUrl")
}

func TestOptimizeTakesTheClaimURLWithoutAnAccessSelection(t *testing.T) {
	in := withoutSection(fullInput(t), "access")
	in.Claim.Spec.Attachments.Optimize.ExternalURL = "https://optimize.example.com"

	optimize, _ := previewAttachments(t, in)
	require.NotNil(t, optimize)

	assert.Equal(t, "https://optimize.example.com", optimize.Spec.ExternalURL)
}

func TestOptimizeKeepsTheLiveURLWhenTheOutputIsMissing(t *testing.T) {
	in := withoutAccessOutputs(fullInput(t))
	in.LiveOptimize = &v1.CamundaOptimize{Spec: v1.CamundaOptimizeSpec{ExternalURL: "https://old.example.com"}}
	live := in.LiveOptimize.DeepCopy()

	optimize, _ := previewAttachments(t, in)
	require.NotNil(t, optimize)

	assert.Equal(t, "https://old.example.com", optimize.Spec.ExternalURL)
	assert.Equal(t, live, in.LiveOptimize)
}

func TestOptimizeWaitsForTheOutputBeforeTheFirstApply(t *testing.T) {
	tests := []struct {
		name       string
		live       *v1.CamundaOptimize
		missing    bool
		wantStatus concepts.GuardStatus
		wantReason string
	}{
		{name: "every output is there", wantStatus: concepts.GuardStatusUnblocked},
		{
			name:       "the output is missing before the Optimize exists",
			missing:    true,
			wantStatus: concepts.GuardStatusBlocked,
			wantReason: "waiting for the outputs of access",
		},
		{
			name:       "the output is missing after the Optimize exists",
			live:       &v1.CamundaOptimize{},
			missing:    true,
			wantStatus: concepts.GuardStatusUnblocked,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := fullInput(t)
			in.LiveOptimize = tt.live
			if tt.missing {
				in = withoutAccessOutputs(in)
			}

			_, waiting, err := optimize(in)
			require.NoError(t, err)
			status, err := waitFor[v1.CamundaOptimize](waiting)(v1.CamundaOptimize{})
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, status.Status)
			assert.Equal(t, tt.wantReason, status.Reason)
		})
	}
}

func TestBackupScheduleCarriesTheClaimFields(t *testing.T) {
	_, schedule := previewAttachments(t, fullInput(t))
	require.NotNil(t, schedule)

	assert.Equal(t, "orders-backups", schedule.Name)
	assert.Equal(t, "team-a", schedule.Namespace)
	assert.Equal(t, map[string]string{fleetv1alpha1.LabelClaim: "orders"}, schedule.Labels)
	assert.Equal(t, v1.ClusterRef{Name: "orders"}, schedule.Spec.ClusterRef)
	assert.Equal(t, "0 2 * * *", schedule.Spec.Schedule)
	assert.Nil(t, schedule.Spec.Retained)
}

func TestAttachmentsLeaveOutWhatTheClaimDoesNotSet(t *testing.T) {
	tests := []struct {
		name                       string
		attachments                *fleetv1alpha1.ClusterAttachments
		wantOptimize, wantSchedule bool
	}{
		{name: "no attachments"},
		{
			name: "only Optimize",
			attachments: &fleetv1alpha1.ClusterAttachments{
				Optimize: &fleetv1alpha1.OptimizeAttachment{ManagementAuthConfigRef: "management"},
			},
			wantOptimize: true,
		},
		{
			name: "only the backup schedule",
			attachments: &fleetv1alpha1.ClusterAttachments{
				BackupSchedule: &fleetv1alpha1.BackupScheduleAttachment{Schedule: "0 2 * * *"},
			},
			wantSchedule: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := fullInput(t)
			in.Claim.Spec.Attachments = tt.attachments

			optimize, schedule := previewAttachments(t, in)

			assert.Equal(t, tt.wantOptimize, optimize != nil)
			assert.Equal(t, tt.wantSchedule, schedule != nil)
		})
	}
}

// The apply of the Optimize sends only the fields that the claim, the
// release, and the outputs set.
func TestOptimizeApplySendsOnlyTheClaimFields(t *testing.T) {
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
	optimize, _ := previewAttachments(t, fullInput(t))
	require.NotNil(t, optimize)

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, camundaoptimize.NewApplyClient(recorder).Patch(t.Context(), optimize, client.Apply))

	var applied struct {
		Spec map[string]json.RawMessage `json:"spec"`
	}
	require.NoError(t, json.Unmarshal(sent, &applied))
	assert.Equal(
		t,
		[]string{"clusterRef", "externalUrl", "indexReplicas", "managementAuthRef", "version"},
		slices.Sorted(maps.Keys(applied.Spec)),
	)
}

// withoutSection returns in without the selection, the section, and the
// request of section.
func withoutSection(in Input, section string) Input {
	in.Claim.Spec.Plugins = slices.DeleteFunc(in.Claim.Spec.Plugins, func(s fleetv1alpha1.PluginSelection) bool {
		return s.Section == section
	})
	in.Order = slices.DeleteFunc(in.Order, func(s fleetv1alpha1.PluginSelection) bool {
		return s.Section == section
	})
	in.Sections = slices.DeleteFunc(in.Sections, func(s pluginsv1alpha1.PluginSection) bool {
		return s.Name == section
	})
	in.Requests = slices.DeleteFunc(in.Requests, func(r pluginsv1alpha1.PluginRequest) bool {
		return r.Spec.Section == section
	})

	return in
}

// withoutAccessOutputs returns in with the request of the access section
// not reporting its outputs.
func withoutAccessOutputs(in Input) Input {
	access := slices.IndexFunc(in.Requests, func(r pluginsv1alpha1.PluginRequest) bool {
		return r.Spec.Section == "access"
	})
	in.Requests[access].Status.Conditions = nil

	return in
}
