/** LockKind is a kind of resource whose fields a lock row names. */
export type LockKind = "CamundaCluster" | "CamundaOptimize" | "BackupSchedule";

/**
 * LockRow names a field that a claim cannot change, by the JSON field names
 * of its path. release: the release sets it. claimController: the claim
 * controller fixes it. section: only a section selection sets it.
 */
export type LockRow = {
  kind: LockKind;
  reason: "release" | "claimController" | "section";
  names: readonly string[];
};

/** LOCK_FIELDS is the lock table of the cluster field states (G1 spec "Cluster field states"). */
export const LOCK_FIELDS: readonly LockRow[] = [
  { kind: "CamundaCluster", reason: "release", names: ["spec", "version"] },
  {
    kind: "CamundaCluster",
    reason: "release",
    names: ["spec", "connectors", "version"],
  },
  { kind: "CamundaOptimize", reason: "release", names: ["spec", "version"] },
  {
    kind: "CamundaCluster",
    reason: "claimController",
    names: ["spec", "serviceAccount", "name"],
  },
  {
    kind: "CamundaCluster",
    reason: "claimController",
    names: ["spec", "connectors", "serviceAccount", "name"],
  },
  {
    kind: "CamundaOptimize",
    reason: "claimController",
    names: ["spec", "clusterRef", "name"],
  },
  {
    kind: "BackupSchedule",
    reason: "claimController",
    names: ["spec", "clusterRef", "name"],
  },
  { kind: "CamundaCluster", reason: "section", names: ["spec", "storageRef"] },
  {
    kind: "CamundaCluster",
    reason: "section",
    names: ["spec", "backupStorageRef"],
  },
];
