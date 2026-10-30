import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { ClusterField, ClusterFieldsInput } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { BUILTIN_CONTROLLER } from "@/lib/labels";
import type { FieldPath, Merged } from "@/lib/merge";
import { PROCESS_LABEL, topologyOf, type Process } from "./topology";

/** SettingsGroup is the fields of one process. embedded marks an app that runs inside the host. */
export type SettingsGroup = {
  label: string;
  embedded: boolean;
  fields: ClusterField[];
};

/**
 * componentSettings returns the groups of the Settings tab of a component:
 * its own fields first, then, for a host, the fields of each app that runs
 * inside it, in the order of topologyOf. A group can have no field.
 * "optimize" has its own group only.
 */
export function componentSettings(
  fields: readonly ClusterField[],
  component: Process | "optimize",
  spec: CamundaClusterSpec,
): SettingsGroup[] {
  const of = (group: string) => fields.filter((f) => f.group === group);
  if (component === "optimize")
    return [{ label: "Optimize", embedded: false, fields: of("optimize") }];
  const box = topologyOf(spec, []).find((b) => b.process === component);
  return [
    { label: PROCESS_LABEL[component], embedded: false, fields: of(component) },
    ...(box?.embedded ?? []).map((app) => ({
      label: app.label,
      embedded: true,
      fields: of(app.process),
    })),
  ];
}

/** claimFieldHref links the Claim tab of a cluster at a claim path. */
export function claimFieldHref(
  namespace: string,
  name: string,
  claimPath: FieldPath,
): string {
  return `/fleet/clusters/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}?tab=claim&field=${encodeURIComponent(claimPath)}`;
}

/**
 * operatorVersionOf returns the operator version that the built-in
 * registrations carry, or "" when none is listed.
 */
export function operatorVersionOf(
  registrations: readonly Resource<"PluginRegistration">[],
): string {
  return (
    registrations.find((r) => r.spec.controller?.name === BUILTIN_CONTROLLER)
      ?.spec.controller?.version ?? ""
  );
}

/**
 * clusterFieldsInputOf returns the input of clusterFields for the objects of
 * the cluster page. The policy is the resolved policy of claim, and the
 * backup schedule is the first schedule by name whose clusterRef names the
 * cluster.
 */
export function clusterFieldsInputOf(input: {
  cluster: Resource<"CamundaCluster">;
  merged: Merged<CamundaClusterSpec>;
  claim?: Resource<"CamundaClusterClaim">;
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  optimize?: Resource<"CamundaOptimize">;
  schedules: readonly Resource<"BackupSchedule">[];
}): ClusterFieldsInput {
  const { schedules, ...rest } = input;
  const name = input.cluster.metadata?.name;
  const backupSchedule = schedules
    .filter((s) => s.spec.clusterRef.name === name)
    .sort((a, b) =>
      (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? ""),
    )[0];
  return {
    ...rest,
    policy: input.claim?.status?.resolvedPolicy,
    backupSchedule,
  };
}
