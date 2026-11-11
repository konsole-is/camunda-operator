import { settingsOf } from "@/components/cluster/editable-settings";
import {
  componentSettings,
  type SettingsGroup,
} from "@/components/cluster/settings";
import type { Process } from "@/components/cluster/topology";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import { clusterFields, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";

/**
 * SettingsPage is the cluster orders in team-a for a settings fixture.
 * preset is spec.cluster of its preset. claim is the claim spec beside its
 * presetRef: the cluster spec is claim.cluster with cluster over it, as the
 * claim controller writes it. optimize is the spec of its CamundaOptimize.
 */
export type SettingsPage = {
  preset?: CamundaClusterSpec;
  cluster?: CamundaClusterSpec;
  claim?: Record<string, unknown>;
  sections?: Resource<"PluginSection">[];
  optimize?: Record<string, unknown>;
};

/** steps returns the field steps of names. */
export function steps(...names: string[]): Step[] {
  return names.map((field) => ({ field }));
}

/** settingsFieldsOf returns the merged spec and the field states of page. */
export function settingsFieldsOf(page: SettingsPage) {
  const spec = {
    ...((page.claim?.cluster ?? {}) as CamundaClusterSpec),
    ...page.cluster,
  };
  const preset =
    page.preset && ({ cluster: page.preset } as CamundaClusterPresetSpec);
  const merged = mergeCluster(preset, undefined, spec);
  const fields = clusterFields({
    cluster: {
      kind: "CamundaCluster",
      metadata: {
        name: "orders",
        namespace: "team-a",
        labels: { [CLAIM_LABEL]: "orders" },
      },
      spec,
    } as unknown as Resource<"CamundaCluster">,
    merged,
    claim: {
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-a" },
      spec: { presetRef: "medium", ...page.claim },
    } as unknown as Resource<"CamundaClusterClaim">,
    sections: page.sections ?? [],
    registrations: [],
    optimize:
      page.optimize &&
      ({
        kind: "CamundaOptimize",
        metadata: { name: "orders", namespace: "team-a" },
        spec: page.optimize,
      } as unknown as Resource<"CamundaOptimize">),
  });
  return { merged, fields };
}

/**
 * settingsGroupOf returns the group of process on the Settings tab of host,
 * or of the fields of the whole cluster for "cluster", with its settings.
 * It throws when host shows no group of process.
 */
export function settingsGroupOf(
  host: Process | "cluster",
  process: Process | "cluster",
  page: SettingsPage,
) {
  const { fields, merged } = settingsFieldsOf(page);
  const group: SettingsGroup | undefined =
    host === "cluster"
      ? {
          label: "Cluster",
          component: "cluster",
          embedded: false,
          fields: fields.filter((f) => f.group === "cluster"),
        }
      : componentSettings(fields, host, merged.spec).find(
          (g) => g.component === process,
        );
  if (!group) throw new Error(`${host} shows no group ${process}`);
  return { group, settings: settingsOf({ group, spec: merged.spec }) };
}

/** accessSection is the section access, whose slot fills the external URLs. */
export function accessSection(): Resource<"PluginSection"> {
  return {
    kind: "PluginSection",
    metadata: { name: "access" },
    spec: {
      label: "Access",
      slot: "externalAccess",
      appliesTo: ["CamundaClusterClaim"],
      selection: "One",
    },
  } as unknown as Resource<"PluginSection">;
}
