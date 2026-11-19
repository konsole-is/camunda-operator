// This module runs in the browser and on the server.
import type { DeleteTarget } from "@/components/cluster/delete-cluster-model";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { type DeletionGroup, deletionGroups } from "@/lib/claims/deletion";
import type { SectionClaim } from "@/lib/claims/sections";
import type { Resource } from "@/lib/k8s/kinds";
import type { ReadError } from "@/lib/k8s/unread";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  claimDeletionChoice,
  type DeletionChoice,
  type DeletionPolicy,
  type PluginSelection,
} from "@/lib/policy";
import { contractReaders } from "./plane-optimize";
import { servedClusters } from "./served-clusters";

/**
 * DeletePlaneData is what the delete page of a plane reads. plane is
 * undefined once the CamundaManagementCluster is gone, or when its read
 * failed. namespace is the namespace of the claim, undefined when its read
 * failed; the model takes the Environment that its label names. requests
 * are the PluginRequests of the namespace; the model takes those that the
 * claim controls. optimizes are every CamundaOptimize, for the readers of
 * the contract. errors holds each read that failed.
 */
export type DeletePlaneData = {
  claim: Resource<"CamundaManagementClusterClaim">;
  plane?: Resource<"CamundaManagementCluster">;
  namespace?: Resource<"Namespace">;
  environments: Resource<"Environment">[];
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
  requests: Resource<"PluginRequest">[];
  optimizes: Resource<"CamundaOptimize">[];
  errors: ReadError[];
};

/** ObjectRef names one object in a namespace. */
export type ObjectRef = { namespace: string; name: string };

/**
 * DeletePlaneModel is what the delete page of a plane shows for a pick.
 * choice is the deletion choice of the claim with policy set to the pick.
 * keepRefused says that the claim keeps its resources and an option
 * cannot. targets are the objects that always go, in the order in which
 * they go: the CamundaManagementCluster when the claim controls it, the
 * requests, and the claim. kept holds a CamundaManagementCluster of the
 * name that the claim does not control. served are the clusters that a
 * plane that goes serves, contract its ManagementAuthConfig, and readers
 * the CamundaOptimize objects that read it. served and contract are unset
 * while the page cannot read the plane, and readers while it cannot read
 * the CamundaOptimize objects. ownKeycloak says that a plane that goes
 * runs its own Keycloak, in the keycloak mode of spec.identityProvider.
 * unread are the kinds that the page could not read; fatesKnown is false
 * when the fates of groups depend on one.
 */
export type DeletePlaneModel = {
  name: string;
  namespace: string;
  choice: DeletionChoice;
  keepRefused: boolean;
  targets: DeleteTarget[];
  kept: DeleteTarget[];
  groups: DeletionGroup[];
  served?: ObjectRef[];
  contract?: string;
  readers?: ObjectRef[];
  ownKeycloak: boolean;
  unread: string[];
  fatesKnown: boolean;
};

const FATE_KINDS: readonly string[] = [
  "PluginRequest",
  "PluginRegistration",
  "PluginSection",
];

/** KEPT_PLANE is the note of a CamundaManagementCluster that the delete keeps. */
export const KEPT_PLANE = "Kept: the claim did not make it";

/**
 * planeDeletionChoice is claimDeletionChoice of the claim of data, while the
 * kinds of unread are not read.
 */
export function planeDeletionChoice(
  data: DeletePlaneData,
  unread: readonly string[],
): DeletionChoice {
  const { claim } = data;
  const name = data.namespace?.metadata?.labels?.[ENVIRONMENT_LABEL];
  const environment =
    name === undefined
      ? undefined
      : data.environments.find((e) => e.metadata?.name === name);
  // The cache objects differ from the generated types only in their
  // timestamps, which claimDeletionChoice does not read.
  return claimDeletionChoice({
    claim: {
      ...claim,
      spec: {
        ...claim.spec,
        plugins: claim.spec?.plugins as PluginSelection[] | undefined,
      },
    },
    environment: environment as unknown as Environment | undefined,
    sections: data.sections as unknown as PluginSection[],
    registrations: data.registrations as unknown as PluginRegistration[],
    registrationsRead: !unread.includes("PluginRegistration"),
    kind: "CamundaManagementClusterClaim",
  });
}

/**
 * deletePlaneModel returns the model of the delete of the plane of data
 * under picked, while the kinds of unread are not read. The plane has the
 * name of its claim. The claim deletes the plane only when it controls it,
 * then the requests that it controls.
 */
export function deletePlaneModel(
  data: DeletePlaneData,
  picked: DeletionPolicy,
  unread: readonly string[] = [],
): DeletePlaneModel {
  const { claim } = data;
  const name = claim.metadata?.name ?? "";
  const namespace = claim.metadata?.namespace ?? "";
  const choice = planeDeletionChoice(data, unread);
  const claimUid = claim.metadata?.uid;
  const controlled = (o: {
    metadata?: { ownerReferences?: { controller?: boolean; uid: string }[] };
  }) =>
    claimUid !== undefined &&
    (o.metadata?.ownerReferences ?? []).some(
      (r) => r.controller === true && r.uid === claimUid,
    );
  // The operator deletes the requests that the claim controls, whatever
  // their labels.
  const requests = data.requests.filter(controlled);
  const groups = deletionGroups({
    claim: claim as unknown as SectionClaim,
    sections: data.sections,
    registrations: data.registrations,
    requests,
    policy: picked,
  });
  const planeUnread = unread.includes("CamundaManagementCluster");
  // status.managementCluster can lag the live plane, so it counts only
  // while the page has no plane.
  const planeGoes = data.plane
    ? controlled(data.plane)
    : claim.status?.managementCluster?.name === name;
  const plane = planeGoes ? data.plane : undefined;
  const contract = planeUnread
    ? undefined
    : plane?.status?.managementAuthConfig || undefined;
  const target = (kind: DeleteTarget["kind"], objectName: string) => ({
    kind,
    name: objectName,
    label: `${kind} ${objectName}`,
  });
  const targets: DeleteTarget[] = [
    ...(planeGoes ? [target("CamundaManagementCluster", name)] : []),
    ...requests.map((r) => {
      const requestName = r.metadata?.name ?? "";
      const section =
        claim.status?.requests?.find((s) => s.name === requestName)?.section ??
        r.spec?.section;
      const label = groups.find((g) => g.section === section)?.label;
      return {
        ...target("PluginRequest", requestName),
        ...(label && { label: `${label} request ${requestName}` }),
      };
    }),
    target("CamundaManagementClusterClaim", name),
  ];
  const readersKnown = !unread.includes("CamundaOptimize");
  return {
    name,
    namespace,
    choice: { ...choice, policy: picked },
    keepRefused: choice.policy === "Orphan" && !choice.keepAllowed,
    targets,
    kept:
      data.plane && !planeGoes
        ? [
            {
              ...target("CamundaManagementCluster", name),
              note: KEPT_PLANE,
            },
          ]
        : [],
    groups,
    ...(!planeUnread && {
      served: plane
        ? servedClusters({ plane, namespaces: [], environments: [] })
            .filter((c) => c.attached)
            .map(({ namespace, name }) => ({ namespace, name }))
        : [],
    }),
    ...(contract && { contract }),
    ...(contract &&
      readersKnown && {
        readers: contractReaders(data.optimizes, contract).map((o) => ({
          namespace: o.metadata?.namespace ?? "",
          name: o.metadata?.name ?? "",
        })),
      }),
    ownKeycloak: plane?.spec?.identityProvider?.keycloak != null,
    unread: [...unread],
    fatesKnown: !unread.some((k) => FATE_KINDS.includes(k)),
  };
}
