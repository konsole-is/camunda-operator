"use client";

import { useSearchParams } from "next/navigation";
import { type ReactNode, useEffect, useState } from "react";
import {
  GitWaitingLine,
  type PushedObject,
} from "@/components/fleet/git-waiting-line";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { ConditionLike } from "@/components/fleet/status";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useResource, useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import {
  ACCESS_SECTION,
  CLAIM_LABEL,
  ENVIRONMENT_LABEL,
  REVISION_ANNOTATION,
  SECTION_LABEL,
  isManaged,
} from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import { runningBackup, runningRestore } from "@/lib/policy/backups";
import { ActionSlot } from "./action-slot";
import { ActivityBanner, UnmanagedBanner } from "./banners";
import { backupRowsOf, backupStorageOf, restoreRowsOf } from "./backups";
import { BackupsTab } from "./backups-tab";
import { claimView } from "./claim";
import { ClaimTab } from "./claim-tab";
import { componentCards } from "./cards";
import { ClusterHeader } from "./cluster-header";
import { ComponentCards } from "./component-card";
import { endpointRows } from "./endpoints";
import { EndpointsTab } from "./endpoints-tab";
import { launchLinks } from "./launch";
import { clusterOptimize } from "./optimize";
import { OverviewConditions } from "./overview-conditions";
import type { ActionSlots } from "./slots";
import { CLUSTER_TABS, type ClusterTab, clusterTabOf } from "./tabs";
import { type Process, topologyOf } from "./topology";
import { TopologyView } from "./topology-view";

/**
 * ClusterPageData is the first snapshot of the cluster page. The namespaced
 * lists hold the objects of the namespace of the cluster. accessRequests are
 * the PluginRequests of the access section. podSelector selects the pods of
 * the cluster by its owner label, and pods holds them. backupStorage holds
 * the objects that the Backups tab follows from the cluster to its backup
 * storage and its schedules. Its databaseServerPresets are cluster-scoped.
 * optimizes holds every CamundaOptimize of the namespace, and sections and
 * registrations every PluginSection and PluginRegistration.
 */
export type ClusterPageData = {
  cluster: Resource<"CamundaCluster">;
  namespace?: Resource<"Namespace">;
  environments: Resource<"Environment">[];
  claims: Resource<"CamundaClusterClaim">[];
  presets: Resource<"CamundaClusterPreset">[];
  releases: Resource<"CamundaRelease">[];
  storage: Resource<"SecondaryStorageConfig">[];
  accessRequests: Resource<"PluginRequest">[];
  pods: Resource<"Pod">[];
  podSelector: string;
  backups: {
    elasticsearch: Resource<"LogicalBackupElasticsearch">[];
    rdbms: Resource<"LogicalBackupRDBMS">[];
  };
  restores: {
    elasticsearch: Resource<"LogicalRestoreElasticsearch">[];
    rdbms: Resource<"LogicalRestoreRDBMS">[];
    pointInTime: Resource<"PointInTimeRestore">[];
  };
  backupStorage: {
    objectStorage: Resource<"ObjectStorageConfig">[];
    databaseConfigs: Resource<"DatabaseConfig">[];
    databaseServerConfigs: Resource<"DatabaseServerConfig">[];
    databaseServers: Resource<"DatabaseServer">[];
    databaseServerPresets: Resource<"DatabaseServerPreset">[];
    schedules: Resource<"BackupSchedule">[];
  };
  optimizes: Resource<"CamundaOptimize">[];
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
};

/**
 * ClusterView is the cluster page. It follows the objects of initial live,
 * and selects the tab that the tab query parameter of the URL names. A
 * click on a tab replaces that parameter. slots fill the action slots of a managed cluster, and an
 * unmanaged cluster ignores them. pushedClaim is the claim file that the
 * fleet manager pushed in Git mode. When the cluster is deleted, the page
 * says so and shows nothing else.
 */
export function ClusterView({
  initial,
  now,
  slots,
  pushedClaim,
}: {
  initial: ClusterPageData;
  now: string;
  slots?: ActionSlots;
  pushedClaim?: PushedObject;
}) {
  const { namespace, name } = initial.cluster.metadata ?? {};
  const ns = namespace ?? "";
  const urlTab = clusterTabOf(useSearchParams().get("tab") ?? undefined);
  const [tab, setTab] = useState<ClusterTab>(urlTab);
  const [seenUrlTab, setSeenUrlTab] = useState(urlTab);
  if (urlTab !== seenUrlTab) {
    setSeenUrlTab(urlTab);
    setTab(urlTab);
  }

  const cluster = useResource(
    "CamundaCluster",
    { namespace: ns, name: name ?? "" },
    initial.cluster,
  );
  const namespaceObject = useResource(
    "Namespace",
    { name: ns },
    initial.namespace,
  );
  const environments = useResources("Environment", {}, initial.environments);
  const presets = useResources("CamundaClusterPreset", {}, initial.presets);
  const releases = useResources("CamundaRelease", {}, initial.releases);
  const claims = useResources(
    "CamundaClusterClaim",
    { namespace: ns },
    initial.claims,
  );
  const storage = useResources(
    "SecondaryStorageConfig",
    { namespace: ns },
    initial.storage,
  );
  const accessRequests = useResources(
    "PluginRequest",
    { namespace: ns, labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}` },
    initial.accessRequests,
  );
  const pods = useResources(
    "Pod",
    { namespace: ns, labelSelector: initial.podSelector },
    initial.pods,
  );
  const backupsES = useResources(
    "LogicalBackupElasticsearch",
    { namespace: ns },
    initial.backups.elasticsearch,
  );
  const backupsRDBMS = useResources(
    "LogicalBackupRDBMS",
    { namespace: ns },
    initial.backups.rdbms,
  );
  const restoresES = useResources(
    "LogicalRestoreElasticsearch",
    { namespace: ns },
    initial.restores.elasticsearch,
  );
  const restoresRDBMS = useResources(
    "LogicalRestoreRDBMS",
    { namespace: ns },
    initial.restores.rdbms,
  );
  const restoresPITR = useResources(
    "PointInTimeRestore",
    { namespace: ns },
    initial.restores.pointInTime,
  );
  const objectStorage = useResources(
    "ObjectStorageConfig",
    { namespace: ns },
    initial.backupStorage.objectStorage,
  );
  const databaseConfigs = useResources(
    "DatabaseConfig",
    { namespace: ns },
    initial.backupStorage.databaseConfigs,
  );
  const databaseServerConfigs = useResources(
    "DatabaseServerConfig",
    { namespace: ns },
    initial.backupStorage.databaseServerConfigs,
  );
  const databaseServers = useResources(
    "DatabaseServer",
    { namespace: ns },
    initial.backupStorage.databaseServers,
  );
  const databaseServerPresets = useResources(
    "DatabaseServerPreset",
    {},
    initial.backupStorage.databaseServerPresets,
  );
  const schedules = useResources(
    "BackupSchedule",
    { namespace: ns },
    initial.backupStorage.schedules,
  );
  const optimizes = useResources(
    "CamundaOptimize",
    { namespace: ns },
    initial.optimizes,
  );
  const sections = useResources("PluginSection", {}, initial.sections);
  const registrations = useResources(
    "PluginRegistration",
    {},
    initial.registrations,
  );
  const backups = [...backupsES.items, ...backupsRDBMS.items];
  const clock = useClock(
    now,
    tab === "backups" || tab === "overview" || tab === "claim",
  );

  const live = cluster.item;
  if (!live) {
    return <p className="text-sm text-muted">This cluster no longer exists.</p>;
  }

  const errors = (
    [
      ["CamundaCluster", cluster],
      ["Namespace", namespaceObject],
      ["Environment", environments],
      ["CamundaClusterPreset", presets],
      ["CamundaRelease", releases],
      ["CamundaClusterClaim", claims],
      ["SecondaryStorageConfig", storage],
      ["PluginRequest", accessRequests],
      ["Pod", pods],
      ["LogicalBackupElasticsearch", backupsES],
      ["LogicalBackupRDBMS", backupsRDBMS],
      ["LogicalRestoreElasticsearch", restoresES],
      ["LogicalRestoreRDBMS", restoresRDBMS],
      ["PointInTimeRestore", restoresPITR],
      ["ObjectStorageConfig", objectStorage],
      ["DatabaseConfig", databaseConfigs],
      ["DatabaseServerConfig", databaseServerConfigs],
      ["DatabaseServer", databaseServers],
      ["DatabaseServerPreset", databaseServerPresets],
      ["BackupSchedule", schedules],
      ["CamundaOptimize", optimizes],
      ["PluginSection", sections],
      ["PluginRegistration", registrations],
    ] as const
  ).flatMap(([kind, result]) =>
    result.error ? [{ kind, message: result.error }] : [],
  );

  const managed = isManaged(live);
  const claimName = live.metadata?.labels?.[CLAIM_LABEL];
  const claim = claims.items.find((c) => c.metadata?.name === claimName);
  const access = accessRequests.items.find(
    (r) =>
      r.spec.claimRef.kind === "CamundaClusterClaim" &&
      r.spec.claimRef.name === claimName,
  );
  const preset = presets.items.find(
    (p) => p.metadata?.name === live.spec.presetRef,
  );
  const release = releases.items.find(
    (r) => r.metadata?.name === live.spec.releaseRef,
  );
  const merged = mergeCluster(preset?.spec, release?.spec, live.spec);

  const environmentName =
    namespaceObject.item?.metadata?.labels?.[ENVIRONMENT_LABEL];
  const environment =
    environmentName === undefined
      ? undefined
      : (environments.items.find((e) => e.metadata?.name === environmentName)
          ?.spec?.displayName ?? environmentName);
  const storageType = storage.items.find(
    (s) => s.metadata?.name === live.spec.storageRef,
  )?.spec.type;
  const ready = live.status?.conditions?.find((c) => c.type === "Ready") as
    ConditionLike | undefined;
  const clusterConditions = conditionRows(live.status?.conditions);
  const optimize = clusterOptimize(name ?? "", optimizes.items);
  const cards = componentCards({
    spec: merged.spec,
    pods: pods.items,
    conditions: clusterConditions,
    optimize,
  });
  const processConditions: Partial<Record<Process, ConditionLike>> =
    Object.fromEntries(
      cards.flatMap((c) =>
        c.id !== "optimize" && c.condition ? [[c.id, c.condition]] : [],
      ),
    );

  const restores = [
    ...restoresES.items,
    ...restoresRDBMS.items,
    ...restoresPITR.items,
  ];

  const selectTab = (value: string) => {
    const next = clusterTabOf(value);
    setTab(next);
    window.history.replaceState(null, "", `?tab=${next}`);
  };
  const slot = (
    slotName: Parameters<typeof ActionSlot>[0]["name"],
  ): ReactNode => (
    <ActionSlot name={slotName} slots={slots} managed={managed} />
  );

  return (
    <div className="flex flex-col gap-4">
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {!managed && <UnmanagedBanner />}
      {managed && (
        <GitWaitingLine
          pushed={pushedClaim}
          liveRevision={claim?.metadata?.annotations?.[REVISION_ANNOTATION]}
          now={now}
        />
      )}
      <ActivityBanner
        backup={runningBackup(live, backups)}
        restore={runningRestore(live, restores)}
        backupsHref="?tab=backups"
        onOpenBackups={() => selectTab("backups")}
      />
      <ClusterHeader
        name={name ?? ""}
        namespace={ns}
        ready={ready}
        environment={environment}
        version={live.status?.management?.version}
        preset={live.spec.presetRef}
        storage={storageType}
        links={launchLinks(live.spec.externalUrl, access?.status?.links)}
        actions={slot("cluster.header.actions")}
      />
      <Tabs value={tab} onValueChange={selectTab}>
        <TabsList>
          {CLUSTER_TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="overview">
          <div className="flex flex-col gap-4">
            <TopologyView
              boxes={topologyOf(merged.spec, pods.items)}
              actions={slot("cluster.topology.actions")}
              conditions={processConditions}
            />
            <ComponentCards cards={cards} />
            <OverviewConditions
              cluster={clusterConditions}
              optimize={
                optimize
                  ? conditionRows(optimize.status?.conditions)
                  : undefined
              }
              claim={
                claim ? conditionRows(claim.status?.conditions) : undefined
              }
              now={clock}
            />
          </div>
        </TabsContent>
        <TabsContent value="backups">
          <BackupsTab
            storage={backupStorageOf({
              cluster: live,
              spec: merged.spec,
              storage: storage.items,
              objectStorage: objectStorage.items,
              databaseConfigs: databaseConfigs.items,
              databaseServerConfigs: databaseServerConfigs.items,
              databaseServers: databaseServers.items,
              databaseServerPresets: databaseServerPresets.items,
              schedules: schedules.items,
              now: clock,
            })}
            backups={backupRowsOf(live, backups, clock)}
            restores={restoreRowsOf(live, restores)}
            actions={slot("cluster.backups.actions")}
          />
        </TabsContent>
        <TabsContent value="endpoints">
          <EndpointsTab
            rows={endpointRows({
              cluster: live,
              spec: merged.spec,
              optimizes: optimizes.items,
              accessOutputs: access?.status?.outputs,
            })}
          />
        </TabsContent>
        <TabsContent value="claim">
          <div className="flex flex-col gap-6">
            {claim ? (
              <ClaimTab
                view={claimView({
                  claim,
                  cluster: live,
                  merged,
                  environments: environments.items,
                  sections: sections.items,
                  registrations: registrations.items,
                })}
                stored={claim}
                now={clock}
              />
            ) : (
              <NoClaim managed={managed} name={claimName} />
            )}
            {slot("cluster.claim.editor")}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function NoClaim({ managed, name }: { managed: boolean; name?: string }) {
  return (
    <p className="text-sm text-muted">
      {managed
        ? `The claim ${name} is not in this namespace.`
        : "This cluster was not made from a claim."}
    </p>
  );
}

// conditionRows gives the conditions of a status as the table rows. The
// API server sends metav1.Condition, whose status is one of the three values
// of ConditionRow.
function conditionRows(
  conditions: readonly { type: string; status: string }[] | undefined,
): ConditionRow[] | undefined {
  return conditions as ConditionRow[] | undefined;
}

// useClock starts at the time of the server, so that the first render in
// the browser matches the render of the server.
function useClock(initial: string, ticking: boolean): string {
  const [clock, setClock] = useState(initial);
  useEffect(() => {
    if (!ticking) return;
    const timer = setInterval(() => setClock(new Date().toISOString()), 1000);
    return () => clearInterval(timer);
  }, [ticking]);
  return clock;
}
