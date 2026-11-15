// This module runs in the browser and on the server.
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  backupWarning,
  type DeletionGroup,
  deletionGroups,
} from "@/lib/claims/deletion";
import type { SectionClaim } from "@/lib/claims/sections";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import {
  claimDeletionChoice,
  type DeletionChoice,
  type DeletionPolicy,
  type PluginSelection,
} from "@/lib/policy";
import { clusterOptimize } from "./optimize";

/**
 * DeleteClusterData is what the delete page reads once on the server.
 * cluster is undefined once the CamundaCluster is gone. environment is the
 * Environment that the label of the namespace names. requests are the
 * PluginRequests of the claim. volumes is whenDeleted of the retention of
 * the broker volumes in the merged spec of the cluster.
 */
export type DeleteClusterData = {
  claim: Resource<"CamundaClusterClaim">;
  cluster?: Resource<"CamundaCluster">;
  environment?: Resource<"Environment">;
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
  requests: Resource<"PluginRequest">[];
  optimizes: Resource<"CamundaOptimize">[];
  schedules: Resource<"BackupSchedule">[];
  backups: {
    elasticsearch: Resource<"LogicalBackupElasticsearch">[];
    rdbms: Resource<"LogicalBackupRDBMS">[];
  };
  volumes: "Retain" | "Delete";
};

/** DeleteTarget is one object that the delete of a cluster always removes. */
export type DeleteTarget = {
  kind: KindName;
  name: string;
  label: string;
  note?: string;
};

/**
 * DeleteClusterModel is what the delete page shows for a pick. choice is
 * the deletion choice of the claim with policy set to the pick.
 * keepRefused says that the claim keeps its resources and an option cannot.
 * targets are the objects that always go, in the order in which they go,
 * the requests included. backups counts the backups of the cluster of both
 * kinds. warning is the backupWarning of groups.
 */
export type DeleteClusterModel = {
  name: string;
  namespace: string;
  choice: DeletionChoice;
  keepRefused: boolean;
  targets: DeleteTarget[];
  groups: DeletionGroup[];
  backups: number;
  warning?: string;
};

/** clusterDeletionChoice is claimDeletionChoice of the claim of data. */
export function clusterDeletionChoice(data: DeleteClusterData): DeletionChoice {
  const { claim } = data;
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
    environment: data.environment as unknown as Environment | undefined,
    sections: data.sections as unknown as PluginSection[],
    registrations: data.registrations as unknown as PluginRegistration[],
    kind: "CamundaClusterClaim",
  });
}

/**
 * deleteClusterModel returns the model of the delete of the cluster of
 * data under picked. The cluster has the name of its claim.
 */
export function deleteClusterModel(
  data: DeleteClusterData,
  picked: DeletionPolicy,
): DeleteClusterModel {
  const { claim } = data;
  const name = claim.metadata?.name ?? "";
  const namespace = claim.metadata?.namespace ?? "";
  const choice = clusterDeletionChoice(data);
  // A claim holds the parameters of a selection as unknown, because the
  // CRD schema does not give them a shape. deletionGroups reads none.
  const groups = deletionGroups({
    claim: claim as SectionClaim,
    sections: data.sections,
    registrations: data.registrations,
    requests: data.requests,
    policy: picked,
  });
  const ofCluster = (o: { spec: { clusterRef: { name: string } } }) =>
    o.spec.clusterRef.name === name;
  const optimize = clusterOptimize(name, data.optimizes);
  const target = (kind: KindName, objectName: string, note?: string) => ({
    kind,
    name: objectName,
    label: `${kind} ${objectName}`,
    ...(note && { note }),
  });
  const sectionOf = (request: string) =>
    claim.status?.requests?.find((r) => r.name === request)?.section;
  const targets: DeleteTarget[] = [
    ...(optimize
      ? [target("CamundaOptimize", optimize.metadata?.name ?? "")]
      : []),
    ...data.schedules
      .filter(ofCluster)
      .map((s) => target("BackupSchedule", s.metadata?.name ?? "")),
    target(
      "CamundaCluster",
      name,
      data.volumes === "Retain"
        ? "Volumes kept for a cluster of the same name"
        : "Volumes deleted",
    ),
    ...data.requests.map((r) => {
      const requestName = r.metadata?.name ?? "";
      const section = sectionOf(requestName);
      const label = groups.find((g) => g.section === section)?.label;
      return {
        ...target("PluginRequest", requestName),
        ...(label && { label: `${label} request ${requestName}` }),
      };
    }),
    target("CamundaClusterClaim", name),
  ];
  const warning = backupWarning(name, groups);
  return {
    name,
    namespace,
    choice: { ...choice, policy: picked },
    keepRefused: choice.policy === "Orphan" && !choice.keepAllowed,
    targets,
    groups,
    backups:
      data.backups.elasticsearch.filter(ofCluster).length +
      data.backups.rdbms.filter(ofCluster).length,
    ...(warning && { warning }),
  };
}

/**
 * initialPolicy is the pick that the delete page opens on: the policy of
 * choice, but Delete when that keeps and Keep is not allowed.
 */
export function initialPolicy(choice: DeletionChoice): DeletionPolicy {
  return choice.policy === "Orphan" && !choice.keepAllowed
    ? "Delete"
    : choice.policy;
}

/** DeleteState is where one target of a delete is. */
export type DeleteState = "Waiting" | "Deleting" | "Deleted";

/**
 * deleteProgress returns the state of each target, in the order of
 * targets, from the live objects of each kind: Deleting while the object
 * of its name has a deletionTimestamp, Waiting while it has none, and
 * Deleted when no object has its name. A kind that live leaves out counts
 * as having no objects.
 */
export function deleteProgress(
  targets: readonly DeleteTarget[],
  live: Partial<
    Record<
      KindName,
      readonly { metadata?: { name?: string; deletionTimestamp?: string } }[]
    >
  >,
): { target: DeleteTarget; state: DeleteState }[] {
  return targets.map((target) => {
    const object = live[target.kind]?.find(
      (o) => o.metadata?.name === target.name,
    );
    const state: DeleteState =
      object === undefined
        ? "Deleted"
        : object.metadata?.deletionTimestamp
          ? "Deleting"
          : "Waiting";
    return { target, state };
  });
}
