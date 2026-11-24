import { clusterVersionOf } from "@/components/cluster/backup-actions";
import type { KindName } from "@/lib/k8s/kinds";
import { type SourceReads, unreadKinds } from "@/lib/k8s/unread";
import { byMinor, mergeCluster, minorOf } from "@/lib/merge";
import { authMethod } from "@/lib/planes/identity";
import { byCodeUnit } from "@/lib/policy";
import {
  PLATFORM_USER_KINDS,
  platformUserCount,
  platformUsers,
  type PlatformUsers,
} from "@/lib/usage";
import type { PlatformSources } from "./sources";

/** PLATFORM_HREF is the path of the Platform page. */
export const PLATFORM_HREF = "/setup/platform";

/** platformConfigHref returns the path of the page of the platform config name. */
export function platformConfigHref(name: string): string {
  return `${PLATFORM_HREF}/${encodeURIComponent(name)}`;
}

/** Condition is the Ready condition of a platform config as the list shows it. */
export type Condition = {
  status: "True" | "False" | "Unknown";
  reason?: string;
  message?: string;
};

/**
 * PlatformRow is one platform config of the list. users is undefined while
 * a kind of PLATFORM_USER_KINDS is not read. ready is undefined while the
 * config has no Ready condition.
 */
export type PlatformRow = {
  name: string;
  method: "basic" | "oidc";
  users?: number;
  ready?: Condition;
};

/** MINOR_KINDS are the kinds whose failed read can hide a Camunda minor of the users. */
export const MINOR_KINDS: readonly KindName[] = [
  "CamundaCluster",
  "CamundaManagementCluster",
  "CamundaClusterPreset",
  "CamundaRelease",
];

/** ROLL_NOTICE says which users a change of a platform config changes. */
export const ROLL_NOTICE =
  "A change rolls the pods of every cluster and management plane that uses this config, and of the Optimize of each such cluster. A database server changes only when the postgres image changes.";

/** CREATE_EFFECT says what the create of a platform config changes. */
export const CREATE_EFFECT =
  "A resource or an environment that names this config uses it from now on.";

/** DELETE_EFFECT says what a delete does to the users of a platform config. */
export const DELETE_EFFECT =
  "Users of a deleted config report InvalidReference and keep running.";

/** unreadUserKinds returns each kind of PLATFORM_USER_KINDS that reads holds as failed or stale, once. */
export function unreadUserKinds(reads: SourceReads): string[] {
  return unreadKinds(reads).filter((kind) =>
    (PLATFORM_USER_KINDS as readonly string[]).includes(kind),
  );
}

// CLUSTER_MINOR_KINDS feed only the minors of clusters.
const CLUSTER_MINOR_KINDS: readonly string[] = [
  "CamundaClusterPreset",
  "CamundaRelease",
];

/**
 * unreadMinorKinds returns each kind of MINOR_KINDS that reads holds as
 * failed or stale and that can hide a Camunda minor of users, once.
 */
export function unreadMinorKinds(
  reads: SourceReads,
  users: PlatformUsers,
): string[] {
  return unreadKinds(reads).filter(
    (kind) =>
      (MINOR_KINDS as readonly string[]).includes(kind) &&
      (users.clusters.length > 0 || !CLUSTER_MINOR_KINDS.includes(kind)),
  );
}

/** platformRows returns one row for each platform config of sources, sorted by name. */
export function platformRows(
  sources: PlatformSources & SourceReads,
): PlatformRow[] {
  const known = unreadUserKinds(sources).length === 0;
  return sources.platformConfigs
    .map((config) => {
      const name = config.metadata.name ?? "";
      const ready = config.status?.conditions?.find((c) => c.type === "Ready");
      return {
        name,
        method: authMethod(config),
        ...(known && { users: platformUserCount(usersOf(name, sources)) }),
        ...(ready && { ready }),
      };
    })
    .sort((a, b) => byCodeUnit(a.name, b.name));
}

/** usersOf returns the users of the platform config name in sources. */
export function usersOf(name: string, sources: PlatformSources): PlatformUsers {
  return platformUsers(name, sources);
}

/**
 * userMinors returns the Camunda minors that the clusters and management
 * planes of users run, deduped and sorted by number. A cluster runs the
 * version that it reports, else the version of its merged spec. A
 * management plane runs the minors of Management Identity, Console, and Web
 * Modeler. Database servers and claims run no Camunda minor.
 */
export function userMinors(
  users: PlatformUsers,
  sources: PlatformSources,
): string[] {
  const versions = [
    ...users.clusters.map((cluster) => {
      const preset = sources.clusterPresets.find(
        (p) => p.metadata.name === cluster.spec.presetRef,
      );
      const release = sources.releases.find(
        (r) => r.metadata.name === cluster.spec.releaseRef,
      );
      return clusterVersionOf(
        cluster,
        mergeCluster(preset?.spec, release?.spec, cluster.spec).spec,
      );
    }),
    ...users.planes.flatMap((plane) => [
      plane.spec.identity?.version,
      plane.spec.console?.version,
      plane.spec.webModeler?.version,
    ]),
  ];
  const minors = new Set(
    versions.flatMap((v) => {
      const minor = minorOf(v ?? "");
      return minor === undefined ? [] : [minor];
    }),
  );
  return [...minors].sort(byMinor);
}
