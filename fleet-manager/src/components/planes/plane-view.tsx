"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { UnmanagedBanner } from "@/components/cluster/banners";
import { EndpointsTab } from "@/components/cluster/endpoints-tab";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { ConditionLike } from "@/components/fleet/status";
import { useClock } from "@/components/fleet/use-clock";
import {
  type SourceReads,
  unreadKinds,
} from "@/components/setup/releases/rows";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useResource, useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, isManaged } from "@/lib/labels";
import { environmentNameOf } from "@/lib/policy";
import type { WriteMode } from "@/lib/write/types";
import type { ClaimActions } from "@/components/fleet/claim-edit";
import type { PlaneClaimTabData } from "./claim/data";
import { PlaneClaimEditProvider } from "./claim/plane-claim-edit";
import { PlaneClaimEditor } from "./claim/plane-claim-editor";
import { ContractTab } from "./contract-tab";
import { OptimizeTab } from "./optimize-tab";
import { planesOf } from "./plane";
import { PlaneConditions } from "./plane-conditions";
import { planeEndpointRows } from "./plane-endpoints";
import { contractReaders, isOidc, optimizeRows } from "./plane-optimize";
import { PlaneHeader } from "./plane-header";
import { PLANE_TABS, type PlaneTab, planeTabOf } from "./plane-tabs";
import type { ReadError } from "./planes-table";
import { servedClusters } from "./served-clusters";
import { ServedClustersTab } from "./served-clusters-tab";
import { unreadText } from "./unread-note";

/**
 * PlanePageData is the first snapshot of the plane page. plane is undefined
 * for a claim-only plane, or when its read failed. claim is undefined for a
 * plane that no claim made, whose claim is not in the namespace, or whose
 * read failed. When the plane read failed, claim is the claim of that name,
 * which the page does not join to the plane. now is the time of the server.
 * optimizes holds every CamundaOptimize, or none when its read failed.
 * managementAuthConfigs holds every ManagementAuthConfig, or none when its
 * read failed. readErrors holds the kinds whose first read failed.
 * claimTab holds the lists of the Claim tab.
 */
export type PlanePageData = {
  namespace: string;
  name: string;
  now: string;
  plane?: Resource<"CamundaManagementCluster">;
  claim?: Resource<"CamundaManagementClusterClaim">;
  namespaces: Resource<"Namespace">[];
  environments: Resource<"Environment">[];
  optimizes: Resource<"CamundaOptimize">[];
  managementAuthConfigs: Resource<"ManagementAuthConfig">[];
  readErrors?: ReadError[];
  claimTab?: PlaneClaimTabData;
};

/**
 * PlaneView is the plane page. It follows the CamundaManagementCluster and
 * the claim of the route name live, and shows the plane once it exists. A
 * claim belongs to the plane while no CamundaManagementCluster exists and
 * its read did not fail, or when the CamundaManagementCluster carries
 * CLAIM_LABEL with the claim name. It selects the tab that the tab query parameter names, and a click
 * on a tab replaces that parameter. writeMode is the write mode of the
 * fleet manager, undefined when the page could not read it. claimActions
 * turn on the claim edit session of the Claim tab. The page
 * names each kind that it could not read. When both objects are gone and
 * neither read failed, the page says that the plane no longer exists and
 * links to all management planes.
 */
export function PlaneView({
  data,
  writeMode,
  claimActions,
}: {
  data: PlanePageData;
  writeMode?: WriteMode;
  claimActions?: ClaimActions;
}) {
  const { namespace, name } = data;
  const params = useSearchParams();
  const urlTab = planeTabOf(params.get("tab") ?? undefined);
  const [tab, setTab] = useState<PlaneTab>(urlTab);
  const [seenUrlTab, setSeenUrlTab] = useState(urlTab);
  if (urlTab !== seenUrlTab) {
    setSeenUrlTab(urlTab);
    setTab(urlTab);
  }

  const planeResult = useResource(
    "CamundaManagementCluster",
    { namespace, name },
    data.plane,
  );
  const claimResult = useResource(
    "CamundaManagementClusterClaim",
    { namespace, name },
    data.claim,
  );
  const namespaces = useResources("Namespace", {}, data.namespaces);
  const environments = useResources("Environment", {}, data.environments);
  const optimizes = useResources("CamundaOptimize", {}, data.optimizes);
  const managementAuthConfigs = useResources(
    "ManagementAuthConfig",
    {},
    data.managementAuthConfigs,
  );
  const clock = useClock(data.now, tab === "overview");

  const watched = [
    ["CamundaManagementCluster", planeResult],
    ["CamundaManagementClusterClaim", claimResult],
    ["Namespace", namespaces],
    ["Environment", environments],
    ["CamundaOptimize", optimizes],
    ["ManagementAuthConfig", managementAuthConfigs],
  ] as const;
  const liveErrors = watched.flatMap(([kind, result]) =>
    result.error ? [{ kind, message: result.error }] : [],
  );
  const followed = [
    { kind: "CamundaManagementCluster", opts: { namespace, name } },
    { kind: "CamundaManagementClusterClaim", opts: { namespace, name } },
    { kind: "Namespace", opts: {} },
    { kind: "Environment", opts: {} },
    { kind: "CamundaOptimize", opts: {} },
    { kind: "ManagementAuthConfig", opts: {} },
  ] as const;
  const answered = useAnswered(followed);
  const fresh = useWatchesFresh(followed);
  const firstErrors = (data.readErrors ?? []).filter(
    (e) => !answered.has(e.kind) && !liveErrors.some((l) => l.kind === e.kind),
  );
  const errors = [...liveErrors, ...firstErrors];
  // A watch that answered once and then lost its connection keeps its last
  // items, which can be out of date. Before its first answer, the page
  // shows the read of the server.
  const reads: SourceReads = {
    errors,
    stale: followed
      .filter(({ kind }, i) => answered.has(kind) && !fresh[i])
      .map(({ kind }) => kind),
  };
  const unreadOf = (kinds: readonly KindName[]) =>
    unreadKinds(reads).filter((k) => kinds.includes(k as KindName));
  const unread = (kind: KindName) => errors.some((e) => e.kind === kind);
  const alerts = errors.map((e, i) => (
    <p key={i} role="alert" className="text-sm text-bad">
      Could not read {e.kind}: {e.message}
    </p>
  ));

  const plane = planeResult.item;
  const live = claimResult.item;
  if (!plane && !live) {
    return (
      <div className="flex flex-col gap-2">
        {alerts}
        {!unread("CamundaManagementCluster") &&
          !unread("CamundaManagementClusterClaim") && (
            <p className="text-sm text-muted">
              This management plane no longer exists.
            </p>
          )}
        <Link
          href="/fleet/planes"
          className="relative w-fit text-sm font-medium text-text underline underline-offset-4 hit-area hover:text-accent"
        >
          All management planes
        </Link>
      </div>
    );
  }

  const planeUnknown = !plane && unread("CamundaManagementCluster");
  const claim =
    live &&
    !planeUnknown &&
    (!plane || plane.metadata?.labels?.[CLAIM_LABEL] === name)
      ? live
      : undefined;
  const summary = planesOf({
    clusters: plane ? [plane] : [],
    claims: claim ? [claim] : [],
    namespaces: namespaces.items,
    environments: environments.items,
  })[0];
  const ready = (plane ?? claim)?.status?.conditions?.find(
    (c) => c.type === "Ready",
  ) as ConditionLike | undefined;
  const namespaceObject = namespaces.items.find(
    (n) => n.metadata?.name === namespace,
  );
  const environmentName = namespaceObject
    ? environmentNameOf(namespaceObject, environments.items)
    : undefined;
  const platformConfig = planeUnknown
    ? undefined
    : plane?.spec.platformConfigRef ||
      claim?.spec.platformConfigRef ||
      environments.items.find((e) => e.metadata?.name === environmentName)?.spec
        .platformConfigRef;

  const contractName = textOf(plane?.status?.managementAuthConfig);
  const oidc = isOidc(plane);
  const endpointRows = planeEndpointRows(plane);
  const endpointsUnread = unreadOf(["CamundaManagementCluster"]);

  const selectTab = (value: string) => {
    const next = planeTabOf(value);
    setTab(next);
    window.history.replaceState(null, "", `?tab=${next}`);
  };

  return (
    <div className="flex flex-col gap-4">
      {alerts}
      {plane && !isManaged(plane) && (
        <UnmanagedBanner subject="management plane" />
      )}
      <PlaneHeader
        name={name}
        namespace={namespace}
        ready={ready}
        mode={summary?.mode}
        platformConfig={platformConfig}
        serves={summary?.serves}
      />
      <PlaneClaimEditProvider
        data={data}
        plane={plane}
        claim={claim}
        namespaces={namespaces.items}
        environments={environments.items}
        pageUnread={unreadKinds(reads)}
        writeMode={writeMode}
        actions={claimActions}
      >
        <Tabs value={tab} onValueChange={selectTab}>
          <TabsList>
            {PLANE_TABS.map((t) => (
              <TabsTrigger key={t.id} value={t.id}>
                {t.label}
              </TabsTrigger>
            ))}
          </TabsList>
          <TabsContent value="overview">
            <div className="flex flex-col gap-4">
              {!plane &&
                !unread("CamundaManagementCluster") &&
                !unread("CamundaManagementClusterClaim") && (
                  <p className="text-sm text-muted">
                    The claim controller has not created the management plane
                    yet.
                  </p>
                )}
              {planeUnknown && (
                <p className="text-sm text-muted">
                  The page could not read the management plane, so it cannot
                  tell if the claim {name} made it.
                </p>
              )}
              <PlaneConditions
                plane={conditionRows(plane?.status?.conditions)}
                claim={conditionRows(
                  (planeUnknown ? live : claim)?.status?.conditions,
                )}
                now={clock}
              />
            </div>
          </TabsContent>
          <TabsContent value="clusters">
            <ServedClustersTab
              rows={servedClusters({
                plane,
                namespaces: namespaces.items,
                environments: environments.items,
                environmentsUnread:
                  unreadOf(["Namespace", "Environment"]).length > 0,
              })}
              unread={unreadOf([
                "CamundaManagementCluster",
                "Namespace",
                "Environment",
              ])}
            />
          </TabsContent>
          <TabsContent value="optimize">
            <OptimizeTab
              rows={optimizeRows({ plane, optimizes: optimizes.items })}
              callbacks={
                plane?.status?.conditions?.find(
                  (c) => c.type === "OptimizeCallbacksReady",
                ) as ConditionLike | undefined
              }
              oidc={oidc}
              unread={unreadOf(
                plane && !oidc
                  ? ["CamundaManagementCluster"]
                  : ["CamundaManagementCluster", "CamundaOptimize"],
              )}
            />
          </TabsContent>
          <TabsContent value="contract">
            <ContractTab
              name={contractName}
              contract={managementAuthConfigs.items.find(
                (c) => c.metadata?.name === contractName,
              )}
              readers={contractReaders(optimizes.items, contractName)}
              unread={unreadOf([
                "CamundaManagementCluster",
                "ManagementAuthConfig",
                "CamundaOptimize",
              ])}
            />
          </TabsContent>
          <TabsContent value="endpoints">
            <div className="flex flex-col gap-2">
              {endpointRows.length > 0 && endpointsUnread.length > 0 && (
                <p className="text-sm text-muted">
                  {unreadText(endpointsUnread)}
                </p>
              )}
              <EndpointsTab
                rows={endpointRows}
                emptyText={
                  endpointsUnread.length > 0
                    ? unreadText(endpointsUnread)
                    : "This plane reports no endpoints yet."
                }
              />
            </div>
          </TabsContent>
          <TabsContent value="claim">
            <PlaneClaimEditor />
          </TabsContent>
        </Tabs>
      </PlaneClaimEditProvider>
    </div>
  );
}

// useAnswered gives the kinds whose watch got a snapshot at least once. A
// kind stays in it after a reconnect, so a failed first read does not come
// back then.
function useAnswered(
  lists: readonly {
    kind: KindName;
    opts: { namespace?: string; name?: string };
  }[],
): ReadonlySet<string> {
  const fresh = useWatchesFresh(lists);
  const [answered, setAnswered] = useState<ReadonlySet<string>>(new Set());
  const added = lists
    .filter(({ kind }, i) => fresh[i] && !answered.has(kind))
    .map(({ kind }) => kind);
  if (added.length > 0) setAnswered(new Set([...answered, ...added]));
  return answered;
}

function textOf(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

// conditionRows gives the conditions of a status as the table rows. The
// API server sends metav1.Condition, whose status is one of the three values
// of ConditionRow.
function conditionRows(
  conditions: readonly { type: string; status: string }[] | undefined,
): ConditionRow[] | undefined {
  return conditions as ConditionRow[] | undefined;
}
