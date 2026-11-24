"use client";

import { useSearchParams } from "next/navigation";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import {
  GitWaitingLine,
  type PushedObject,
} from "@/components/fleet/git-waiting-line";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { NotRead } from "@/components/fleet/not-read";
import { PrecheckLine } from "@/components/fleet/precheck-line";
import type { ConditionLike } from "@/components/fleet/status";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { FieldDoc } from "@/generated/fielddocs";
import { useClock } from "@/components/fleet/use-clock";
import { useAnswered } from "@/lib/k8s/answered";
import {
  useResource,
  useResources,
  useWatchesFresh,
  useWatchesOverdue,
} from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import { currentReadErrors, unreadKinds } from "@/lib/k8s/unread";
import { claimClusterOf } from "@/lib/clusters/draft";
import { waitedRequests } from "@/lib/clusters/progress";
import {
  ACCESS_SECTION,
  CLAIM_LABEL,
  ENVIRONMENT_LABEL,
  REVISION_ANNOTATION,
  SECTION_LABEL,
  isManaged,
} from "@/lib/labels";
import { type SectionClaim, sectionComponents } from "@/lib/claims/sections";
import { clusterFields } from "@/lib/fields";
import type { WriteMode } from "@/lib/write/types";
import { mergeCluster } from "@/lib/merge";
import { usePartitions, usePodUsage } from "@/lib/observe/use-observe";
import { runningBackup, runningRestore } from "@/lib/policy/backups";
import { ActionSlot, rowActionsOf } from "./action-slot";
import { ActivityBanner, UnmanagedBanner } from "./banners";
import { backupRowsOf, backupStorageOf, restoreRowsOf } from "./backups";
import { BackupsTab } from "./backups-tab";
import {
  type CardId,
  componentCards,
  componentOf,
  isSectionId,
  overviewHref,
  sectionCards,
} from "./cards";
import { claimNameOf, claimView } from "./claim";
import { ClaimTab } from "./claim-tab";
import { BuiltinStorage } from "./builtin-storage";
import { ClusterComponentDetail } from "./cluster-detail";
import { ClusterHeader } from "./cluster-header";
import { ClusterWaiting } from "./cluster-waiting";
import { ComponentCards } from "./component-card";
import { ComponentSettings } from "./component-settings";
import { endpointRows } from "./endpoints";
import { EndpointsTab } from "./endpoints-tab";
import { launchLinks } from "./launch";
import { clusterOptimize } from "./optimize";
import { OverviewConditions } from "./overview-conditions";
import { RequestOverview } from "./request-detail";
import { builtinStorageOf, requestDetail } from "./requests";
import {
  clusterFieldsInputOf,
  componentSettings,
  operatorVersionOf,
  sectionSettings,
} from "./settings";
import { type ClaimActions, ClusterEditProvider } from "./edit/cluster-edit";
import { SectionComponentDetail } from "./section-detail";
import type { ClusterSlotContext, ClusterSlots } from "./slots";
import { CLUSTER_TABS, type ClusterTab, clusterTabOf } from "./tabs";
import { type Process, topologyOf } from "./topology";
import { TopologyView } from "./topology-view";

/**
 * ClusterPageData is the first snapshot of the cluster page. ref is the
 * namespace and name of the route. cluster is undefined when no cluster of
 * that name exists or its read failed. The namespaced lists hold the objects
 * of that namespace. requests holds the PluginRequests of the namespace
 * whose claim label names claimNameOf of the cluster. podSelector selects
 * the pods of the cluster by its owner label, and pods holds them.
 * backupStorage holds the objects that the Backups tab follows from the
 * cluster to its backup
 * storage and its schedules. Its databaseServerPresets are cluster-scoped.
 * optimizes holds every CamundaOptimize of the namespace, and sections and
 * registrations every PluginSection and PluginRegistration.
 * platformConfigs and managementAuthConfigs are cluster-scoped. grants and
 * elasticsearchClusters hold the Grants and ElasticsearchClusters of the
 * namespace. fieldDocs holds the help of the settings fields by field key.
 * readErrors names each kind whose first read failed: its list is then
 * empty.
 */
export type ClusterPageData = {
  ref: { namespace: string; name: string };
  cluster?: Resource<"CamundaCluster">;
  namespace?: Resource<"Namespace">;
  environments: Resource<"Environment">[];
  claims: Resource<"CamundaClusterClaim">[];
  presets: Resource<"CamundaClusterPreset">[];
  releases: Resource<"CamundaRelease">[];
  storage: Resource<"SecondaryStorageConfig">[];
  requests: Resource<"PluginRequest">[];
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
  platformConfigs: Resource<"CamundaPlatformConfig">[];
  managementAuthConfigs: Resource<"ManagementAuthConfig">[];
  grants: Resource<"Grant">[];
  elasticsearchClusters: Resource<"ElasticsearchCluster">[];
  fieldDocs: Record<string, FieldDoc>;
  readErrors?: { kind: string; message: string }[];
};

/**
 * ClusterView is the cluster page. It follows the objects of initial live,
 * and selects the tab that the tab query parameter of the URL names. A
 * click on a tab replaces that parameter. The component query parameter
 * opens the detail of a used component, of an app embedded in one, or of a
 * selection of the claim, on the Overview tab, in place of the cards. slots
 * fill the action slots of a managed cluster, and an unmanaged cluster
 * ignores them. A render function of slots gets the slot context of the
 * page, and the render function of cluster.backups.row also gets each
 * backup row. For a managed cluster with a
 * claim, a filled cluster.claim.editor slot shows in place of the read view
 * of the claim. writeMode is the write mode of the fleet manager, undefined
 * when the page could not read it. claimActions turn on the edit host of a
 * managed cluster, which the slots reach through useClusterEdit. pushedClaim
 * is the claim file that the fleet manager pushed in Git mode. A precheck
 * refusal of the claim shows under the header.
 *
 * Without a cluster, the page of a claim of the route name shows the claim:
 * its section cards, its conditions, and what it waits for. Its Claim tab
 * and slots get claimClusterOf of the claim as the cluster, with drafted
 * set. With neither, or while the cluster could not be read, the page shows
 * only the read errors, and names the cluster or claim kind that it could
 * not read, or says that the cluster no longer exists.
 */
export function ClusterView({
  initial,
  now,
  slots,
  pushedClaim,
  writeMode,
  claimActions,
}: {
  initial: ClusterPageData;
  now: string;
  slots?: ClusterSlots;
  pushedClaim?: PushedObject;
  writeMode?: WriteMode;
  claimActions?: ClaimActions;
}) {
  const { namespace: ns, name } = initial.ref;
  const params = useSearchParams();
  const urlTab = clusterTabOf(params.get("tab") ?? undefined);
  const [tab, setTab] = useState<ClusterTab>(urlTab);
  const [seenUrlTab, setSeenUrlTab] = useState(urlTab);
  if (urlTab !== seenUrlTab) {
    setSeenUrlTab(urlTab);
    setTab(urlTab);
  }
  const overviewRef = useRef<HTMLDivElement>(null);
  const focusNext = useRef<string | null>(null);
  const urlComponent = params.get("component");
  const [component, setComponent] = useState(urlComponent);
  const [seenUrlComponent, setSeenUrlComponent] = useState(urlComponent);
  if (urlComponent !== seenUrlComponent) {
    setSeenUrlComponent(urlComponent);
    setComponent(urlComponent);
  }

  const cluster = useResource(
    "CamundaCluster",
    { namespace: ns, name },
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
  const claimName = claimNameOf(cluster.item, name);
  const requestsOpts = {
    namespace: ns,
    labelSelector: `${CLAIM_LABEL}=${claimName ?? ""}`,
  };
  const requests = useResources(
    "PluginRequest",
    requestsOpts,
    initial.requests,
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
  const platformConfigs = useResources(
    "CamundaPlatformConfig",
    {},
    initial.platformConfigs,
  );
  const managementAuthConfigs = useResources(
    "ManagementAuthConfig",
    {},
    initial.managementAuthConfigs,
  );
  const grants = useResources("Grant", { namespace: ns }, initial.grants);
  const elasticsearchClusters = useResources(
    "ElasticsearchCluster",
    { namespace: ns },
    initial.elasticsearchClusters,
  );
  // The detail of a section reads these lists, and the Backups tab the
  // lists after them. A list is not current when its watch answered and is
  // not fresh, or never answered and is overdue: the read of the server
  // then has no time limit.
  const sectionLists = [
    { kind: "PluginRequest", opts: requestsOpts },
    { kind: "CamundaClusterClaim", opts: { namespace: ns } },
    { kind: "PluginSection", opts: {} },
    { kind: "PluginRegistration", opts: {} },
    { kind: "Grant", opts: { namespace: ns } },
    { kind: "ElasticsearchCluster", opts: { namespace: ns } },
    { kind: "DatabaseServer", opts: { namespace: ns } },
    { kind: "SecondaryStorageConfig", opts: { namespace: ns } },
    { kind: "ObjectStorageConfig", opts: { namespace: ns } },
    { kind: "DatabaseConfig", opts: { namespace: ns } },
    { kind: "Environment", opts: {} },
  ] as const;
  const followedLists = [
    ...sectionLists,
    { kind: "LogicalBackupElasticsearch", opts: { namespace: ns } },
    { kind: "LogicalBackupRDBMS", opts: { namespace: ns } },
    { kind: "LogicalRestoreElasticsearch", opts: { namespace: ns } },
    { kind: "LogicalRestoreRDBMS", opts: { namespace: ns } },
    { kind: "PointInTimeRestore", opts: { namespace: ns } },
  ] as const;
  const listFresh = useWatchesFresh(followedLists);
  const listOverdue = useWatchesOverdue(followedLists);
  const clusterList = [
    { kind: "CamundaCluster", opts: { namespace: ns, name } },
  ] as const;
  const [clusterFresh] = useWatchesFresh(clusterList);
  const [clusterOverdue] = useWatchesOverdue(clusterList);
  const answered = useAnswered([
    ...followedLists,
    { kind: "CamundaCluster", opts: { namespace: ns, name } },
    { kind: "Namespace", opts: { name: ns } },
    "CamundaClusterPreset",
    "CamundaRelease",
    {
      kind: "Pod",
      opts: { namespace: ns, labelSelector: initial.podSelector },
    },
    { kind: "DatabaseServerConfig", opts: { namespace: ns } },
    "DatabaseServerPreset",
    { kind: "BackupSchedule", opts: { namespace: ns } },
    { kind: "CamundaOptimize", opts: { namespace: ns } },
    "CamundaPlatformConfig",
    "ManagementAuthConfig",
  ]);
  useEffect(() => {
    const selector = focusNext.current;
    if (!selector) return;
    focusNext.current = null;
    overviewRef.current?.querySelector<HTMLElement>(selector)?.focus();
  }, [component]);
  const usage = usePodUsage(ns, initial.podSelector);
  const partitions = usePartitions(ns, name);
  const backups = useMemo(
    () => [...backupsES.items, ...backupsRDBMS.items],
    [backupsES.items, backupsRDBMS.items],
  );
  const clock = useClock(
    now,
    tab === "backups" || tab === "overview" || tab === "claim",
  );

  // The claim check of the edit page reads these, and the page renders
  // each second for its clock.
  const claimRequests = useMemo(
    () =>
      requests.items.filter(
        (r) =>
          r.spec.claimRef.kind === "CamundaClusterClaim" &&
          r.spec.claimRef.name === claimName,
      ),
    [requests.items, claimName],
  );

  const live = cluster.item;
  const liveErrors = (
    [
      ["CamundaCluster", cluster],
      ["Namespace", namespaceObject],
      ["Environment", environments],
      ["CamundaClusterPreset", presets],
      ["CamundaRelease", releases],
      ["CamundaClusterClaim", claims],
      ["SecondaryStorageConfig", storage],
      ["PluginRequest", requests],
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
      ["CamundaPlatformConfig", platformConfigs],
      ["ManagementAuthConfig", managementAuthConfigs],
      ["Grant", grants],
      ["ElasticsearchCluster", elasticsearchClusters],
    ] as const
  ).flatMap(([kind, result]) =>
    result.error ? [{ kind, message: result.error }] : [],
  );
  const errors = currentReadErrors(liveErrors, initial.readErrors, answered);
  const requestsError = errors.find((e) => e.kind === "PluginRequest")?.message;
  const stale = followedLists
    .filter(
      ({ kind }, i) => !listFresh[i] && (answered.has(kind) || listOverdue[i]),
    )
    .map(({ kind }) => kind);
  // A watch keeps the last claim when it loses its connection, so the claim
  // can be there and still not be current.
  const claimUnread =
    claimName !== undefined &&
    [...stale, ...errors.map((e) => e.kind)].includes("CamundaClusterClaim");

  const claim = claims.items.find((c) => c.metadata?.name === claimName);
  const environmentName =
    namespaceObject.item?.metadata?.labels?.[ENVIRONMENT_LABEL];
  const environmentObject =
    environmentName === undefined
      ? undefined
      : environments.items.find((e) => e.metadata?.name === environmentName);
  // A watch that lost its connection keeps its last answer, so a cluster
  // can exist while the page still has none.
  const clusterStale =
    !clusterFresh && (answered.has("CamundaCluster") || clusterOverdue);
  const clusterUnread =
    clusterStale || errors.some((e) => e.kind === "CamundaCluster");
  const drafted =
    !live && claim && !clusterUnread
      ? claimClusterOf(
          { name, namespace: ns, spec: claim.spec },
          environmentObject as unknown as Environment | undefined,
        )
      : undefined;
  const subject = live ?? drafted;
  if (!subject) {
    const objectUnread = unreadKinds({
      errors,
      stale: clusterStale ? [...stale, "CamundaCluster"] : stale,
    }).filter(
      (kind) => kind === "CamundaCluster" || kind === "CamundaClusterClaim",
    );
    return (
      <div className="flex flex-col gap-4">
        {errors.map((e, i) => (
          <p key={i} role="alert" className="text-sm text-bad">
            Could not read {e.kind}: {e.message}
          </p>
        ))}
        {objectUnread.length > 0 ? (
          <NotRead kinds={objectUnread} />
        ) : (
          <p className="text-sm text-muted">This cluster no longer exists.</p>
        )}
      </div>
    );
  }

  const managed = isManaged(subject);
  const access = claimRequests.find(
    (r) => r.metadata?.labels?.[SECTION_LABEL] === ACCESS_SECTION,
  );
  const preset = presets.items.find(
    (p) => p.metadata?.name === subject.spec.presetRef,
  );
  const release = releases.items.find(
    (r) => r.metadata?.name === subject.spec.releaseRef,
  );
  const merged = mergeCluster(preset?.spec, release?.spec, subject.spec);

  const environment =
    environmentName === undefined
      ? undefined
      : (environmentObject?.spec?.displayName ?? environmentName);
  const storageObject = storage.items.find(
    (s) => s.metadata?.name === subject.spec.storageRef,
  );
  const storageType = storageObject?.spec.type;
  const ready = (live ?? claim)?.status?.conditions?.find(
    (c) => c.type === "Ready",
  ) as ConditionLike | undefined;
  const clusterConditions = conditionRows(live?.status?.conditions);
  const optimize = clusterOptimize(name, optimizes.items);
  const sectionList = claim
    ? sectionComponents({
        // A claim holds the parameters of a selection as unknown, because the
        // CRD schema does not give them a shape. sectionComponents reads no
        // parameter.
        claim: claim as SectionClaim,
        sections: sections.items,
        requests: claimRequests,
      })
    : [];
  const cards = [
    ...(live
      ? componentCards({
          spec: merged.spec,
          pods: pods.items,
          conditions: clusterConditions,
          optimize,
          usage,
          partitions,
        })
      : []),
    ...sectionCards(
      sectionList,
      requestsError,
      (stale.includes("PluginRequest") || claimUnread) &&
        requestsError === undefined,
    ),
  ];
  const processConditions: Partial<Record<Process, ConditionLike>> =
    Object.fromEntries(
      cards.flatMap((c) =>
        c.id !== "optimize" && !isSectionId(c.id) && c.condition
          ? [[c.id, c.condition]]
          : [],
      ),
    );

  const restores = [
    ...restoresES.items,
    ...restoresRDBMS.items,
    ...restoresPITR.items,
  ];
  const backupStorage = backupStorageOf({
    cluster: subject,
    spec: merged.spec,
    storage: storage.items,
    objectStorage: objectStorage.items,
    databaseConfigs: databaseConfigs.items,
    databaseServerConfigs: databaseServerConfigs.items,
    databaseServers: databaseServers.items,
    databaseServerPresets: databaseServerPresets.items,
    schedules: schedules.items,
    now: clock,
  });
  const fields = clusterFields(
    clusterFieldsInputOf({
      cluster: subject,
      merged,
      claim,
      sections: sections.items,
      registrations: registrations.items,
      optimize,
      schedules: schedules.items,
    }),
  );

  const selectTab = (value: string) => {
    const next = clusterTabOf(value);
    setTab(next);
    setComponent(null);
    window.history.replaceState(null, "", `?tab=${next}`);
  };
  const openComponent = (id: CardId | null) => {
    focusNext.current = id
      ? "h2"
      : openId
        ? `a[href="${overviewHref(openId)}"]`
        : null;
    setTab("overview");
    setComponent(id);
    window.history.replaceState(null, "", overviewHref(id));
  };
  const openId = componentOf(component, cards);
  const openSection = sectionList.find((s) => s.id === openId);
  const unread = unreadKinds({ errors, stale });
  const requestsStale = unread.includes("PluginRequest");
  const context: ClusterSlotContext = {
    cluster: subject,
    claim,
    merged,
    environment: environmentObject,
    environments: environments.items,
    presets: presets.items,
    releases: releases.items,
    platformConfigs: platformConfigs.items,
    managementAuthConfigs: managementAuthConfigs.items,
    sections: sections.items,
    registrations: registrations.items,
    requests: claimRequests,
    storage: storageObject,
    backups: live ? backups : [],
    restores: live ? restores : [],
    backupStorage,
    schedules: schedules.items,
    optimize,
    fields,
    writeMode,
    openId,
    now: clock,
    unread,
    ...(drafted && { drafted: true }),
  };
  const slot = (
    slotName: Parameters<typeof ActionSlot>[0]["name"],
  ): ReactNode => (
    <ActionSlot
      name={slotName}
      slots={slots}
      managed={managed}
      context={context}
    />
  );

  const sectionDetail =
    openSection && claim
      ? requestDetail({
          component: openSection,
          claim,
          cluster: name,
          sections: sections.items,
          registrations: registrations.items,
          components: sectionList,
          grants: grants.items,
          contracts: {
            SecondaryStorageConfig: storage.items,
            ObjectStorageConfig: objectStorage.items,
            DatabaseConfig: databaseConfigs.items,
          },
        })
      : undefined;
  const builtin =
    openSection?.request &&
    builtinStorageOf(
      openSection.request,
      elasticsearchClusters.items,
      databaseServers.items,
    );
  const sectionOverview = (builtin || sectionDetail) && (
    <div className="flex flex-col gap-4">
      {builtin && (
        <BuiltinStorage storage={builtin} unread={unread} now={clock} />
      )}
      {sectionDetail && (
        <RequestOverview detail={sectionDetail} unread={unread} now={clock} />
      )}
    </div>
  );
  const claimSection =
    openSection && claim
      ? claimView({
          claim,
          cluster: subject,
          merged,
          environments: environments.items,
          sections: sections.items,
          registrations: registrations.items,
        }).sections.find((s) => s.id === openSection.id)
      : undefined;
  const sectionBlocks = claimSection ? sectionSettings(claimSection) : [];
  const settingsUnread = unread.filter((kind) => SETTINGS_KINDS.includes(kind));

  const content = (
    <div className="flex flex-col gap-4">
      {errors.map((e, i) => (
        <p key={i} role="alert" className="text-sm text-bad">
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
      {live && (
        <ActivityBanner
          backup={runningBackup(live, backups)}
          restore={runningRestore(live, restores)}
          backupsHref="?tab=backups"
          onOpenBackups={() => selectTab("backups")}
        />
      )}
      <ClusterHeader
        name={name}
        namespace={ns}
        ready={ready}
        environment={environment}
        version={live?.status?.management?.version}
        preset={subject.spec.presetRef}
        storage={storageType}
        links={
          live ? launchLinks(live.spec.externalUrl, access?.status?.links) : []
        }
        actions={slot("cluster.header.actions")}
      />
      {claim && !claimUnread && (
        <PrecheckLine
          claim={claim}
          href={tab === "claim" ? undefined : "?tab=claim"}
        />
      )}
      <Tabs value={tab} onValueChange={selectTab}>
        <TabsList>
          {CLUSTER_TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="overview" ref={overviewRef}>
          {openId ? (
            <div className="flex flex-col gap-3">
              <a
                href={overviewHref(null)}
                onClick={(event) => {
                  event.preventDefault();
                  openComponent(null);
                }}
                className="w-fit text-sm text-text-2 underline-offset-4 hover:text-text hover:underline"
              >
                All components
              </a>
              {isSectionId(openId) ? (
                openSection && (
                  <SectionComponentDetail
                    component={openSection}
                    registrations={registrations.items}
                    namespace={ns}
                    requestsError={requestsError}
                    requestsStale={requestsStale}
                    settings={
                      <div className="flex flex-col gap-4">
                        {settingsUnread.length > 0 ? (
                          <NotRead kinds={settingsUnread} />
                        ) : (
                          <>
                            <ComponentSettings
                              groups={sectionBlocks}
                              docs={initial.fieldDocs}
                              operatorVersion={operatorVersionOf(
                                registrations.items,
                              )}
                              cluster={{ namespace: ns, name }}
                            />
                            {slot("component.settings.editor")}
                          </>
                        )}
                      </div>
                    }
                    now={clock}
                  >
                    {sectionOverview}
                  </SectionComponentDetail>
                )
              ) : (
                <ClusterComponentDetail
                  id={openId}
                  cards={cards}
                  cluster={subject}
                  namespace={ns}
                  podSelector={initial.podSelector}
                  conditions={clusterConditions}
                  optimize={optimize}
                  partitions={partitions}
                  now={clock}
                  settings={
                    <div className="flex flex-col gap-4">
                      <ComponentSettings
                        groups={componentSettings(fields, openId, merged.spec)}
                        docs={initial.fieldDocs}
                        operatorVersion={operatorVersionOf(registrations.items)}
                        cluster={{ namespace: ns, name }}
                      />
                      {slot("component.settings.editor")}
                    </div>
                  }
                />
              )}
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              {live ? (
                <TopologyView
                  boxes={topologyOf(merged.spec, pods.items)}
                  actions={slot("cluster.topology.actions")}
                  conditions={processConditions}
                />
              ) : (
                claim && (
                  <ClusterWaiting
                    // A claim holds the parameters of a selection as
                    // unknown, and waitedRequests reads no parameter.
                    waits={waitedRequests(
                      claim as unknown as CamundaClusterClaim,
                      sections.items as unknown as PluginSection[],
                      claimRequests as unknown as PluginRequest[],
                    )}
                    deleting={Boolean(claim.metadata?.deletionTimestamp)}
                    unread={unread.filter((kind) => WAIT_KINDS.includes(kind))}
                  />
                )
              )}
              <ComponentCards
                cards={cards}
                hrefOf={overviewHref}
                onOpen={openComponent}
              />
              {/* Without a cluster, ClusterWaiting already names the claim kind. */}
              {claimUnread && live && (
                <NotRead kinds={["CamundaClusterClaim"]} />
              )}
              <OverviewConditions
                cluster={clusterConditions}
                optimize={
                  optimize
                    ? conditionRows(optimize.status?.conditions)
                    : undefined
                }
                claim={
                  claim && !claimUnread
                    ? conditionRows(claim.status?.conditions)
                    : undefined
                }
                now={clock}
              />
            </div>
          )}
        </TabsContent>
        <TabsContent value="backups">
          {live ? (
            <BackupsTab
              storage={backupStorage}
              backups={backupRowsOf(live, backups, clock)}
              restores={restoreRowsOf(live, restores)}
              actions={slot("cluster.backups.actions")}
              rowActions={rowActionsOf(slots, managed, context)}
              unread={unread}
            />
          ) : (
            <p className="text-sm text-muted">
              The cluster does not exist yet, so it has no backups.
            </p>
          )}
        </TabsContent>
        <TabsContent value="endpoints">
          {live ? (
            <EndpointsTab
              rows={endpointRows({
                cluster: live,
                spec: merged.spec,
                optimizes: optimizes.items,
                accessOutputs: access?.status?.outputs,
              })}
            />
          ) : (
            <EndpointsTab
              rows={[]}
              emptyText="The cluster does not exist yet, so it has no endpoints."
            />
          )}
        </TabsContent>
        <TabsContent value="claim">
          {claimUnread ? (
            <NotRead kinds={["CamundaClusterClaim"]} />
          ) : managed &&
            claim &&
            slots?.["cluster.claim.editor"] !== undefined ? (
            slot("cluster.claim.editor")
          ) : (
            <div className="flex flex-col gap-6">
              {claim ? (
                <ClaimTab
                  view={claimView({
                    claim,
                    cluster: subject,
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
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
  return managed && claimActions ? (
    <ClusterEditProvider context={context} actions={claimActions}>
      {content}
    </ClusterEditProvider>
  ) : (
    content
  );
}

// WAIT_KINDS are the kinds of the lists that waitedRequests reads.
const WAIT_KINDS: readonly string[] = [
  "CamundaClusterClaim",
  "PluginSection",
  "PluginRequest",
];

// SETTINGS_KINDS are the kinds of the lists that claimView reads.
const SETTINGS_KINDS: readonly string[] = [
  "CamundaClusterClaim",
  "PluginSection",
  "PluginRegistration",
  "Environment",
];

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
