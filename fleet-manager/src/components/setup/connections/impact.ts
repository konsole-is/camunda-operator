import type { ContractKind, ContractUser } from "@/lib/usage";

// What each kind of user does when a contract of a kind changes, as the
// operator reacts. A cluster, an Optimize, and a management plane put the
// generation of each contract that they read in the config hash of their
// pods, so any change of the spec restarts their pods.
const EFFECTS: Record<ContractKind, Record<string, string>> = {
  ObjectStorageConfig: {
    CamundaCluster: "restarts its pods, all but Connectors.",
    CamundaClusterClaim:
      "passes the change to its cluster, which restarts its pods.",
    ElasticsearchCluster:
      "registers its snapshot repository again. A change of the workload identity restarts its pods.",
    DatabaseServer:
      "applies its archive again. A new bucket, base path, region, or endpoint moves the archive: a backup from before the move no longer counts for a rollback.",
    LogicalBackupElasticsearch:
      "fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then cannot reach the old location.",
    LogicalBackupRDBMS:
      "fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then leaves the dump in the old location.",
    PluginRequest:
      "passes the change to the cluster of its claim, which restarts its pods.",
  },
  SecondaryStorageConfig: {
    CamundaCluster:
      "restarts all its pods and the pods of its Optimize. A new endpoint, host, port, or database is a new storage backend for the cluster.",
    CamundaClusterClaim:
      "passes the change to its cluster, which restarts its pods.",
    LogicalBackupElasticsearch:
      "fails while it runs when the endpoint changes.",
    PluginRequest:
      "passes the change to the cluster of its claim, which restarts its pods.",
  },
  DatabaseServerConfig: {
    Database:
      "waits until the operator checks the server again. On a new server it creates its database and roles there. The database on the old server stays.",
    DatabaseConfig:
      "checks the server again. A cluster or management plane that uses this database config restarts its pods.",
    DatabaseServer: "reads the changed contract for the rollback that runs.",
    PointInTimeRestore:
      "fails when the host or the port changes before its recovery step.",
  },
  DatabaseConfig: {
    SecondaryStorageConfig:
      "checks the database config again. A cluster that uses this secondary storage restarts its pods. A new database name is a new storage backend for that cluster.",
    CamundaManagementCluster:
      "restarts its pods: Identity, Console, and Web Modeler for the Keycloak or the Identity database, the Web Modeler REST API for the Web Modeler database. Keycloak gets the new database settings.",
    CamundaManagementClusterClaim:
      "passes the change to its management plane, which restarts its pods.",
    PluginRequest:
      "passes the change to the management plane of its claim, which restarts its pods.",
  },
  ManagementAuthConfig: {
    CamundaOptimize: "restarts its pods.",
    CamundaClusterClaim:
      "passes the change to the Optimize of its cluster, which restarts its pods.",
  },
};

/** CREATE_EFFECT says what the create of a contract changes. */
export const CREATE_EFFECT =
  "A resource that names this contract uses it from now on.";

/**
 * saveEffects returns the lines that the review of a change of a contract
 * of kind shows: each user and what it does on the change. A contract
 * without users gives one line that says so.
 */
export function saveEffects(
  kind: ContractKind,
  users: readonly ContractUser[],
): string[] {
  const lines = users.map(
    (u) =>
      `${u.kind} ${u.namespace}/${u.name} ${
        EFFECTS[kind][u.kind] ?? "reads the changed contract."
      }`,
  );
  return lines.length > 0 ? lines : ["No resource uses this contract."];
}
