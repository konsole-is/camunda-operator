import type { PluginSectionSpec } from "@/generated/crds/pluginsection";
import { child } from "@/lib/merge";

/** SlotKind is a kind of resource whose fields a slot fills. */
export type SlotKind =
  "CamundaCluster" | "CamundaOptimize" | "CamundaManagementCluster";

/**
 * SlotField is one field that a slot fills for one claim kind. path is the
 * field from the root of the object, in the merge FieldPath grammar, and
 * names are the JSON field names of that path. output
 * is the output key of the section that gives the value. A host output
 * becomes prefix + host + suffix. object marks an output that holds a JSON
 * object as text.
 */
export type SlotField = {
  claimKind: PluginSectionSpec["appliesTo"][number];
  slot: NonNullable<PluginSectionSpec["slot"]>;
  kind: SlotKind;
  path: string;
  names: readonly string[];
  output: string;
  prefix?: string;
  suffix?: string;
  object?: boolean;
};

const cluster = "CamundaClusterClaim";
const plane = "CamundaManagementClusterClaim";

/** SLOT_FIELDS is the slot table of the contract. It is slotTable of pkg/claimpolicy. */
export const SLOT_FIELDS: readonly SlotField[] = (
  [
    {
      claimKind: cluster,
      slot: "workloadIdentity",
      kind: "CamundaCluster",
      names: ["spec", "serviceAccount", "annotations"],
      output: "serviceAccountAnnotations",
      object: true,
    },
    {
      claimKind: cluster,
      slot: "connectorsIdentity",
      kind: "CamundaCluster",
      names: ["spec", "connectors", "serviceAccount", "annotations"],
      output: "serviceAccountAnnotations",
      object: true,
    },
    {
      claimKind: cluster,
      slot: "volumeEncryption",
      kind: "CamundaCluster",
      names: ["spec", "zeebe", "storageClassName"],
      output: "storageClassName",
    },
    {
      claimKind: cluster,
      slot: "backupStorage",
      kind: "CamundaCluster",
      names: ["spec", "backupStorageRef"],
      output: "objectStorageConfig",
    },
    {
      claimKind: cluster,
      slot: "secondaryStorage",
      kind: "CamundaCluster",
      names: ["spec", "storageRef"],
      output: "secondaryStorageConfig",
    },
    {
      claimKind: cluster,
      slot: "externalAccess",
      kind: "CamundaCluster",
      names: ["spec", "externalUrl"],
      output: "externalUrl",
    },
    {
      claimKind: cluster,
      slot: "externalAccess",
      kind: "CamundaOptimize",
      names: ["spec", "externalUrl"],
      output: "optimizeUrl",
    },
    ...(
      [
        [["spec", "identityProvider", "keycloak", "externalUrl"], "/auth"],
        [["spec", "identity", "externalUrl"], "/identity"],
        [["spec", "console", "externalUrl"], "/console"],
        [["spec", "webModeler", "externalUrl"], "/modeler"],
        [["spec", "webModeler", "websocketsExternalUrl"], "/modeler-ws"],
      ] as const
    ).map(([names, suffix]): Row => ({
      claimKind: plane,
      slot: "externalAccess",
      kind: "CamundaManagementCluster",
      names,
      output: "managementHost",
      prefix: "https://",
      suffix,
    })),
    ...(
      [
        [
          ["spec", "identityProvider", "keycloak", "databaseConfigRef"],
          "keycloakDatabaseConfig",
        ],
        [["spec", "identity", "databaseConfigRef"], "identityDatabaseConfig"],
        [
          ["spec", "webModeler", "databaseConfigRef"],
          "webModelerDatabaseConfig",
        ],
      ] as const
    ).map(([names, output]): Row => ({
      claimKind: plane,
      slot: "managementDatabases",
      kind: "CamundaManagementCluster",
      names,
      output,
    })),
  ] satisfies Row[]
).map((row) => ({ ...row, path: row.names.reduce(child, "") }));

type Row = Omit<SlotField, "path">;
