import {
  type BackupObject,
  type BackupRow,
  newestBackupRowOf,
} from "@/components/cluster/backups";
import { componentCards } from "@/components/cluster/cards";
import { STORAGE_LABEL } from "@/components/cluster/cluster-header";
import { clusterOptimize } from "@/components/cluster/optimize";
import type { PendingPush } from "@/components/fleet/git-pushes";
import type { ConditionLike } from "@/components/fleet/status";
import {
  type SectionClaim,
  type SectionComponent,
  sectionComponents,
} from "@/lib/claims/sections";
import type { Resource } from "@/lib/k8s/kinds";
import type { RefKey } from "@/lib/write/types";
import { CLAIM_LABEL, isManaged } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import type { Unavailable } from "@/lib/observe/types";
import { byCodeUnit, environmentNameOf } from "@/lib/policy";
import {
  type PodWarnings,
  type Warning,
  backupWarning,
  requestWarnings,
} from "@/lib/warnings";

/** FleetSources are the lists that the fleet view follows. */
export type FleetSources = {
  clusters: Resource<"CamundaCluster">[];
  claims: Resource<"CamundaClusterClaim">[];
  environments: Resource<"Environment">[];
  namespaces: Resource<"Namespace">[];
  presets: Resource<"CamundaClusterPreset">[];
  releases: Resource<"CamundaRelease">[];
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
  requests: Resource<"PluginRequest">[];
  storage: Resource<"SecondaryStorageConfig">[];
  optimizes: Resource<"CamundaOptimize">[];
  backups: BackupObject[];
};

/** ComponentDot is one dot of the Components column. A dot that is not used is grey "Not used". */
export type ComponentDot = {
  id: string;
  label: string;
  used: boolean;
  condition?: ConditionLike;
};

/**
 * ClusterRow is one row of a cluster table: a CamundaCluster, or a live claim
 * that has no CamundaCluster yet. environment is the name of the Environment
 * of the namespace, and undefined puts the row into "No environment". href
 * is the cluster page, and a claim without a cluster has none. sections are
 * the section components of the claim, empty without a claim.
 */
export type ClusterRow = {
  id: string;
  namespace: string;
  name: string;
  environment?: string;
  managed: boolean;
  ready?: ConditionLike;
  version?: string;
  preset?: string;
  components: ComponentDot[];
  sections: SectionComponent[];
  lastBackup?: BackupRow;
  href?: string;
};

/** WaitingRow is a claim that the fleet manager pushed to Git and that the cluster does not show yet. */
export type WaitingRow = {
  id: string;
  key: RefKey;
  namespace: string;
  name: string;
  path: string;
  pushedAt: string;
};

const STORAGE_DOT = "Secondary storage";
const CLUSTER_ROW = "cluster/";

/**
 * componentDots returns the dots of cluster: one for each Camunda process
 * that is not embedded, Connectors and Optimize grey when not used, then one
 * for each section component of its claim, labelled "<section>
 * (<registration>)". A cluster whose claim is not in sources gets the dot of
 * the SecondaryStorageConfig that spec.storageRef names instead, without a
 * state when that object is missing, and no storage dot without storageRef.
 */
export function componentDots(
  cluster: Resource<"CamundaCluster">,
  sources: FleetSources,
): ComponentDot[] {
  const index = indexOf(sources);
  const claim = index.claims.get(claimKey(cluster));
  return dotsOf(
    cluster,
    sources,
    index,
    claim && sectionsOf(claim, sources, index),
  );
}

// dotsOf gives the dots of componentDots. sections is undefined for a
// cluster whose claim is not in sources.
function dotsOf(
  cluster: Resource<"CamundaCluster">,
  sources: FleetSources,
  index: FleetIndex,
  sections: SectionComponent[] | undefined,
): ComponentDot[] {
  const namespace = cluster.metadata?.namespace ?? "";
  const name = cluster.metadata?.name ?? "";
  const merged = mergeCluster(
    sources.presets.find((p) => p.metadata?.name === cluster.spec.presetRef)
      ?.spec,
    sources.releases.find((r) => r.metadata?.name === cluster.spec.releaseRef)
      ?.spec,
    cluster.spec,
  ).spec;
  const processes = componentCards({
    spec: merged,
    pods: [],
    conditions: cluster.status?.conditions,
    optimize: clusterOptimize(name, index.optimizes.get(namespace) ?? []),
  }).map((card): ComponentDot => ({
    id: card.id,
    label: card.label,
    used: card.used,
    ...(card.condition ? { condition: card.condition } : {}),
  }));

  if (sections) {
    return [
      ...processes,
      ...sections.map((component): ComponentDot => ({
        id: component.id,
        label: `${component.label} (${registrationLabel(component.selection.registration, sources.registrations)})`,
        used: true,
        ...(component.condition ? { condition: component.condition } : {}),
      })),
    ];
  }

  const storageRef = cluster.spec.storageRef;
  if (!storageRef) return processes;
  const storage = index.storage.get(key(namespace, storageRef));
  const condition = storage?.status?.conditions?.find(
    (c) => c.type === "Ready",
  );
  return [
    ...processes,
    {
      id: "secondary-storage",
      label: storage
        ? `${STORAGE_DOT} (${STORAGE_LABEL[storage.spec.type]})`
        : STORAGE_DOT,
      used: true,
      ...(condition ? { condition } : {}),
    },
  ];
}

/**
 * clusterRows returns one row for each CamundaCluster, then one for each
 * claim that no CamundaCluster of its namespace names with CLAIM_LABEL,
 * sorted by namespace, then name. A suspended cluster has no version.
 */
export function clusterRows(sources: FleetSources, now: string): ClusterRow[] {
  const index = indexOf(sources);
  const environment = (namespace: string) => {
    const object = index.namespaces.get(namespace);
    return object ? environmentNameOf(object, sources.environments) : undefined;
  };
  const joined = new Set<string>();
  const clusters = sources.clusters.map((cluster): ClusterRow => {
    const namespace = cluster.metadata?.namespace ?? "";
    const name = cluster.metadata?.name ?? "";
    const claim = index.claims.get(claimKey(cluster));
    if (claim) joined.add(key(namespace, claim.metadata?.name));
    const sections = claim && sectionsOf(claim, sources, index);
    const ready = readyOf(cluster.status?.conditions);
    const version = cluster.spec.suspend
      ? undefined
      : cluster.status?.management?.version;
    const lastBackup = newestBackupRowOf(
      cluster,
      index.backups.get(key(namespace, name)) ?? [],
      now,
    );
    return {
      id: `${CLUSTER_ROW}${key(namespace, name)}`,
      namespace,
      name,
      environment: environment(namespace),
      managed: isManaged(cluster),
      ...(ready ? { ready } : {}),
      ...(version ? { version } : {}),
      ...(cluster.spec.presetRef ? { preset: cluster.spec.presetRef } : {}),
      components: dotsOf(cluster, sources, index, sections),
      sections: sections ?? [],
      ...(lastBackup ? { lastBackup } : {}),
      href: `/fleet/clusters/${namespace}/${name}`,
    };
  });
  const claims = sources.claims
    .filter((c) => !joined.has(key(c.metadata?.namespace, c.metadata?.name)))
    .map((claim): ClusterRow => {
      const namespace = claim.metadata?.namespace ?? "";
      const ready = readyOf(claim.status?.conditions);
      return {
        id: `claim/${key(namespace, claim.metadata?.name)}`,
        namespace,
        name: claim.metadata?.name ?? "",
        environment: environment(namespace),
        managed: true,
        ...(ready ? { ready } : {}),
        preset: claim.spec.presetRef,
        components: [],
        sections: sectionsOf(claim, sources, index),
      };
    });
  return [...clusters, ...claims].sort(
    (a, b) =>
      byCodeUnit(a.namespace, b.namespace) || byCodeUnit(a.name, b.name),
  );
}

/**
 * waitingRows returns one row for each pushed CamundaClusterClaim that is
 * not a delete and has no live claim of the same namespace and name.
 */
export function waitingRows(
  pushes: readonly PendingPush[],
  claims: readonly Resource<"CamundaClusterClaim">[],
): WaitingRow[] {
  const live = new Set(
    claims.map((c) => key(c.metadata?.namespace, c.metadata?.name)),
  );
  return pushes
    .filter(
      (push) =>
        push.ref.kind === "CamundaClusterClaim" &&
        !("deleted" in push) &&
        !live.has(key(push.ref.namespace, push.ref.name)),
    )
    .map((push) => ({
      id: `push/${push.key}`,
      key: push.key,
      namespace: push.ref.namespace ?? "",
      name: push.ref.name,
      path: push.path,
      pushedAt: push.pushedAt,
    }));
}

export const ALL_TAB = "all";
export const NO_ENVIRONMENT_TAB = "none";

/** environmentTabValue is the tab value of an Environment. A prefix keeps it apart from ALL_TAB and NO_ENVIRONMENT_TAB. */
export function environmentTabValue(name: string): string {
  return `environment:${name}`;
}

/** EnvironmentTab is one tab. count is the number of cluster rows of the tab, before the filters. */
export type EnvironmentTab = {
  value: string;
  label: string;
  color?: string;
  count: number;
};

const HEX_COLOR = /^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$/;

/**
 * environmentTabs returns no tabs when no Environment exists. Else it returns
 * All, each Environment by name, then No environment when a row has no
 * environment. A tab has a color only when the Environment color is a hex
 * color.
 */
export function environmentTabs(
  environments: readonly Resource<"Environment">[],
  rows: readonly ClusterRow[],
): EnvironmentTab[] {
  if (environments.length === 0) return [];
  const loose = rows.filter((r) => r.environment === undefined).length;
  return [
    { value: ALL_TAB, label: "All", count: rows.length },
    ...byName(environments).map((environment) => ({
      value: environmentTabValue(environment.metadata.name ?? ""),
      ...describe(environment),
      count: rows.filter((r) => r.environment === environment.metadata.name)
        .length,
    })),
    ...(loose > 0
      ? [{ value: NO_ENVIRONMENT_TAB, label: "No environment", count: loose }]
      : []),
  ];
}

/** RowGroup is one cluster table. environment and label are unset for the single list without Environments. */
export type RowGroup = {
  environment?: string;
  label?: string;
  color?: string;
  rows: ClusterRow[];
};

/**
 * groupRows returns the cluster tables of tab. Without Environments it is
 * one table of every row. On ALL_TAB it is one table for each Environment
 * that has rows, in the order of the tabs, then No environment when it has
 * rows. On another tab it is the one table of that tab, also when it is
 * empty.
 */
export function groupRows(
  rows: readonly ClusterRow[],
  environments: readonly Resource<"Environment">[],
  tab: string,
): RowGroup[] {
  if (environments.length === 0) return [{ rows: [...rows] }];
  const groupOf = (environment: Resource<"Environment">): RowGroup => {
    const { label, color } = describe(environment);
    return {
      environment: environment.metadata.name,
      label,
      ...(color ? { color } : {}),
      rows: rows.filter((r) => r.environment === environment.metadata.name),
    };
  };
  const loose: RowGroup = {
    label: "No environment",
    rows: rows.filter((r) => r.environment === undefined),
  };
  if (tab === NO_ENVIRONMENT_TAB) return [loose];
  if (tab === ALL_TAB) {
    return [...byName(environments).map(groupOf), loose].filter(
      (g) => g.rows.length > 0,
    );
  }
  const environment = environments.find(
    (e) => environmentTabValue(e.metadata.name ?? "") === tab,
  );
  return environment ? [groupOf(environment)] : [];
}

export type StatusFilter = "all" | "not-ready" | "with-warnings";

/** RowFilter is the filter bar: search matches the name, and namespace "" is every namespace. */
export type RowFilter = {
  search: string;
  namespace: string;
  status: StatusFilter;
};

/**
 * filterRows keeps the rows whose name holds search, without case and
 * without the spaces around search, that are in namespace, for
 * "not-ready" for which isReady is false, and for "with-warnings" for
 * which hasWarnings is true. Without hasWarnings, "with-warnings" keeps no
 * row.
 */
export function filterRows<R extends { name: string; namespace: string }>(
  rows: readonly R[],
  filter: RowFilter,
  isReady: (row: R) => boolean,
  hasWarnings: (row: R) => boolean = () => false,
): R[] {
  const search = filter.search.trim().toLowerCase();
  return rows.filter(
    (row) =>
      row.name.toLowerCase().includes(search) &&
      (filter.namespace === "" || row.namespace === filter.namespace) &&
      (filter.status !== "not-ready" || !isReady(row)) &&
      (filter.status !== "with-warnings" || hasWarnings(row)),
  );
}

/**
 * rowWarnings returns the warnings of row: the pod warnings of its
 * CamundaCluster, then the backup and request warnings. A claim without a
 * cluster has no pod warnings, and neither has a row while pods is
 * undefined or "unavailable".
 */
export function rowWarnings(
  row: ClusterRow,
  pods: PodWarnings | Unavailable | undefined,
): Warning[] {
  const own =
    pods && pods !== "unavailable" && row.id.startsWith(CLUSTER_ROW)
      ? (pods.clusters[key(row.namespace, row.name)] ?? [])
      : [];
  const backup = backupWarning(row.lastBackup);
  return [
    ...own,
    ...(backup ? [backup] : []),
    ...requestWarnings(row.sections),
  ];
}

/** rowNamespaces returns the namespaces of rows, each once, sorted. */
export function rowNamespaces(
  rows: readonly { namespace: string }[],
): string[] {
  return [...new Set(rows.map((r) => r.namespace))].sort(byCodeUnit);
}

function key(namespace: string | undefined, name: string | undefined) {
  return `${namespace ?? ""}/${name ?? ""}`;
}

// FleetIndex holds the lists of FleetSources that a row looks up, by
// "<namespace>/<name>", or by namespace for optimizes. backups go by the
// cluster that spec.clusterRef names.
type FleetIndex = {
  claims: Map<string, Resource<"CamundaClusterClaim">>;
  requests: Map<string, Resource<"PluginRequest">>;
  backups: Map<string, BackupObject[]>;
  optimizes: Map<string, Resource<"CamundaOptimize">[]>;
  storage: Map<string, Resource<"SecondaryStorageConfig">>;
  namespaces: Map<string, Resource<"Namespace">>;
};

function indexOf(sources: FleetSources): FleetIndex {
  const named = <
    T extends { metadata?: { namespace?: string; name?: string } },
  >(
    items: readonly T[],
  ) =>
    new Map(
      items.map((i) => [key(i.metadata?.namespace, i.metadata?.name), i]),
    );
  return {
    claims: named(sources.claims),
    requests: named(sources.requests),
    backups: grouped(sources.backups, (b) =>
      key(b.metadata?.namespace, b.spec.clusterRef.name),
    ),
    optimizes: grouped(sources.optimizes, (o) => o.metadata?.namespace ?? ""),
    storage: named(sources.storage),
    namespaces: new Map(
      sources.namespaces.map((n) => [n.metadata?.name ?? "", n]),
    ),
  };
}

function grouped<T>(
  items: readonly T[],
  keyOf: (item: T) => string,
): Map<string, T[]> {
  const groups = new Map<string, T[]>();
  for (const item of items) {
    const k = keyOf(item);
    const group = groups.get(k);
    if (group) group.push(item);
    else groups.set(k, [item]);
  }
  return groups;
}

function claimKey(cluster: Resource<"CamundaCluster">): string {
  return key(
    cluster.metadata?.namespace,
    cluster.metadata?.labels?.[CLAIM_LABEL] ?? "",
  );
}

// A claim holds the parameters of a selection as unknown, because the CRD
// schema does not give them a shape. sectionComponents reads no parameter.
function sectionsOf(
  claim: Resource<"CamundaClusterClaim">,
  sources: FleetSources,
  index: FleetIndex,
): SectionComponent[] {
  const namespace = claim.metadata?.namespace;
  return sectionComponents({
    claim: claim as SectionClaim,
    sections: sources.sections,
    requests: (claim.status?.requests ?? []).flatMap((r) => {
      const request = index.requests.get(key(namespace, r.name));
      return request ? [request] : [];
    }),
  });
}

function registrationLabel(
  name: string,
  registrations: readonly Resource<"PluginRegistration">[],
): string {
  return (
    registrations.find((r) => r.metadata?.name === name)?.spec.label || name
  );
}

function readyOf(
  conditions: readonly (ConditionLike & { type: string })[] | undefined,
): ConditionLike | undefined {
  return conditions?.find((c) => c.type === "Ready");
}

function byName(
  environments: readonly Resource<"Environment">[],
): Resource<"Environment">[] {
  return [...environments].sort((a, b) =>
    byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
  );
}

function describe(environment: Resource<"Environment">): {
  label: string;
  color?: string;
} {
  const color = environment.spec?.color;
  return {
    label: environment.spec?.displayName || (environment.metadata.name ?? ""),
    ...(color && HEX_COLOR.test(color) ? { color } : {}),
  };
}
