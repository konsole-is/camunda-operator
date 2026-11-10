"use client";

import Link from "next/link";
import { type ReactNode, useEffect, useId, useMemo, useState } from "react";
import { type PendingPush, useGitPushes } from "@/components/fleet/git-pushes";
import { PlanesTable, usePlanes } from "@/components/planes/planes-table";
import {
  type Plane,
  type PlaneSources,
  planesIn,
} from "@/components/planes/plane";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName } from "@/lib/k8s/kinds";
import type { RefKey } from "@/lib/write/types";
import { ClusterTable, WaitingTable } from "./cluster-table";
import {
  ALL_TAB,
  type ClusterRow,
  type FleetSources,
  NO_ENVIRONMENT_TAB,
  type RowFilter,
  type RowGroup,
  type StatusFilter,
  clusterRows,
  environmentTabValue,
  environmentTabs,
  filterRows,
  groupRows,
  rowNamespaces,
  waitingRows,
} from "./model";

import type { ReadError } from "@/components/planes/planes-table";

export type { ReadError };

/**
 * FleetPageData is the first snapshot of the fleet page. readErrors holds
 * the kinds whose first read failed.
 */
export type FleetPageData = FleetSources & {
  planes: PlaneSources;
  readErrors?: ReadError[];
};

const NOW_INTERVAL_MS = 30_000;
const NO_MATCH = "No cluster matches the search and the filters.";
const EMPTY_FLEET =
  "No Camunda cluster yet. Create one with New cluster, or apply a CamundaCluster with kubectl.";

const isReady = (row: ClusterRow) => row.ready?.status === "True";

// A failed read of one of these kinds leaves the fleet without rows, so the
// page does not say that the fleet is empty.
const UNREAD_EMPTY = ["CamundaCluster", "CamundaClusterClaim"];

/** FleetView is the fleet page on the given lists. It fetches nothing. */
export function FleetView({
  sources,
  planes,
  pushes,
  errors = [],
  now,
  onStopWaiting,
}: {
  sources: FleetSources;
  planes: Plane[];
  pushes: PendingPush[];
  errors?: ReadError[];
  now: string;
  onStopWaiting?: (key: RefKey) => void;
}): ReactNode {
  const [tab, setTab] = useState(ALL_TAB);
  const [filter, setFilter] = useState<RowFilter>({
    search: "",
    namespace: "",
    status: "all",
  });
  const rows = useMemo(() => clusterRows(sources, now), [sources, now]);
  const waiting = waitingRows(pushes, sources.claims);
  const tabs = environmentTabs(sources.environments, rows);
  const active = tabs.some((t) => t.value === tab) ? tab : ALL_TAB;

  const content = (
    <TabContent
      tab={tabs.length === 0 ? undefined : active}
      rows={rows}
      waiting={waiting}
      planes={planes}
      sources={sources}
      filter={filter}
      onFilter={setFilter}
      now={now}
      unread={errors.some((e) => UNREAD_EMPTY.includes(e.kind))}
      onStopWaiting={onStopWaiting}
    />
  );

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
          Fleet
        </h1>
        <Button asChild>
          <Link href="/fleet/clusters/new">New cluster</Link>
        </Button>
      </div>
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {tabs.length === 0 ? (
        content
      ) : (
        <Tabs value={active} onValueChange={setTab} className="gap-6">
          <TabsList className="max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto">
            {tabs.map((t) => (
              <TabsTrigger
                key={t.value}
                value={t.value}
                className="max-w-full min-w-0 flex-none px-2.5"
              >
                {t.color && (
                  <span
                    data-swatch
                    aria-hidden
                    className="size-2.5 shrink-0 rounded-sm"
                    style={{ backgroundColor: t.color }}
                  />
                )}
                <span title={t.label} className="truncate">
                  {t.label}
                </span>
                <span className="text-xs text-muted tabular-nums">
                  {t.count}
                </span>
              </TabsTrigger>
            ))}
          </TabsList>
          {tabs.map((t) => (
            <TabsContent key={t.value} value={t.value}>
              {content}
            </TabsContent>
          ))}
        </Tabs>
      )}
    </div>
  );
}

// TabContent is the content of one tab. tab is undefined when no
// Environment exists and the view has no tabs.
function TabContent({
  tab,
  rows,
  waiting,
  planes,
  sources,
  filter,
  onFilter,
  now,
  unread,
  onStopWaiting,
}: {
  tab?: string;
  rows: ClusterRow[];
  waiting: ReturnType<typeof waitingRows>;
  planes: Plane[];
  sources: FleetSources;
  filter: RowFilter;
  onFilter: (filter: RowFilter) => void;
  now: string;
  unread: boolean;
  onStopWaiting?: (key: RefKey) => void;
}) {
  const environment =
    tab === undefined || tab === ALL_TAB
      ? undefined
      : tab === NO_ENVIRONMENT_TAB
        ? null
        : sources.environments.find(
            (e) => environmentTabValue(e.metadata.name ?? "") === tab,
          )?.metadata.name;
  const shownPlanes = planesIn(planes, environment);
  const namespaces = rowNamespaces([
    ...groupRows(rows, sources.environments, tab ?? ALL_TAB).flatMap(
      (g) => g.rows,
    ),
    ...waiting,
  ]);
  // A live update can remove the last row of the selected namespace. The
  // select then shows All, so the rows must not stay filtered by it.
  const active = namespaces.includes(filter.namespace)
    ? filter
    : { ...filter, namespace: "" };
  const groups = groupRows(
    filterRows(rows, active, isReady),
    sources.environments,
    tab ?? ALL_TAB,
  );
  const shownWaiting = filterRows(waiting, active, () => false);
  const empty = rows.length === 0 && waiting.length === 0 && !unread;

  return (
    <div className="flex flex-col gap-8">
      <section className="flex flex-col gap-3">
        <h2 className="font-heading text-lg font-semibold">
          Management planes
        </h2>
        {shownPlanes.length > 0 ? (
          <PlanesTable
            planes={planes}
            environment={environment}
            toolbar={false}
          />
        ) : (
          <p className="text-sm text-muted">
            {environment === undefined
              ? "No management plane yet."
              : "No management plane serves this environment."}
          </p>
        )}
      </section>
      <section className="flex flex-col gap-4">
        <h2 className="font-heading text-lg font-semibold">Clusters</h2>
        {empty ? (
          <p className="text-sm text-muted">{EMPTY_FLEET}</p>
        ) : (
          <>
            <FilterBar
              filter={filter}
              onFilter={onFilter}
              namespaces={namespaces}
            />
            {shownWaiting.length > 0 && (
              <WaitingTable
                key={`${active.search}|${active.namespace}|${active.status}`}
                rows={shownWaiting}
                now={now}
                onStopWaiting={onStopWaiting}
              />
            )}
            {groups.length === 0 ? (
              <p className="text-sm text-muted">{NO_MATCH}</p>
            ) : (
              groups.map((group) => (
                <Group
                  // A new key on each change of the filters starts the
                  // table again at its first page.
                  key={`${group.environment ?? group.label ?? "all"}|${active.search}|${active.namespace}|${active.status}`}
                  group={group}
                  filtered={rows.some((r) => inGroup(r, group))}
                />
              ))
            )}
          </>
        )}
      </section>
    </div>
  );
}

function inGroup(row: ClusterRow, group: RowGroup): boolean {
  if (group.label === undefined) return true;
  return row.environment === group.environment;
}

function Group({ group, filtered }: { group: RowGroup; filtered: boolean }) {
  const table = (
    <ClusterTable
      label={group.label ?? "Clusters"}
      rows={group.rows}
      emptyText={filtered ? NO_MATCH : "No cluster in this environment yet."}
    />
  );
  if (group.label === undefined) return table;
  return (
    <div className="flex flex-col gap-2">
      <h3 className="flex items-center gap-2.5 text-sm font-semibold text-text-2">
        <span
          aria-hidden
          className="h-4 w-1 rounded-full bg-border"
          style={group.color ? { backgroundColor: group.color } : undefined}
        />
        {group.label}
      </h3>
      {table}
    </div>
  );
}

function FilterBar({
  filter,
  onFilter,
  namespaces,
}: {
  filter: RowFilter;
  onFilter: (filter: RowFilter) => void;
  namespaces: string[];
}) {
  const id = useId();
  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="flex min-w-48 flex-1 flex-col gap-1">
        <label htmlFor={`${id}-search`} className="text-sm text-muted">
          Search clusters
        </label>
        <Input
          id={`${id}-search`}
          type="search"
          value={filter.search}
          onChange={(e) => onFilter({ ...filter, search: e.target.value })}
        />
      </div>
      <div className="flex min-w-40 flex-col gap-1">
        <label htmlFor={`${id}-namespace`} className="text-sm text-muted">
          Namespace
        </label>
        <NativeSelect
          id={`${id}-namespace`}
          value={namespaces.includes(filter.namespace) ? filter.namespace : ""}
          onChange={(e) => onFilter({ ...filter, namespace: e.target.value })}
        >
          <NativeSelectOption value="">All</NativeSelectOption>
          {namespaces.map((n) => (
            <NativeSelectOption key={n} value={n}>
              {n}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      <div className="flex min-w-40 flex-col gap-1">
        <label htmlFor={`${id}-status`} className="text-sm text-muted">
          Status
        </label>
        <NativeSelect
          id={`${id}-status`}
          value={filter.status}
          onChange={(e) =>
            onFilter({ ...filter, status: e.target.value as StatusFilter })
          }
        >
          <NativeSelectOption value="all">All</NativeSelectOption>
          <NativeSelectOption value="not-ready">Not ready</NativeSelectOption>
        </NativeSelect>
      </div>
    </div>
  );
}

/** FleetViewLive is FleetView on the live lists, starting from initial. It never watches pods, logs, or events. */
export function FleetViewLive({
  initial,
  now,
}: {
  initial: FleetPageData;
  now: string;
}): ReactNode {
  const clusters = useResources("CamundaCluster", {}, initial.clusters);
  const claims = useResources("CamundaClusterClaim", {}, initial.claims);
  const environments = useResources("Environment", {}, initial.environments);
  const namespaces = useResources("Namespace", {}, initial.namespaces);
  const presets = useResources("CamundaClusterPreset", {}, initial.presets);
  const releases = useResources("CamundaRelease", {}, initial.releases);
  const sections = useResources("PluginSection", {}, initial.sections);
  const registrations = useResources(
    "PluginRegistration",
    {},
    initial.registrations,
  );
  const requests = useResources("PluginRequest", {}, initial.requests);
  const storage = useResources("SecondaryStorageConfig", {}, initial.storage);
  const optimizes = useResources("CamundaOptimize", {}, initial.optimizes);
  const backupsES = useResources(
    "LogicalBackupElasticsearch",
    {},
    initial.backups.filter((b) => b.kind === "LogicalBackupElasticsearch"),
  );
  const backupsRDBMS = useResources(
    "LogicalBackupRDBMS",
    {},
    initial.backups.filter((b) => b.kind === "LogicalBackupRDBMS"),
  );
  const planes = usePlanes(initial.planes);
  const { pushes, settle } = useGitPushes();
  const clock = useClock(now);

  const sources: FleetSources = useMemo(
    () => ({
      clusters: clusters.items,
      claims: claims.items,
      environments: environments.items,
      namespaces: namespaces.items,
      presets: presets.items,
      releases: releases.items,
      sections: sections.items,
      registrations: registrations.items,
      requests: requests.items,
      storage: storage.items,
      optimizes: optimizes.items,
      backups: [...backupsES.items, ...backupsRDBMS.items],
    }),
    [
      clusters.items,
      claims.items,
      environments.items,
      namespaces.items,
      presets.items,
      releases.items,
      sections.items,
      registrations.items,
      requests.items,
      storage.items,
      optimizes.items,
      backupsES.items,
      backupsRDBMS.items,
    ],
  );
  const watched = [
    ["CamundaCluster", clusters],
    ["CamundaClusterClaim", claims],
    ["CamundaClusterPreset", presets],
    ["CamundaRelease", releases],
    ["PluginSection", sections],
    ["PluginRegistration", registrations],
    ["PluginRequest", requests],
    ["SecondaryStorageConfig", storage],
    ["CamundaOptimize", optimizes],
    ["LogicalBackupElasticsearch", backupsES],
    ["LogicalBackupRDBMS", backupsRDBMS],
    ["Environment", environments],
    ["Namespace", namespaces],
  ] as const;
  const liveErrors = [
    ...watched
      .filter(([kind]) => kind !== "Environment" && kind !== "Namespace")
      .flatMap(([kind, result]) =>
        result.error ? [{ kind, message: result.error }] : [],
      ),
    ...planes.errors,
  ];
  const answered = useAnswered(watched.map(([kind]) => kind));
  const firstErrors = (initial.readErrors ?? []).filter(
    (e) =>
      watched.some(([kind]) => kind === e.kind) &&
      !answered.has(e.kind) &&
      !liveErrors.some((l) => l.kind === e.kind),
  );
  const errors = [...liveErrors, ...firstErrors];

  return (
    <FleetView
      sources={sources}
      planes={planes.planes}
      pushes={pushes}
      errors={errors}
      now={clock}
      onStopWaiting={settle}
    />
  );
}

// useAnswered gives the kinds whose watch sent a snapshot at least once. A
// kind stays in it after a reconnect, so a failed first read does not come
// back then.
function useAnswered(kinds: readonly KindName[]): ReadonlySet<string> {
  const fresh = useWatchesFresh(kinds.map((kind) => ({ kind, opts: {} })));
  const [answered, setAnswered] = useState<ReadonlySet<string>>(new Set());
  const added = kinds.filter((kind, i) => fresh[i] && !answered.has(kind));
  if (added.length > 0) setAnswered(new Set([...answered, ...added]));
  return answered;
}

// useClock starts at the time of the server, so that the first render in
// the browser matches the render of the server.
function useClock(initial: string): string {
  const [clock, setClock] = useState(initial);
  useEffect(() => {
    const timer = setInterval(
      () => setClock(new Date().toISOString()),
      NOW_INTERVAL_MS,
    );
    return () => clearInterval(timer);
  }, []);
  return clock;
}
