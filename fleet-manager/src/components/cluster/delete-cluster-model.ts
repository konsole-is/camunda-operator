// This module runs in the browser and on the server.
import type { ReadError } from "@/components/planes/planes-table";
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
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import {
  claimDeletionChoice,
  type DeletionChoice,
  type DeletionPolicy,
  type PluginSelection,
} from "@/lib/policy";

/**
 * DeleteClusterData is what the delete page reads. cluster is undefined
 * once the CamundaCluster is gone, or when its read failed. namespace is
 * the namespace of the claim, undefined when its read failed; the model
 * takes the Environment that its label names. requests are the
 * PluginRequests of the namespace. The model takes those that the claim
 * controls. presets and releases give the merged spec of the cluster.
 * errors holds each read that failed.
 */
export type DeleteClusterData = {
  claim: Resource<"CamundaClusterClaim">;
  cluster?: Resource<"CamundaCluster">;
  namespace?: Resource<"Namespace">;
  environments: Resource<"Environment">[];
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
  requests: Resource<"PluginRequest">[];
  optimizes: Resource<"CamundaOptimize">[];
  schedules: Resource<"BackupSchedule">[];
  backups: {
    elasticsearch: Resource<"LogicalBackupElasticsearch">[];
    rdbms: Resource<"LogicalBackupRDBMS">[];
  };
  presets: Resource<"CamundaClusterPreset">[];
  releases: Resource<"CamundaRelease">[];
  errors: ReadError[];
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
 * the requests included. kept are the CamundaCluster, CamundaOptimize, and
 * BackupSchedule objects of the cluster that stay, each with the reason in
 * note. backups counts the backups of the cluster of both kinds. warning is the backupWarning of groups, unset while
 * fatesKnown is false. unread are the kinds that the page could not read;
 * fatesKnown is false when the fates of groups depend on one.
 */
export type DeleteClusterModel = {
  name: string;
  namespace: string;
  choice: DeletionChoice;
  keepRefused: boolean;
  targets: DeleteTarget[];
  kept: DeleteTarget[];
  groups: DeletionGroup[];
  backups: number;
  warning?: string;
  unread: string[];
  fatesKnown: boolean;
};

const FATE_KINDS: readonly string[] = [
  "PluginRequest",
  "PluginRegistration",
  "PluginSection",
];
const VOLUME_KINDS: readonly string[] = [
  "CamundaCluster",
  "CamundaClusterPreset",
  "CamundaRelease",
];

/** KEPT_ATTACHMENT is the note of each object in kept. */
export const KEPT_ATTACHMENT = "Kept: the claim did not make it";

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
    environment: environmentOf(data) as unknown as Environment | undefined,
    sections: data.sections as unknown as PluginSection[],
    registrations: data.registrations as unknown as PluginRegistration[],
    kind: "CamundaClusterClaim",
  });
}

function environmentOf(
  data: DeleteClusterData,
): Resource<"Environment"> | undefined {
  const name = data.namespace?.metadata?.labels?.[ENVIRONMENT_LABEL];
  return name === undefined
    ? undefined
    : data.environments.find((e) => e.metadata?.name === name);
}

/**
 * deleteClusterModel returns the model of the delete of the cluster of
 * data under picked, while the kinds of unread are not read. The cluster
 * has the name of its claim.
 */
export function deleteClusterModel(
  data: DeleteClusterData,
  picked: DeletionPolicy,
  unread: readonly string[] = [],
): DeleteClusterModel {
  const { claim } = data;
  const name = claim.metadata?.name ?? "";
  const namespace = claim.metadata?.namespace ?? "";
  const choice = clusterDeletionChoice(data);
  const claimUid = claim.metadata?.uid;
  const controlled = (o: Owned) =>
    claimUid !== undefined &&
    (o.metadata?.ownerReferences ?? []).some(
      (r) => r.controller === true && r.uid === claimUid,
    );
  // The operator deletes the requests that the claim controls, whatever
  // their labels.
  const requests = data.requests.filter(controlled);
  // A claim holds the parameters of a selection as unknown, because the
  // CRD schema does not give them a shape. deletionGroups reads none.
  const groups = deletionGroups({
    claim: claim as SectionClaim,
    sections: data.sections,
    registrations: data.registrations,
    requests,
    policy: picked,
  });
  const ofCluster = (o: { spec: { clusterRef: { name: string } } }) =>
    o.spec.clusterRef.name === name;
  const target = (kind: KindName, objectName: string, note?: string) => ({
    kind,
    name: objectName,
    label: `${kind} ${objectName}`,
    ...(note && { note }),
  });
  const sectionOf = (request: string) =>
    claim.status?.requests?.find((r) => r.name === request)?.section;
  const attachments = (
    kind: "CamundaOptimize" | "BackupSchedule",
    objects: readonly Attachment[],
    generated: string,
  ) => {
    // The claim finalizer deletes only <name>-optimize and <name>-backups,
    // and only when the claim is their controller.
    const goes = (o: Attachment) =>
      o.metadata?.name === generated && controlled(o);
    const mine = objects.filter(
      (o) => ofCluster(o) || o.metadata?.name === generated,
    );
    return {
      goes: mine.filter(goes).map((o) => target(kind, o.metadata?.name ?? "")),
      stays: mine
        .filter((o) => !goes(o))
        .map((o) => target(kind, o.metadata?.name ?? "", KEPT_ATTACHMENT)),
    };
  };
  const optimizes = attachments(
    "CamundaOptimize",
    data.optimizes,
    `${name}-optimize`,
  );
  const schedules = attachments(
    "BackupSchedule",
    data.schedules,
    `${name}-backups`,
  );
  const volumesKnown = !unread.some((k) => VOLUME_KINDS.includes(k));
  const fatesKnown = !unread.some((k) => FATE_KINDS.includes(k));
  // The claim finalizer deletes the cluster only when the claim controls
  // it. status.cluster can lag the live cluster, so it counts only while
  // the page has no cluster.
  const clusterGoes = data.cluster
    ? controlled(data.cluster)
    : claim.status?.cluster?.name === name;
  const clusterKept =
    !clusterGoes && data.cluster
      ? [target("CamundaCluster", name, KEPT_ATTACHMENT)]
      : [];
  const targets: DeleteTarget[] = [
    ...optimizes.goes,
    ...schedules.goes,
    ...(clusterGoes
      ? [
          target(
            "CamundaCluster",
            name,
            !volumesKnown
              ? "Volumes not known"
              : volumesOf(data) === "Retain"
                ? "Volumes kept for a cluster of the same name"
                : "Volumes deleted",
          ),
        ]
      : []),
    ...requests.map((r) => {
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
  const warning = fatesKnown ? backupWarning(name, groups) : undefined;
  return {
    name,
    namespace,
    choice: { ...choice, policy: picked },
    keepRefused: choice.policy === "Orphan" && !choice.keepAllowed,
    targets,
    kept: [...clusterKept, ...optimizes.stays, ...schedules.stays],
    groups,
    backups:
      data.backups.elasticsearch.filter(ofCluster).length +
      data.backups.rdbms.filter(ofCluster).length,
    ...(warning && { warning }),
    unread: [...unread],
    fatesKnown,
  };
}

// volumesOf returns whenDeleted of the retention of the broker volumes in
// the merged spec of the cluster of data, Delete without one.
function volumesOf(data: DeleteClusterData): string {
  const spec = data.cluster?.spec;
  if (!spec) return "Delete";
  return (
    mergeCluster(
      data.presets.find((p) => p.metadata?.name === spec.presetRef)?.spec,
      data.releases.find((r) => r.metadata?.name === spec.releaseRef)?.spec,
      spec,
    ).spec.zeebe?.persistentVolumeClaimRetentionPolicy?.whenDeleted ?? "Delete"
  );
}

// Owned is the part of an object that names its controller.
type Owned = {
  metadata?: {
    name?: string;
    ownerReferences?: { controller?: boolean; uid: string }[];
  };
};

// Attachment is the part of a CamundaOptimize or a BackupSchedule that the
// model reads.
type Attachment = Owned & { spec: { clusterRef: { name: string } } };

/**
 * initialPolicy is the pick that the delete page opens on: the policy of
 * choice, but Delete when that keeps and Keep is not allowed.
 */
export function initialPolicy(choice: DeletionChoice): DeletionPolicy {
  return choice.policy === "Orphan" && !choice.keepAllowed
    ? "Delete"
    : choice.policy;
}

const TARGET_ORDER: readonly KindName[] = [
  "CamundaOptimize",
  "BackupSchedule",
  "CamundaCluster",
  "PluginRequest",
  "CamundaClusterClaim",
];

/**
 * mergeTargets returns kept with each target of found that it lacks, in
 * the order in which the kinds go.
 */
export function mergeTargets(
  kept: readonly DeleteTarget[],
  found: readonly DeleteTarget[],
): DeleteTarget[] {
  const added = found.filter(
    (f) => !kept.some((k) => k.kind === f.kind && k.name === f.name),
  );
  const rank = (t: DeleteTarget) => TARGET_ORDER.indexOf(t.kind);
  return [...kept, ...added].sort((a, b) => rank(a) - rank(b));
}

/** DeleteState is where one target of a delete is. */
export type DeleteState = "Waiting" | "Deleting" | "Deleted" | "Not known";

/**
 * deleteProgress returns the state of each target, in the order of
 * targets, from the live objects of each kind: Deleting while the object
 * of its name has a deletionTimestamp, Waiting while it has none, and
 * Deleted when no object has its name. A kind that live leaves out counts
 * as having no objects. A target of a kind in unread is Not known.
 */
export function deleteProgress(
  targets: readonly DeleteTarget[],
  live: Partial<
    Record<
      KindName,
      readonly { metadata?: { name?: string; deletionTimestamp?: string } }[]
    >
  >,
  unread: readonly string[] = [],
): { target: DeleteTarget; state: DeleteState }[] {
  return targets.map((target) => {
    if (unread.includes(target.kind))
      return { target, state: "Not known" as const };
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
