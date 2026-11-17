// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { KindName } from "@/lib/k8s/kinds";
import { unreadDeleteText } from "@/lib/k8s/unread";
import { byCodeUnit } from "@/lib/policy";
import { allowedIn, defaultIn } from "./environments";
import { platformUsers, type PlatformUsageSources } from "./platform";
import {
  findPresetUsers,
  type PresetKind,
  type PresetUser,
  type PresetUsageSources,
} from "./presets";
import { type ReleaseUsageSources, releaseUsage } from "./releases";

/**
 * DeleteBlock says why a delete is not possible. reason is the sentence
 * that the page shows. users names each object that uses the deleted
 * object, as "<kind> <namespace>/<name>" or "Environment <name>", and is
 * empty when a list is not read or not current.
 */
export type DeleteBlock = { reason: string; users: string[] };

/** deleteRefusalText is the message of a refused delete: the reason, then the users of block. */
export function deleteRefusalText(block: DeleteBlock): string {
  return block.users.length === 0
    ? block.reason
    : `${block.reason} Used by ${block.users.join(", ")}.`;
}

/** PRESET_DELETE_KINDS are the lists that can hold a user of a preset of each kind. */
export const PRESET_DELETE_KINDS: Record<PresetKind, readonly KindName[]> = {
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

/**
 * PresetDeleteCheck is the delete guard of one preset: users are the
 * presetUsers, allowed the environments that allow it, unread each kind of
 * PRESET_DELETE_KINDS that is not read, and block is set when the delete
 * is not possible.
 */
export type PresetDeleteCheck = {
  users: PresetUser[];
  allowed: string[];
  unread: KindName[];
  block?: DeleteBlock;
};

/**
 * presetDeleteChecks returns the delete guard of each preset over sources.
 * unread holds each kind whose list failed, and pending each kind whose
 * list is not current yet. A delete is possible only while every list of
 * PRESET_DELETE_KINDS of the kind is read and current, nothing uses the
 * preset, and no environment allows it.
 */
export function presetDeleteChecks(
  sources: PresetUsageSources,
  unread: readonly string[],
  pending: readonly string[] = [],
): (kind: PresetKind, name: string) => PresetDeleteCheck {
  const usersOf = findPresetUsers(sources);
  return (kind, name) => {
    const users = usersOf(kind, name);
    const allowed = allowedIn(sources.environments, kind, name);
    const missing = PRESET_DELETE_KINDS[kind].filter((k) => unread.includes(k));
    const stale = PRESET_DELETE_KINDS[kind].some((k) => pending.includes(k));
    const reason = presetBlockReason(missing, stale, users, allowed);
    const named =
      missing.length > 0 || stale
        ? []
        : [
            ...users.map((u) => `${u.kind} ${u.namespace}/${u.name}`),
            ...allowed.map((e) => `Environment ${e}`),
          ];
    return {
      users,
      allowed,
      unread: missing,
      ...(reason !== undefined && { block: { reason, users: named } }),
    };
  };
}

function presetBlockReason(
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

/** RELEASE_IN_USE_TEXT says why the delete of a release that something uses is not possible. */
export const RELEASE_IN_USE_TEXT =
  "Delete is possible when no cluster, Elasticsearch cluster, database server, or claim names this release and no environment allows it.";

/**
 * releaseDeleteBlock returns why the release name cannot be deleted, or
 * undefined when it can. unread holds each kind of the lists of sources
 * that failed or is not current. A delete is possible only while unread is
 * empty, nothing uses the release, claims included, and no environment
 * allows it or has it as its default.
 */
export function releaseDeleteBlock(
  name: string,
  sources: ReleaseUsageSources & { environments: readonly Environment[] },
  unread: readonly string[],
): DeleteBlock | undefined {
  if (unread.length > 0) return { reason: unreadDeleteText(unread), users: [] };
  const usage = releaseUsage(name, sources);
  const environments = [
    ...new Set([
      ...defaultIn(sources.environments, "CamundaRelease", name),
      ...allowedIn(sources.environments, "CamundaRelease", name),
    ]),
  ].sort(byCodeUnit);
  const users = [
    ...usage.clusters.map((ref) => `CamundaCluster ${ref}`),
    ...usage.elasticsearchClusters.map((ref) => `ElasticsearchCluster ${ref}`),
    ...usage.databaseServers.map((ref) => `DatabaseServer ${ref}`),
    ...usage.claims.map((ref) => `CamundaClusterClaim ${ref}`),
    ...environments.map((e) => `Environment ${e}`),
  ];
  return users.length === 0
    ? undefined
    : { reason: RELEASE_IN_USE_TEXT, users };
}

/** PLATFORM_USER_KINDS are the kinds whose failed read can hide a user of a CamundaPlatformConfig. */
export const PLATFORM_USER_KINDS: readonly KindName[] = [
  "CamundaCluster",
  "CamundaManagementCluster",
  "DatabaseServer",
  "DatabaseServerPreset",
  "CamundaClusterClaim",
  "CamundaManagementClusterClaim",
  "Environment",
];

/** PLATFORM_IN_USE_TEXT says why the delete of a platform config that something uses is not possible. */
export const PLATFORM_IN_USE_TEXT =
  "Delete is possible when no cluster, management plane, database server, claim, or environment uses this config.";

/**
 * platformDeleteBlock returns why the CamundaPlatformConfig name cannot be
 * deleted, or undefined when it can. unread holds each kind whose list
 * failed or is not current. A delete is possible only while no kind of
 * PLATFORM_USER_KINDS is in unread and platformUsers finds no user.
 */
export function platformDeleteBlock(
  name: string,
  sources: PlatformUsageSources,
  unread: readonly string[],
): DeleteBlock | undefined {
  const missing = unread.filter((k) =>
    (PLATFORM_USER_KINDS as readonly string[]).includes(k),
  );
  if (missing.length > 0)
    return { reason: unreadDeleteText(missing), users: [] };
  const found = platformUsers(name, sources);
  const ref = (o: { metadata: { namespace?: string; name?: string } }) =>
    `${o.metadata.namespace ?? ""}/${o.metadata.name ?? ""}`;
  const users = [
    ...found.clusters.map((c) => `CamundaCluster ${ref(c)}`),
    ...found.planes.map((p) => `CamundaManagementCluster ${ref(p)}`),
    ...found.databaseServers.map((d) => `DatabaseServer ${ref(d.server)}`),
    ...found.clusterClaims.map((c) => `CamundaClusterClaim ${ref(c)}`),
    ...found.planeClaims.map((c) => `CamundaManagementClusterClaim ${ref(c)}`),
    ...found.environments.map((e) => `Environment ${e}`),
  ];
  return users.length === 0
    ? undefined
    : { reason: PLATFORM_IN_USE_TEXT, users };
}
