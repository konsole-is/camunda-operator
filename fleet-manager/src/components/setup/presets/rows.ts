import type { V1Namespace } from "@kubernetes/client-node";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { KindName } from "@/lib/k8s/kinds";
import { type ReadError, unreadDeleteText } from "@/lib/k8s/unread";
import { presetDescription } from "@/lib/labels";
import { lastChanged } from "@/lib/merge";
import { byCodeUnit } from "@/lib/policy";
import {
  allowedIn,
  type PresetKind,
  type PresetUser,
  type PresetUserKind,
  findPresetUsers,
} from "@/lib/usage";
import { PRESET_KINDS, type PresetObject, presetBody } from "./kinds";
import type { PresetSources } from "./sources";

/** Fact is one key fact of a preset, as a label and a value. */
export type Fact = { label: string; value: string };

/**
 * PresetRow is one preset of the list. id is <slug>/<name>, unique over the
 * three kinds. lastChanged is an ISO time. blockedBy says why delete is not
 * offered, and is set exactly when deletable is false.
 */
export type PresetRow = {
  id: string;
  kind: PresetKind;
  name: string;
  preset: PresetObject;
  description?: string;
  facts: Fact[];
  allowedIn: string[];
  users: PresetUser[];
  /** allowedUnknown is true while the environments cannot be read, so allowedIn can miss entries. */
  allowedUnknown: boolean;
  /** usersUnknown is true while a list that the users of the preset come from cannot be read, so users can miss entries. */
  usersUnknown: boolean;
  lastChanged?: string;
  deletable: boolean;
  blockedBy?: string;
};

/** GUARD_KINDS are the lists that the delete guard of each preset kind reads. */
export const GUARD_KINDS: Record<PresetKind, readonly KindName[]> = {
  CamundaClusterPreset: [
    "Environment",
    "CamundaCluster",
    "CamundaClusterClaim",
  ],
  ElasticsearchClusterPreset: [
    "Environment",
    "Namespace",
    "PluginRegistration",
    "ElasticsearchCluster",
    "CamundaClusterClaim",
    "CamundaManagementClusterClaim",
    "PluginRequest",
  ],
  DatabaseServerPreset: [
    "Environment",
    "Namespace",
    "PluginRegistration",
    "DatabaseServer",
    "CamundaClusterClaim",
    "CamundaManagementClusterClaim",
    "PluginRequest",
  ],
};

const FACTS: Record<PresetKind, [string, string[]][]> = {
  CamundaClusterPreset: [
    ["Brokers", ["zeebe", "replicas"]],
    ["Partitions", ["zeebe", "partitions"]],
    ["CPU", ["zeebe", "resources", "requests", "cpu"]],
    ["Memory", ["zeebe", "resources", "requests", "memory"]],
    ["Storage", ["zeebe", "storageSize"]],
  ],
  ElasticsearchClusterPreset: [
    ["Nodes", ["replicas"]],
    ["CPU", ["resources", "requests", "cpu"]],
    ["Memory", ["resources", "requests", "memory"]],
    ["Storage", ["storageSize"]],
  ],
  DatabaseServerPreset: [
    ["Instances", ["instances"]],
    ["CPU", ["resources", "requests", "cpu"]],
    ["Memory", ["resources", "requests", "memory"]],
    ["Storage", ["storageSize"]],
  ],
};

/**
 * presetFacts returns the key facts that body sets, in a fixed order for
 * each kind: the brokers, nodes, or instances, the partitions of a cluster,
 * the CPU and memory requests, and the storage size.
 */
export function presetFacts(
  kind: PresetKind,
  body: Record<string, unknown> | undefined,
): Fact[] {
  return FACTS[kind].flatMap(([label, steps]) => {
    const value = steps.reduce<unknown>(
      (at, step) =>
        typeof at === "object" && at !== null
          ? (at as Record<string, unknown>)[step]
          : undefined,
      body,
    );
    return value === undefined || value === null || value === ""
      ? []
      : [{ label, value: String(value) }];
  });
}

/**
 * presetRows returns one row for each preset of sources, sorted by the order
 * of PRESET_KINDS, then by name. A preset is deletable only when no list of
 * its GUARD_KINDS is in errors (unread) or pending (not current yet),
 * nothing uses it, and no environment allows it.
 */
export function presetRows(
  sources: PresetSources,
  errors: readonly ReadError[],
  pending: readonly KindName[] = [],
): PresetRow[] {
  const lists: Record<PresetKind, readonly PresetObject[]> = {
    CamundaClusterPreset: sources.clusterPresets,
    ElasticsearchClusterPreset: sources.elasticsearchPresets,
    DatabaseServerPreset: sources.databasePresets,
  };
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which allowedIn and findPresetUsers do not read.
  const environments = sources.environments as unknown as Environment[];
  const usersOf = findPresetUsers({
    ...sources,
    environments,
    namespaces: sources.namespaces as unknown as V1Namespace[],
    registrations: sources.registrations as unknown as PluginRegistration[],
  });
  return PRESET_KINDS.flatMap(({ kind, slug }) =>
    [...lists[kind]]
      .sort((a, b) => byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""))
      .map((preset) => {
        const name = preset.metadata.name ?? "";
        const users = usersOf(kind, name);
        const allowed = allowedIn(environments, kind, name);
        const unread = GUARD_KINDS[kind].filter((k) =>
          errors.some((e) => e.kind === k),
        );
        const blockedBy = blockedByOf(
          unread,
          GUARD_KINDS[kind].some((k) => pending.includes(k)),
          users,
          allowed,
        );
        const description = presetDescription(preset);
        const changed = lastChanged(preset);
        return {
          id: `${slug}/${name}`,
          kind,
          name,
          preset,
          ...(description !== undefined && { description }),
          facts: presetFacts(kind, presetBody(preset)),
          allowedIn: allowed,
          users,
          allowedUnknown: unread.includes("Environment"),
          // The environments give the policy of a claim of an Elasticsearch
          // or PostgreSQL preset, so they find its users too.
          usersUnknown: unread.some(
            (k) => k !== "Environment" || kind !== "CamundaClusterPreset",
          ),
          ...(changed !== undefined && { lastChanged: changed }),
          deletable: blockedBy === undefined,
          ...(blockedBy !== undefined && { blockedBy }),
        };
      }),
  );
}

function blockedByOf(
  unread: readonly KindName[],
  pending: boolean,
  users: readonly PresetUser[],
  allowed: readonly string[],
): string | undefined {
  if (unread.length > 0) return unreadDeleteText(unread);
  if (pending)
    return "Delete is possible when the fleet manager has the current lists.";
  if (users.length > 0)
    return "Delete is possible when no resource, claim, or request uses the preset.";
  if (allowed.length > 0)
    return "Delete is possible when no environment allows the preset.";
  return undefined;
}

const USER_WORDS: [readonly PresetUserKind[], string][] = [
  [["CamundaCluster"], "cluster"],
  [["ElasticsearchCluster"], "Elasticsearch cluster"],
  [["DatabaseServer"], "database server"],
  [["CamundaClusterClaim", "CamundaManagementClusterClaim"], "claim"],
  [["PluginRequest"], "request"],
];

/** usedByLabel returns "Unused", or the number of users of each kind, such as "2 clusters, 1 claim". Both claim kinds count as claims. */
export function usedByLabel(users: readonly PresetUser[]): string {
  const counts = USER_WORDS.flatMap(([kinds, word]) => {
    const n = users.filter((u) => kinds.includes(u.kind)).length;
    return n === 0 ? [] : [`${n} ${word}${n === 1 ? "" : "s"}`];
  });
  return counts.length === 0 ? "Unused" : counts.join(", ");
}

/** changedLabel returns an ISO time as YYYY-MM-DD HH:MM UTC. */
export function changedLabel(iso: string): string {
  const time = new Date(iso).toISOString();
  return `${time.slice(0, 10)} ${time.slice(11, 16)} UTC`;
}
