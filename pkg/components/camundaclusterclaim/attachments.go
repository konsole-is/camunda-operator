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
	"encoding/json"
	"fmt"
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/backupschedule"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundaoptimize"
)

// attachmentsComponentName is the name of the attachments component. The
// field manager of each attachment is
// CamundaClusterClaim/attachments/<claim UID>.
const attachmentsComponentName = "attachments"

// attachmentsComponent builds the component of the CamundaOptimize and the
// BackupSchedule of the claim. It waits for ClusterReady. An attachment
// that the claim does not set is deleted when the claim owns it.
func attachmentsComponent(in Input) (*component.Component, error) {
	var attachments fleetv1alpha1.ClusterAttachments
	if in.Claim.Spec.Attachments != nil {
		attachments = *in.Claim.Spec.Attachments
	}

	desired, waiting, err := optimize(in)
	if err != nil {
		return nil, err
	}
	optimizeResource, err := camundaoptimize.NewBuilder(desired).
		WithCustomConvergeStatus(camundaoptimize.VerbatimConvergingStatusHandler).
		WithGuard(waitFor[v1.CamundaOptimize](waiting)).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building the CamundaOptimize %s: %w", desired.Name, err)
	}

	schedule := backupSchedule(in)
	scheduleResource, err := backupschedule.NewBuilder(schedule).Build()
	if err != nil {
		return nil, fmt.Errorf("building the BackupSchedule %s: %w", schedule.Name, err)
	}

	// A blocked guard stops every resource after it, so the schedule goes
	// first: it waits for no output.
	return component.NewComponentBuilder().
		WithName(attachmentsComponentName).
		WithConditionType(component.ConditionType(fleetv1alpha1.ConditionAttachmentsReady)).
		WithPrerequisite(component.DependsOn(component.ConditionType(fleetv1alpha1.ConditionClusterReady))).
		WithResource(
			scheduleResource,
			component.DeleteWhen(attachments.BackupSchedule == nil),
			component.DeleteOnlyIfOwned(),
			component.BlockOnForeignController(),
		).
		WithResource(
			optimizeResource,
			component.DeleteWhen(attachments.Optimize == nil),
			component.DeleteOnlyIfOwned(),
			component.BlockOnForeignController(),
		).
		Build()
}

// optimize returns the CamundaOptimize of the claim: the cluster reference,
// the version of the release, the fields of spec.attachments.optimize, and
// externalUrl from the optimizeUrl output. Without spec.attachments.optimize
// it holds the name, the namespace, and the label only. waiting is empty,
// or, while the CamundaOptimize does not exist, it names the sections whose
// output is missing.
func optimize(in Input) (desired *v1.CamundaOptimize, waiting string, err error) {
	desired = &v1.CamundaOptimize{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaOptimize"},
		ObjectMeta: attachmentMeta(in.Claim, OptimizeName(in.Claim.Name)),
	}
	if in.Claim.Spec.Attachments == nil || in.Claim.Spec.Attachments.Optimize == nil {
		return desired, "", nil
	}

	attachment := in.Claim.Spec.Attachments.Optimize.DeepCopy()
	desired.Spec = v1.CamundaOptimizeSpec{
		ClusterRef:        v1.ClusterRef{Name: in.Claim.Name},
		ManagementAuthRef: attachment.ManagementAuthConfigRef,
		Version:           in.ReleaseVersion,
		ExternalURL:       attachment.ExternalURL,
		Webapp:            attachment.Webapp,
		Importer:          attachment.Importer,
		IndexReplicas:     attachment.IndexReplicas,
		Monitoring:        attachment.Monitoring,
	}

	fields, err := claimpolicy.SlotFields(
		pluginsv1alpha1.ClaimKindCamundaClusterClaim, in.Sections, in.Claim.Spec.Plugins, in.Requests,
	)
	if err != nil {
		return nil, "", err
	}

	var missing []string
	for _, f := range fields {
		if f.Kind != claimpolicy.KindCamundaOptimize || f.Path != claimpolicy.PathExternalURL {
			continue
		}

		if f.Value == nil {
			missing = append(missing, f.Section)
			if in.LiveOptimize != nil {
				desired.Spec.ExternalURL = in.LiveOptimize.Spec.ExternalURL
			}
			continue
		}
		if err := json.Unmarshal(f.Value.Raw, &desired.Spec.ExternalURL); err != nil {
			return nil, "", fmt.Errorf("setting externalUrl from the outputs of section %q: %w", f.Section, err)
		}
	}
	slices.Sort(missing)

	if in.LiveOptimize == nil {
		waiting = claimpolicy.WaitingReason(slices.Compact(missing))
	}

	return desired, waiting, nil
}

// backupSchedule returns the BackupSchedule of the claim: the cluster
// reference and the fields of spec.attachments.backupSchedule. Without
// spec.attachments.backupSchedule it holds the name, the namespace, and the
// label only.
func backupSchedule(in Input) *v1.BackupSchedule {
	schedule := &v1.BackupSchedule{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "BackupSchedule"},
		ObjectMeta: attachmentMeta(in.Claim, BackupScheduleName(in.Claim.Name)),
	}
	if in.Claim.Spec.Attachments == nil || in.Claim.Spec.Attachments.BackupSchedule == nil {
		return schedule
	}

	attachment := in.Claim.Spec.Attachments.BackupSchedule.DeepCopy()
	schedule.Spec = v1.BackupScheduleSpec{
		ClusterRef: v1.ClusterRef{Name: in.Claim.Name},
		Schedule:   attachment.Schedule,
		Retained:   attachment.Retained,
	}

	return schedule
}

func attachmentMeta(claim *fleetv1alpha1.CamundaClusterClaim, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: claim.Namespace,
		Labels:    map[string]string{fleetv1alpha1.LabelClaim: claim.Name},
	}
}
