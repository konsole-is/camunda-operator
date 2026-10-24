import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { ClusterField } from "@/lib/fields";
import type { FieldPath } from "@/lib/merge";
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
