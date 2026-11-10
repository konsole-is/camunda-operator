import type { ReactNode } from "react";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { ClusterField } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import type { Merged } from "@/lib/merge";
import type { WriteMode } from "@/lib/write/types";
import type { BackupObject, BackupStorage, RestoreObject } from "./backups";
import type { CardId } from "./cards";

/** ACTION_SLOTS names the places where the write flows mount their actions. */
export const ACTION_SLOTS = [
  "cluster.header.actions",
  "cluster.topology.actions",
  "cluster.backups.actions",
  "cluster.claim.editor",
  "component.settings.editor",
] as const;

export type ActionSlotName = (typeof ACTION_SLOTS)[number];

/**
 * ClusterSlotContext is what ClusterView gives each slot: the live objects
 * of the page, as its hooks follow them. claim is undefined when the claim
 * of the cluster is not in the namespace. environment is the Environment
 * that the label of the namespace names. storage is the
 * SecondaryStorageConfig of spec.storageRef of the cluster. backups and
 * restores hold every backup and restore of the namespace. fields are the
 * field states of the cluster (clusterFields). writeMode is undefined when
 * the page could not read the write mode. openId is the component whose
 * detail the Overview tab shows. now is the clock of the page.
 */
export type ClusterSlotContext = {
  cluster: Resource<"CamundaCluster">;
  claim?: Resource<"CamundaClusterClaim">;
  merged: Merged<CamundaClusterSpec>;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  managementAuthConfigs: readonly Resource<"ManagementAuthConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  storage?: Resource<"SecondaryStorageConfig">;
  backups: readonly BackupObject[];
  restores: readonly RestoreObject[];
  backupStorage: BackupStorage;
  schedules: readonly Resource<"BackupSchedule">[];
  optimize?: Resource<"CamundaOptimize">;
  fields: readonly ClusterField[];
  writeMode?: WriteMode;
  openId?: CardId;
  now: string;
};

/** ActionSlotFill is what fills one slot: a node, or a render function over the slot context. */
export type ActionSlotFill =
  ReactNode | ((context: ClusterSlotContext) => ReactNode);

/** ActionSlots holds the fill of each filled slot. */
export type ActionSlots = Partial<Record<ActionSlotName, ActionSlotFill>>;
