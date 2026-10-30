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

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
)

// slotSection is a section with a slot, the test registration that serves
// it, and the outputs that its request reports.
type slotSection struct {
	section, registration string
	slot                  pluginsv1alpha1.Slot
	outputs               map[string]string
}

// accessSection is the section of slotSections with the externalAccess
// slot.
const accessSection = "access"

// slotSections holds one section for each slot that fills a field of the
// CamundaCluster.
var slotSections = []slotSection{
	{
		section: "cloud-identity", registration: "test-identity", slot: pluginsv1alpha1.SlotWorkloadIdentity,
		outputs: map[string]string{"serviceAccountAnnotations": `{"eks.amazonaws.com/role-arn":"arn:camunda"}`},
	},
	{
		section: "connectors-permissions", registration: "test-connectors",
		slot:    pluginsv1alpha1.SlotConnectorsIdentity,
		outputs: map[string]string{"serviceAccountAnnotations": `{"eks.amazonaws.com/role-arn":"arn:connectors"}`},
	},
	{
		section: "secondary-storage", registration: "elasticsearch", slot: pluginsv1alpha1.SlotSecondaryStorage,
		outputs: map[string]string{"secondaryStorageConfig": "orders-es"},
	},
	{
		section: "backup-storage", registration: "test-bucket", slot: pluginsv1alpha1.SlotBackupStorage,
		outputs: map[string]string{"objectStorageConfig": "orders-backups"},
	},
	{
		section: "volume-encryption", registration: "test-volumes", slot: pluginsv1alpha1.SlotVolumeEncryption,
		outputs: map[string]string{"storageClassName": "orders-encrypted"},
	},
	{
		section: accessSection, registration: "test-access", slot: pluginsv1alpha1.SlotExternalAccess,
		outputs: map[string]string{
			"externalUrl": "https://orders.example.com",
			"optimizeUrl": "https://orders.example.com/optimize",
		},
	},
}

// storage is the BuiltinStorage that the controller passes.
var storage = BuiltinStorage{
	ElasticsearchRegistration:  "elasticsearch",
	ElasticsearchSuffix:        "-es",
	DatabaseServerRegistration: "database-server",
	DatabaseServerSuffix:       "-pg",
}

// fullInput is the input of a claim orders in namespace team-a that selects
// each section of slotSections. Every request reports its outputs.
func fullInput(t *testing.T) Input {
	t.Helper()

	in := Input{
		Claim: &fleetv1alpha1.CamundaClusterClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"},
			Spec: fleetv1alpha1.CamundaClusterClaimSpec{
				PresetRef: "small",
				Attachments: &fleetv1alpha1.ClusterAttachments{
					Optimize: &fleetv1alpha1.OptimizeAttachment{
						ManagementAuthConfigRef: "management",
						IndexReplicas:           ptr.To[int32](1),
					},
					BackupSchedule: &fleetv1alpha1.BackupScheduleAttachment{Schedule: "0 2 * * *"},
				},
			},
		},
		ReleaseRef:        "camunda-8-9",
		ReleaseVersion:    "8.9.9",
		PlatformConfigRef: "default",
		Storage:           storage,
	}
	for _, s := range slotSections {
		in.Claim.Spec.Plugins = append(
			in.Claim.Spec.Plugins,
			fleetv1alpha1.PluginSelection{Section: s.section, Registration: s.registration},
		)
		in.Sections = append(in.Sections, pluginsv1alpha1.PluginSection{
			ObjectMeta: metav1.ObjectMeta{Name: s.section},
			Spec:       pluginsv1alpha1.PluginSectionSpec{Slot: s.slot},
		})
		in.Registrations = append(in.Registrations, pluginsv1alpha1.PluginRegistration{
			ObjectMeta: metav1.ObjectMeta{Name: s.registration},
			Spec:       pluginsv1alpha1.PluginRegistrationSpec{Sections: []string{s.section}},
		})
		in.Requests = append(in.Requests, readyRequest(s.section, s.registration, s.outputs))
	}

	order, err := claimpolicy.Order(in.Claim.Spec.Plugins, in.Registrations)
	require.NoError(t, err)
	in.Order = order

	return in
}

// readyRequest returns the request of the claim orders in section, with
// OutputsReady=True for its generation and outputs.
func readyRequest(section, registration string, outputs map[string]string) pluginsv1alpha1.PluginRequest {
	return pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-" + section, Namespace: "team-a", Generation: 1},
		Spec:       pluginsv1alpha1.PluginRequestSpec{Section: section, Registration: registration},
		Status: pluginsv1alpha1.PluginRequestStatus{
			Conditions: []metav1.Condition{{
				Type:               pluginsv1alpha1.ConditionOutputsReady,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
			}},
			Outputs: outputs,
		},
	}
}

// previewCluster returns the CamundaCluster that the cluster component of
// in renders.
func previewCluster(t *testing.T, in Input) *v1.CamundaCluster {
	t.Helper()

	comps, err := Build(in)
	require.NoError(t, err)

	objects, err := comps.Cluster.Preview()
	require.NoError(t, err)
	require.Len(t, objects, 1)

	cluster, ok := objects[0].(*v1.CamundaCluster)
	require.True(t, ok, "unexpected %T", objects[0])

	return cluster
}

// previewAttachments returns the CamundaOptimize and the BackupSchedule that
// the attachments component of in renders, nil for each one that it does not
// hold.
func previewAttachments(t *testing.T, in Input) (optimize *v1.CamundaOptimize, schedule *v1.BackupSchedule) {
	t.Helper()

	comps, err := Build(in)
	require.NoError(t, err)

	objects, err := comps.Attachments.Preview()
	require.NoError(t, err)
	for _, obj := range objects {
		switch o := obj.(type) {
		case *v1.CamundaOptimize:
			optimize = o
		case *v1.BackupSchedule:
			schedule = o
		default:
			require.Failf(t, "unexpected object", "%T", obj)
		}
	}

	return optimize, schedule
}
