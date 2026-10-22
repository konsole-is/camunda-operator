import type { FieldPath } from "@/lib/merge";

/**
 * FieldGroup is where a field shows: one of the six components of a cluster,
 * "cluster" for every other field of the CamundaCluster spec, an attachment,
 * or the section parameters.
 */
export type FieldGroup =
  | "zeebe"
  | "gateway"
  | "operate"
  | "tasklist"
  | "admin"
  | "connectors"
  | "cluster"
  | "optimize"
  | "backupSchedule"
  | "parameters";

/** FieldTarget is the object that holds a field. A parameter is held by the request of one selection. */
export type FieldTarget =
  | { kind: "CamundaCluster" | "CamundaOptimize" | "BackupSchedule" }
  | { kind: "PluginRequest"; section: string; registration: string };

/**
 * FieldState says where the value of a field comes from. takeOver of setBy
 * is true when setting the field in the claim takes it from its manager. A
 * setBy without a manager has no known writer. default is a value that no
 * manager owns.
 */
export type FieldState =
  | { kind: "filled"; section: string; sectionLabel: string }
  | { kind: "setBy"; manager?: string; takeOver: boolean }
  | { kind: "claim" }
  | { kind: "release" }
  | { kind: "preset" }
  | { kind: "admin" }
  | { kind: "environment" }
  | { kind: "default" };

/** FieldLock says why a field cannot change. */
export type FieldLock =
  | { reason: "filled"; section: string; sectionLabel: string }
  | {
      reason:
        "release" | "claimController" | "section" | "notManaged" | "admin";
    };

/**
 * ClusterField is one value of a cluster as the admin sees it. key is
 * unique in one result. path is the FieldPath in the target object, label
 * the path from the root of its group. value is undefined while the field
 * has no value. claimPath is where a claim sets the value, when it can.
 * docNames are the JSON field names from the root of a CRD target, without
 * list items, for the field docs. description is the schema description of
 * a section parameter.
 */
export type ClusterField = {
  key: string;
  target: FieldTarget;
  group: FieldGroup;
  path: FieldPath;
  label: string;
  value: unknown;
  state: FieldState;
  lock?: FieldLock;
  claimPath?: FieldPath;
  docNames?: string[];
  description?: string;
};
