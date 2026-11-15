"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { UnmanagedBanner } from "@/components/cluster/banners";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { ConditionLike } from "@/components/fleet/status";
import { useClock } from "@/components/fleet/use-clock";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useResource, useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, isManaged } from "@/lib/labels";
import { environmentNameOf } from "@/lib/policy";
import type { WriteMode } from "@/lib/write/types";
import { planesOf } from "./plane";
import { PlaneConditions } from "./plane-conditions";
import { PlaneHeader } from "./plane-header";
import { PLANE_TABS, type PlaneTab, planeTabOf } from "./plane-tabs";

/**
 * PlanePageData is the first snapshot of the plane page. plane is undefined
 * for a claim-only plane. claim is undefined for a plane that no claim made,
 * or whose claim is not in the namespace. now is the time of the server.
 */
export type PlanePageData = {
  namespace: string;
  name: string;
  now: string;
  plane?: Resource<"CamundaManagementCluster">;
  claim?: Resource<"CamundaManagementClusterClaim">;
  namespaces: Resource<"Namespace">[];
  environments: Resource<"Environment">[];
};

/**
 * PlaneView is the plane page. It follows the CamundaManagementCluster and
 * the claim of the route name live, and shows the plane once it exists. A
 * claim belongs to the plane while no CamundaManagementCluster exists, or
 * when the CamundaManagementCluster carries CLAIM_LABEL with the claim
 * name. It selects the tab that the tab query parameter names, and a click
 * on a tab replaces that parameter. writeMode is the write mode of the
 * fleet manager, undefined when the page could not read it. When both
 * objects are gone, the page says so and shows nothing else.
 */
export function PlaneView({
  data,
}: {
  data: PlanePageData;
  writeMode?: WriteMode;
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
  const clock = useClock(data.now, tab === "overview");

  const plane = planeResult.item;
  const live = claimResult.item;
  if (!plane && !live) {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-sm text-muted">
          This management plane no longer exists.
        </p>
        <Link
          href="/fleet/planes"
          className="relative w-fit text-sm font-medium text-text underline underline-offset-4 hit-area hover:text-accent"
        >
          All management planes
        </Link>
      </div>
    );
  }

  const claim =
    live && (!plane || plane.metadata?.labels?.[CLAIM_LABEL] === name)
      ? live
      : undefined;
  const managed = plane ? isManaged(plane) : true;
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
  const platformConfig =
    plane?.spec.platformConfigRef ||
    claim?.spec.platformConfigRef ||
    environments.items.find((e) => e.metadata?.name === environmentName)?.spec
      .platformConfigRef;

  const errors = (
    [
      ["CamundaManagementCluster", planeResult],
      ["CamundaManagementClusterClaim", claimResult],
      ["Namespace", namespaces],
      ["Environment", environments],
    ] as const
  ).flatMap(([kind, result]) =>
    result.error ? [{ kind, message: result.error }] : [],
  );

  const selectTab = (value: string) => {
    const next = planeTabOf(value);
    setTab(next);
    window.history.replaceState(null, "", `?tab=${next}`);
  };

  return (
    <div className="flex flex-col gap-4">
      {errors.map((e, i) => (
        <p key={i} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {!managed && <UnmanagedBanner subject="management plane" />}
      <PlaneHeader
        name={name}
        namespace={namespace}
        ready={ready}
        mode={summary?.mode}
        platformConfig={platformConfig}
        serves={summary?.serves ?? { environments: [] }}
      />
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
            {!plane && (
              <p className="text-sm text-muted">
                The claim controller has not created the management plane yet.
              </p>
            )}
            <PlaneConditions
              plane={conditionRows(plane?.status?.conditions)}
              claim={conditionRows(claim?.status?.conditions)}
              now={clock}
            />
          </div>
        </TabsContent>
      </Tabs>
    </div>
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
