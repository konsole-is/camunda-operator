"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { UnmanagedBanner } from "@/components/cluster/banners";
import { ComponentCards } from "@/components/cluster/component-card";
import { EndpointsTab } from "@/components/cluster/endpoints-tab";
import type { ClaimActions } from "@/components/fleet/claim-edit";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { NotRead, notReadText } from "@/components/fleet/not-read";
import type { ConditionLike } from "@/components/fleet/status";
import { useClock } from "@/components/fleet/use-clock";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAnswered } from "@/lib/k8s/answered";
import {
  useResource,
  useResources,
  useWatchesFresh,
  useWatchesOverdue,
} from "@/lib/k8s/hooks";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { planeFields } from "@/lib/fields";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import {
  currentReadErrors,
  type ReadError,
  type SourceReads,
  unreadKinds,
} from "@/lib/k8s/unread";
import { CLAIM_LABEL, isManaged, MANAGEMENT_CLUSTER_LABEL } from "@/lib/labels";
import { ownerName } from "@/lib/labels/owner-name";
import { usePodUsage } from "@/lib/observe/use-observe";
import { environmentNameOf } from "@/lib/policy";
import type { WriteMode } from "@/lib/write/types";
import type { PlaneClaimTabData } from "./claim/data";
import { PlaneClaimEditProvider } from "./claim/plane-claim-edit";
import { PlaneClaimEditor } from "./claim/plane-claim-editor";
import { ContractTab } from "./contract-tab";
import { OptimizeTab } from "./optimize-tab";
import { planesOf } from "./plane";
import { PlaneActions } from "./plane-actions";
import {
  type PlaneCardId,
  planeCardHref,
  planeCardOf,
  planeCards,
} from "./plane-cards";
import { PlaneConditions } from "./plane-conditions";
import { PlaneComponentDetail } from "./plane-detail";
import { planeEndpointRows } from "./plane-endpoints";
import { contractReaders, isOidc, optimizeRows } from "./plane-optimize";
import { PlaneHeader } from "./plane-header";
import { PLANE_TABS, type PlaneTab, planeTabOf } from "./plane-tabs";
import { servedClusters } from "./served-clusters";
import { ServedClustersTab } from "./served-clusters-tab";

/**
 * PlanePageData is the first snapshot of the plane page. plane is undefined
 * for a claim-only plane, or when its read failed. claim is undefined for a
 * plane that no claim made, whose claim is not in the namespace, or whose
 * read failed. When the plane read failed, claim is the claim of that name,
 * which the page does not join to the plane. now is the time of the server.
 * optimizes holds every CamundaOptimize, or none when its read failed.
 * managementAuthConfigs holds every ManagementAuthConfig, or none when its
 * read failed. pods holds the pods of the plane in its namespace, which
 * carry MANAGEMENT_CLUSTER_LABEL with ownerName of the plane name.
 * platformConfigs and registrations hold every CamundaPlatformConfig and
 * PluginRegistration. Each of them is empty when its read failed.
 * readErrors holds the kinds whose first read failed. claimTab holds the
 * lists of the Claim tab.
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
  pods: Resource<"Pod">[];
  platformConfigs: Resource<"CamundaPlatformConfig">[];
  registrations: Resource<"PluginRegistration">[];
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
 * links to all management planes. The Overview tab shows a card for each
 * component of the plane, and the component query parameter opens the
 * detail of a card. With claimActions, a plane that a claim made gets
 * Suspend or Resume, and Delete, in its header. It needs a
 * GitPushesProvider and a QueryClientProvider above it.
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
  const overviewRef = useRef<HTMLDivElement>(null);
  const focusNext = useRef<string | null>(null);
  const urlComponent = params.get("component");
  const [component, setComponent] = useState(urlComponent);
  const [seenUrlComponent, setSeenUrlComponent] = useState(urlComponent);
  if (urlComponent !== seenUrlComponent) {
    setSeenUrlComponent(urlComponent);
    setComponent(urlComponent);
  }
  useEffect(() => {
    const selector = focusNext.current;
    if (!selector) return;
    focusNext.current = null;
    overviewRef.current?.querySelector<HTMLElement>(selector)?.focus();
  }, [component]);

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
  const inPlane = {
    namespace,
    labelSelector: `${MANAGEMENT_CLUSTER_LABEL}=${ownerName(name)}`,
  };
  const pods = useResources("Pod", inPlane, data.pods);
  const platformConfigs = useResources(
    "CamundaPlatformConfig",
    {},
    data.platformConfigs,
  );
  const registrations = useResources(
    "PluginRegistration",
    {},
    data.registrations,
  );
  const usage = usePodUsage(namespace, inPlane.labelSelector);
  const clock = useClock(data.now, tab === "overview");

  const watched = [
    ["CamundaManagementCluster", planeResult],
    ["CamundaManagementClusterClaim", claimResult],
    ["Namespace", namespaces],
    ["Environment", environments],
    ["CamundaOptimize", optimizes],
    ["ManagementAuthConfig", managementAuthConfigs],
    ["Pod", pods],
    ["CamundaPlatformConfig", platformConfigs],
    ["PluginRegistration", registrations],
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
    { kind: "Pod", opts: inPlane },
    { kind: "CamundaPlatformConfig", opts: {} },
    { kind: "PluginRegistration", opts: {} },
  ] as const;
  const answered = useAnswered(followed);
  const fresh = useWatchesFresh(followed);
  const overdue = useWatchesOverdue(followed);
  const errors = currentReadErrors(liveErrors, data.readErrors, answered);
  // Before its first answer, a list can also be the cache of an earlier visit.
  const stale = followed
    .filter(({ kind }, i) => (answered.has(kind) ? !fresh[i] : overdue[i]))
    .map(({ kind }) => kind);
  const reads: SourceReads = { errors, stale };
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
  const objectUnread = unreadOf([
    "CamundaManagementCluster",
    "CamundaManagementClusterClaim",
  ]);
  if (!plane && !live) {
    return (
      <div className="flex flex-col gap-2">
        {alerts}
        {objectUnread.length === 0 ? (
          <p className="text-sm text-muted">
            This management plane no longer exists.
          </p>
        ) : (
          <NotRead kinds={objectUnread} />
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
  const environment = environments.items.find(
    (e) => e.metadata?.name === environmentName,
  );
  const unreadAll = unreadKinds(reads);
  const cards = plane
    ? planeCards({
        plane,
        pods: pods.items,
        config: platformConfigs.items.find(
          (c) => c.metadata?.name === plane.spec.platformConfigRef,
        ),
        usage,
        unread: unreadAll,
      })
    : [];
  const openId = planeCardOf(component, cards);
  // Only the Settings tab of an open detail shows the fields. The cache
  // objects differ from the generated types only in their timestamps,
  // which planeFields does not read.
  const fields =
    claim && openId
      ? planeFields({
          claim,
          plane,
          registrations: registrations.items as unknown as PluginRegistration[],
          environment: environment as unknown as Environment | undefined,
        })
      : [];
  const actionsUnread = unreadOf([
    "CamundaManagementCluster",
    "CamundaManagementClusterClaim",
  ]);
  const endpointRows = planeEndpointRows(plane);
  const endpointsUnread = unreadOf(["CamundaManagementCluster"]);

  const selectTab = (value: string) => {
    const next = planeTabOf(value);
    setTab(next);
    setComponent(null);
    window.history.replaceState(null, "", `?tab=${next}`);
  };
  const openComponent = (id: PlaneCardId | null) => {
    focusNext.current = id
      ? "h2"
      : openId
        ? `a[href="${planeCardHref(openId)}"]`
        : null;
    setTab("overview");
    setComponent(id);
    window.history.replaceState(null, "", planeCardHref(id) ?? "?tab=overview");
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
        readyUnknown={
          unreadOf([
            plane
              ? "CamundaManagementCluster"
              : "CamundaManagementClusterClaim",
          ]).length > 0 || planeUnknown
        }
        actions={
          claim &&
          claimActions && (
            <PlaneActions
              claim={claim}
              served={summary?.attached ?? 0}
              ownKeycloak={summary?.mode === "Keycloak"}
              apply={claimActions.apply}
              preview={claimActions.preview}
              deleteHref={`/fleet/planes/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/delete`}
              writeMode={writeMode}
              unread={actionsUnread}
              now={clock}
            />
          )
        }
      />
      <PlaneClaimEditProvider
        data={data}
        plane={plane}
        claim={claim}
        namespaces={namespaces.items}
        environments={environments.items}
        optimizes={optimizes.items}
        pageUnread={unreadKinds(reads)}
        pageStale={stale}
        writeMode={writeMode}
        actions={claimActions}
      >
        <Tabs value={tab} onValueChange={selectTab}>
          <TabsList>
            {PLANE_TABS.map((t) => (
              <TabsTrigger key={t.value} value={t.value}>
                {t.label}
              </TabsTrigger>
            ))}
          </TabsList>
          <TabsContent value="overview" ref={overviewRef}>
            {plane && openId ? (
              <div className="flex flex-col gap-3">
                <a
                  href={planeCardHref(null)}
                  onClick={(event) => {
                    event.preventDefault();
                    openComponent(null);
                  }}
                  className="w-fit text-sm text-text-2 underline-offset-4 hover:text-text hover:underline"
                >
                  All components
                </a>
                <PlaneComponentDetail
                  id={openId}
                  cards={cards}
                  plane={plane}
                  podSelector={inPlane.labelSelector}
                  fields={fields}
                  unread={unreadAll}
                  now={clock}
                />
              </div>
            ) : (
              <div className="flex flex-col gap-4">
                {!plane && objectUnread.length === 0 && (
                  <p className="text-sm text-muted">
                    The claim controller has not created the management plane
                    yet.
                  </p>
                )}
                {!plane && objectUnread.length > 0 && !planeUnknown && (
                  <NotRead kinds={objectUnread} />
                )}
                {planeUnknown && (
                  <p className="text-sm text-muted">
                    The page could not read the management plane, so it cannot
                    tell if the claim {name} made it.
                  </p>
                )}
                {plane && (
                  <>
                    <NotRead
                      tab
                      kinds={unreadOf([
                        "CamundaManagementCluster",
                        "Pod",
                        "CamundaPlatformConfig",
                      ])}
                    />
                    <ComponentCards<PlaneCardId>
                      cards={cards}
                      hrefOf={planeCardHref}
                      onOpen={openComponent}
                    />
                  </>
                )}
                <PlaneConditions
                  plane={conditionRows(plane?.status?.conditions)}
                  claim={conditionRows(
                    (planeUnknown ? live : claim)?.status?.conditions,
                  )}
                  now={clock}
                />
              </div>
            )}
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
                  {notReadText(endpointsUnread, true)}
                </p>
              )}
              <EndpointsTab
                rows={endpointRows}
                emptyText={
                  endpointsUnread.length > 0
                    ? notReadText(endpointsUnread, true)
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
