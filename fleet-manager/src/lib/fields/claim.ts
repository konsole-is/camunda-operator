import type { Step } from "./walk";

/**
 * RefusedField is a field that a cluster claim cannot set in spec.cluster,
 * and what sets it instead: the claim in its own field, the release, the
 * claim controller, or a selected section.
 */
export type RefusedField = {
  names: readonly string[];
  setBy: "claim" | "release" | "claimController" | "section";
};

/**
 * REFUSED_CLUSTER_FIELDS are the fields of the CEL rule on spec.cluster of
 * CamundaClusterClaim, in its order. names are the JSON names under
 * spec.cluster.
 */
export const REFUSED_CLUSTER_FIELDS: readonly RefusedField[] = [
  { names: ["presetRef"], setBy: "claim" },
  { names: ["releaseRef"], setBy: "claim" },
  { names: ["platformConfigRef"], setBy: "claim" },
  { names: ["suspend"], setBy: "claim" },
  { names: ["version"], setBy: "release" },
  { names: ["connectors", "version"], setBy: "release" },
  { names: ["serviceAccount", "name"], setBy: "claimController" },
  { names: ["connectors", "serviceAccount", "name"], setBy: "claimController" },
  { names: ["storageRef"], setBy: "section" },
  { names: ["backupStorageRef"], setBy: "section" },
];

/** refusedSteps returns the steps of a row from the root of the claim: spec, cluster, then names. */
export function refusedSteps(row: RefusedField): Step[] {
  return ["spec", "cluster", ...row.names].map((field) => ({ field }));
}
