"use client";

import { type ReactNode, useState } from "react";
import {
  GitWaitingLine,
  type PushedObject,
} from "@/components/fleet/git-waiting-line";
import type { ConditionLike } from "@/components/fleet/status";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useResource, useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import {
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
import { ClusterHeader } from "./cluster-header";
import { ACCESS_SECTION, launchLinks } from "./launch";
import type { ActionSlots } from "./slots";
import { topologyOf } from "./topology";
import { TopologyView } from "./topology-view";

export type ClusterTab = "overview" | "backups" | "endpoints" | "claim";

export const CLUSTER_TABS: readonly { value: ClusterTab; label: string }[] = [
  { value: "overview", label: "Overview" },
  { value: "backups", label: "Backups" },
  { value: "endpoints", label: "Endpoints" },
  { value: "claim", label: "Claim" },
];

/**
 * clusterTabOf returns the tab that the tab query parameter names. Of an
 * array, the first item counts. An unknown or missing value is overview.
 */
export function clusterTabOf(value: string | string[] | undefined): ClusterTab {
  const first = Array.isArray(value) ? value[0] : value;
  return CLUSTER_TABS.find((t) => t.value === first)?.value ?? "overview";
}

/**
 * ClusterPageData is the first snapshot of the cluster page. The namespaced
 * lists hold the objects of the namespace of the cluster. accessRequests are
 * the PluginRequests of the access section. podSelector selects the pods of
 * the cluster by its owner label, and pods holds them.
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
};

/**
 * ClusterView is the cluster page. It follows the objects of initial live,
 * and opens on tab. A change of the tab replaces the tab query parameter of
 * the URL. slots fill the action slots of a managed cluster, and an
 * unmanaged cluster ignores them. pushedClaim is the claim file that the
 * fleet manager pushed in Git mode. When the cluster is deleted, the page
 * says so and shows nothing else.
 */
export function ClusterView({
  initial,
  tab: initialTab,
  now,
  slots,
  pushedClaim,
}: {
  initial: ClusterPageData;
  tab: ClusterTab;
  now: string;
  slots?: ActionSlots;
  pushedClaim?: PushedObject;
}) {
  const { namespace, name } = initial.cluster.metadata ?? {};
  const ns = namespace ?? "";
  const [tab, setTab] = useState<ClusterTab>(initialTab);

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
    ] as const
  ).flatMap(([kind, result]) =>
    result.error ? [{ kind, message: result.error }] : [],
  );

  const managed = isManaged(live);
  const claimName = live.metadata?.labels?.[CLAIM_LABEL];
  const claim = claims.items.find((c) => c.metadata?.name === claimName);
  const access = accessRequests.items.find(
    (r) => claimName && r.metadata?.labels?.[CLAIM_LABEL] === claimName,
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
        backup={runningBackup(live, [
          ...backupsES.items,
          ...backupsRDBMS.items,
        ])}
        restore={runningRestore(live, [
          ...restoresES.items,
          ...restoresRDBMS.items,
          ...restoresPITR.items,
        ])}
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
          <TopologyView
            boxes={topologyOf(merged.spec, pods.items)}
            actions={slot("cluster.topology.actions")}
          />
        </TabsContent>
        <TabsContent value="backups">
          {slot("cluster.backups.actions")}
        </TabsContent>
        <TabsContent value="endpoints" />
        <TabsContent value="claim">{slot("cluster.claim.editor")}</TabsContent>
      </Tabs>
    </div>
  );
}
