// This module runs in the browser and on the server.
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  backupScheduleName,
  optimizeName,
  requestName,
} from "@/lib/claims/requests";
import type { ClusterClaimObject } from "./draft";

/**
 * createdObjects returns the review lines of what the create of claim
 * makes, in order: the namespace when newNamespace, the claim, the request
 * of each selection in claim order, the CamundaCluster, and the
 * CamundaOptimize and the BackupSchedule when the claim attaches them.
 */
export function createdObjects(input: {
  claim: ClusterClaimObject;
  newNamespace: boolean;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
}): string[] {
  const { claim } = input;
  const { name, namespace } = claim.metadata;
  const { optimize, backupSchedule } = claim.spec.attachments ?? {};
  return [
    ...(input.newNamespace ? [`Creates the namespace ${namespace}.`] : []),
    `Creates the claim ${name}.`,
    ...(claim.spec.plugins ?? []).map((selection) => {
      const section = input.sections.find(
        (s) => s.metadata.name === selection.section,
      );
      const registration = input.registrations.find(
        (r) => r.metadata.name === selection.registration,
      );
      const sectionLabel = section?.spec.label || selection.section;
      const optionLabel = registration?.spec.label || selection.registration;
      const request = requestName(
        name,
        selection,
        section?.spec.selection ?? "",
      );
      return `Requests ${sectionLabel}: ${optionLabel} as ${request}.`;
    }),
    `Creates the CamundaCluster ${name}.`,
    ...(optimize ? [`Creates the CamundaOptimize ${optimizeName(name)}.`] : []),
    ...(backupSchedule
      ? [`Creates the BackupSchedule ${backupScheduleName(name)}.`]
      : []),
  ];
}
