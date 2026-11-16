import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaOptimize } from "@/generated/crds/camundaoptimize";
import type { Database } from "@/generated/crds/database";
import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchCluster } from "@/generated/crds/elasticsearchcluster";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type { LogicalBackupElasticsearch } from "@/generated/crds/logicalbackupelasticsearch";
import type { LogicalBackupRDBMS } from "@/generated/crds/logicalbackuprdbms";
import type { PointInTimeRestore } from "@/generated/crds/pointintimerestore";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import {
  ENVIRONMENT_LABEL,
  MANAGEMENT_CLUSTER_LABEL,
  MANAGEMENT_CLUSTER_NAMESPACE_LABEL,
  REGISTRATION_LABEL,
  REQUEST_LABEL,
  REQUEST_NAMESPACE_LABEL,
} from "@/lib/labels";
import { ownerName } from "@/lib/labels/owner-name";
import {
  type ContractRef,
  type ContractUserSources,
  type ContractWriter,
  contractKey,
  contractUsage,
  contractWriter,
  unreadUserKinds,
  writerEditable,
  writerLabel,
} from "./contracts";

function labelled(labels: Record<string, string>, namespace?: string) {
  return { metadata: { ...(namespace && { namespace }), labels } };
}

function owned(owner: { kind: string; name: string; controller?: boolean }) {
  return { metadata: { namespace: "team-a", ownerReferences: [owner] } };
}

function request(namespace: string, name: string) {
  return { metadata: { namespace, name } };
}

describe("contractWriter", () => {
  it("names the registration of a contract with the plugin labels and a live request", () => {
    expect(
      contractWriter(
        labelled(
          { [REGISTRATION_LABEL]: "aws-s3", [REQUEST_LABEL]: "orders-backup" },
          "team-a",
        ),
        [request("team-a", "orders-backup")],
      ),
    ).toEqual({ by: "registration", registration: "aws-s3" });
  });

  it("shows Kept from when the request that the label names is gone", () => {
    expect(
      contractWriter(
        labelled(
          { [REGISTRATION_LABEL]: "aws-s3", [REQUEST_LABEL]: "orders-backup" },
          "team-a",
        ),
        [request("team-b", "orders-backup")],
      ),
    ).toEqual({ by: "kept", registration: "aws-s3" });
  });

  it("looks for the request of a cluster-scoped contract in the namespace of its label", () => {
    const contract = labelled({
      [REGISTRATION_LABEL]: "r",
      [REQUEST_LABEL]: "q",
      [REQUEST_NAMESPACE_LABEL]: "team-b",
    });
    expect(contractWriter(contract, [request("team-b", "q")]).by).toBe(
      "registration",
    );
    expect(contractWriter(contract, [request("team-a", "q")]).by).toBe("kept");
  });

  it("keeps the registration while the request list is not known", () => {
    expect(
      contractWriter(
        labelled(
          { [REGISTRATION_LABEL]: "aws-s3", [REQUEST_LABEL]: "gone" },
          "team-a",
        ),
        undefined,
      ),
    ).toEqual({ by: "registration", registration: "aws-s3" });
  });

  it("keeps the registration of a contract without a request label", () => {
    expect(
      contractWriter(
        labelled({ [REGISTRATION_LABEL]: "aws-s3" }, "team-a"),
        [],
      ),
    ).toEqual({ by: "registration", registration: "aws-s3" });
  });

  it("names the plane of a contract with the management labels", () => {
    expect(
      contractWriter(
        labelled({
          [MANAGEMENT_CLUSTER_LABEL]: "plane-a",
          [MANAGEMENT_CLUSTER_NAMESPACE_LABEL]: "platform",
        }),
        [],
      ),
    ).toEqual({ by: "plane", namespace: "platform", name: "plane-a" });
  });

  it("takes the namespace of a namespaced contract when the plane namespace label is missing", () => {
    expect(
      contractWriter(
        labelled({ [MANAGEMENT_CLUSTER_LABEL]: "plane-a" }, "platform"),
        [],
      ),
    ).toEqual({ by: "plane", namespace: "platform", name: "plane-a" });
  });

  it("names a plane with a long name by its own name when the plane is known", () => {
    const name = `plane-${"a".repeat(70)}`;
    expect(
      contractWriter(
        labelled({
          [MANAGEMENT_CLUSTER_LABEL]: ownerName(name),
          [MANAGEMENT_CLUSTER_NAMESPACE_LABEL]: "platform",
        }),
        [],
        [
          { metadata: { namespace: "other", name } },
          { metadata: { namespace: "platform", name } },
        ],
      ),
    ).toEqual({ by: "plane", namespace: "platform", name });
  });

  it("shows the label value of a plane that is not known", () => {
    const name = `plane-${"a".repeat(70)}`;
    expect(
      contractWriter(
        labelled({
          [MANAGEMENT_CLUSTER_LABEL]: ownerName(name),
          [MANAGEMENT_CLUSTER_NAMESPACE_LABEL]: "platform",
        }),
        [],
        [{ metadata: { namespace: "other", name } }],
      ),
    ).toEqual({ by: "plane", namespace: "platform", name: ownerName(name) });
  });

  it("names the controller owner of a contract", () => {
    expect(
      contractWriter(
        owned({
          kind: "ElasticsearchCluster",
          name: "orders-es",
          controller: true,
        }),
        [],
      ),
    ).toEqual({ by: "owner", kind: "ElasticsearchCluster", name: "orders-es" });
  });

  it("ignores an owner reference that is not the controller", () => {
    expect(contractWriter(owned({ kind: "ConfigMap", name: "x" }), [])).toEqual(
      { by: "hand" },
    );
  });

  it("says Created by hand without labels and owner", () => {
    expect(contractWriter({ metadata: { namespace: "team-a" } }, [])).toEqual({
      by: "hand",
    });
  });

  it("ignores empty writer labels", () => {
    expect(
      contractWriter(
        labelled(
          { [REGISTRATION_LABEL]: "", [MANAGEMENT_CLUSTER_LABEL]: "" },
          "team-a",
        ),
        [],
      ),
    ).toEqual({ by: "hand" });
  });

  it("puts the plugin labels before an owner reference", () => {
    expect(
      contractWriter(
        {
          metadata: {
            namespace: "team-a",
            labels: {
              [REGISTRATION_LABEL]: "elasticsearch",
              [REQUEST_LABEL]: "orders-es",
            },
            ownerReferences: [
              { kind: "PluginRequest", name: "orders-es", controller: true },
            ],
          },
        },
        [request("team-a", "orders-es")],
      ).by,
    ).toBe("registration");
  });

  it("puts the management labels before an owner reference", () => {
    expect(
      contractWriter(
        {
          metadata: {
            namespace: "platform",
            labels: { [MANAGEMENT_CLUSTER_LABEL]: "plane-a" },
            ownerReferences: [
              {
                kind: "CamundaManagementCluster",
                name: "plane-a",
                controller: true,
              },
            ],
          },
        },
        [],
      ).by,
    ).toBe("plane");
  });

  it("puts the plugin labels before the management labels", () => {
    expect(
      contractWriter(
        labelled(
          { [REGISTRATION_LABEL]: "r", [MANAGEMENT_CLUSTER_LABEL]: "plane-a" },
          "platform",
        ),
        [],
      ).by,
    ).toBe("registration");
  });
});

describe("writerLabel", () => {
  it.each<[ContractWriter, string]>([
    [{ by: "registration", registration: "aws-s3" }, "Registration aws-s3"],
    [{ by: "kept", registration: "aws-s3" }, "Kept from aws-s3"],
    [
      { by: "plane", namespace: "platform", name: "plane-a" },
      "Management plane platform/plane-a",
    ],
    [
      { by: "owner", kind: "ElasticsearchCluster", name: "orders-es" },
      "ElasticsearchCluster orders-es",
    ],
    [{ by: "hand" }, "Created by hand"],
  ])("names %j as %j", (writer, label) => {
    expect(writerLabel(writer)).toBe(label);
  });
});

describe("writerEditable", () => {
  it.each<[ContractWriter, boolean]>([
    [{ by: "hand" }, true],
    [{ by: "kept", registration: "aws-s3" }, true],
    [{ by: "registration", registration: "aws-s3" }, false],
    [{ by: "plane", namespace: "platform", name: "plane-a" }, false],
    [{ by: "owner", kind: "ElasticsearchCluster", name: "orders-es" }, false],
  ])("is %j for %j", (writer, editable) => {
    expect(writerEditable(writer)).toBe(editable);
  });
});

// object builds a user of kind in namespace. The cast stands in for the
// required spec fields that a test of the references does not read.
function object<T>(
  kind: string,
  namespace: string,
  name: string,
  spec: object,
  status?: object,
): T {
  return {
    kind,
    metadata: { namespace, name },
    spec,
    ...(status && { status }),
  } as T;
}

function sourcesWith(
  partial: Partial<ContractUserSources>,
): ContractUserSources {
  return {
    namespaces: [],
    environments: [],
    clusters: [],
    clusterClaims: [],
    elasticsearchClusters: [],
    elasticsearchPresets: [],
    databaseServers: [],
    databasePresets: [],
    databases: [],
    databaseConfigs: [],
    secondaryStorageConfigs: [],
    optimizes: [],
    planes: [],
    elasticsearchBackups: [],
    rdbmsBackups: [],
    pointInTimeRestores: [],
    ...partial,
  };
}

function usersOf(sources: ContractUserSources, ref: ContractRef) {
  return contractUsage(sources).get(contractKey(ref)) ?? [];
}

const objectStorage = (namespace: string, name: string): ContractRef => ({
  kind: "ObjectStorageConfig",
  namespace,
  name,
});

describe("contractKey", () => {
  it("keeps a namespaced and a cluster-scoped contract of one name apart", () => {
    expect(contractKey(objectStorage("team-a", "b"))).toBe(
      "ObjectStorageConfig/team-a/b",
    );
    expect(contractKey({ kind: "ManagementAuthConfig", name: "b" })).toBe(
      "ManagementAuthConfig//b",
    );
  });
});

describe("contractUsage", () => {
  it("lists a cluster under each contract it names, by field", () => {
    const sources = sourcesWith({
      clusters: [
        object<CamundaCluster>("CamundaCluster", "team-a", "orders", {
          storageRef: "es",
          backupStorageRef: "b",
          documentStorageRef: "b",
        }),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "b"))).toEqual([
      {
        kind: "CamundaCluster",
        namespace: "team-a",
        name: "orders",
        fields: ["spec.backupStorageRef", "spec.documentStorageRef"],
      },
    ]);
    expect(
      usersOf(sources, {
        kind: "SecondaryStorageConfig",
        namespace: "team-a",
        name: "es",
      }),
    ).toEqual([
      {
        kind: "CamundaCluster",
        namespace: "team-a",
        name: "orders",
        fields: ["spec.storageRef"],
      },
    ]);
  });

  it("resolves a namespaced reference in the namespace of the user", () => {
    const sources = sourcesWith({
      clusters: [
        object<CamundaCluster>("CamundaCluster", "team-a", "orders", {
          backupStorageRef: "backups",
        }),
      ],
    });
    expect(usersOf(sources, objectStorage("team-b", "backups"))).toEqual([]);
    expect(usersOf(sources, objectStorage("team-a", "backups"))).toHaveLength(
      1,
    );
  });

  it("lists users of a Management Identity contract from every namespace", () => {
    const sources = sourcesWith({
      optimizes: [
        object<CamundaOptimize>("CamundaOptimize", "team-a", "optimize", {
          managementAuthRef: "plane-a-optimize",
        }),
      ],
      clusterClaims: [
        object<CamundaClusterClaim>("CamundaClusterClaim", "team-b", "orders", {
          attachments: {
            optimize: { managementAuthConfigRef: "plane-a-optimize" },
          },
        }),
      ],
    });
    expect(
      usersOf(sources, {
        kind: "ManagementAuthConfig",
        name: "plane-a-optimize",
      }),
    ).toEqual([
      {
        kind: "CamundaClusterClaim",
        namespace: "team-b",
        name: "orders",
        fields: ["spec.attachments.optimize.managementAuthConfigRef"],
      },
      {
        kind: "CamundaOptimize",
        namespace: "team-a",
        name: "optimize",
        fields: ["spec.managementAuthRef"],
      },
    ]);
  });

  it("takes the snapshot reference that a preset sets", () => {
    const sources = sourcesWith({
      elasticsearchPresets: [
        {
          kind: "ElasticsearchClusterPreset",
          metadata: { name: "es-small" },
          spec: { cluster: { snapshotStorageRef: "snapshots" } },
        } as ElasticsearchClusterPreset,
      ],
      elasticsearchClusters: [
        object<ElasticsearchCluster>(
          "ElasticsearchCluster",
          "team-a",
          "orders-es",
          {
            presetRef: "es-small",
          },
        ),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "snapshots"))).toEqual([
      {
        kind: "ElasticsearchCluster",
        namespace: "team-a",
        name: "orders-es",
        fields: ["spec.snapshotStorageRef"],
        preset: "es-small",
      },
    ]);
  });

  it("takes the archive reference that a preset sets", () => {
    const sources = sourcesWith({
      databasePresets: [
        {
          kind: "DatabaseServerPreset",
          metadata: { name: "pg-small" },
          spec: { server: { archive: { objectStorageRef: "wal" } } },
        } as DatabaseServerPreset,
      ],
      databaseServers: [
        object<DatabaseServer>("DatabaseServer", "team-a", "pg", {
          presetRef: "pg-small",
        }),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "wal"))).toEqual([
      {
        kind: "DatabaseServer",
        namespace: "team-a",
        name: "pg",
        fields: ["spec.archive.objectStorageRef"],
        preset: "pg-small",
      },
    ]);
  });

  it("names no preset when the user sets the reference itself", () => {
    const sources = sourcesWith({
      elasticsearchPresets: [
        {
          kind: "ElasticsearchClusterPreset",
          metadata: { name: "es-small" },
          spec: { cluster: { snapshotStorageRef: "snapshots" } },
        } as ElasticsearchClusterPreset,
      ],
      elasticsearchClusters: [
        object<ElasticsearchCluster>(
          "ElasticsearchCluster",
          "team-a",
          "orders-es",
          {
            presetRef: "es-small",
            snapshotStorageRef: "own",
          },
        ),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "snapshots"))).toEqual([]);
    expect(usersOf(sources, objectStorage("team-a", "own"))[0].preset).toBe(
      undefined,
    );
  });

  it("merges a missing preset as no preset", () => {
    const sources = sourcesWith({
      elasticsearchClusters: [
        object<ElasticsearchCluster>(
          "ElasticsearchCluster",
          "team-a",
          "orders-es",
          {
            presetRef: "gone",
            snapshotStorageRef: "own",
          },
        ),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "own"))).toEqual([
      {
        kind: "ElasticsearchCluster",
        namespace: "team-a",
        name: "orders-es",
        fields: ["spec.snapshotStorageRef"],
      },
    ]);
  });

  it("counts the archive records of a database server status", () => {
    const sources = sourcesWith({
      databaseServers: [
        object<DatabaseServer>(
          "DatabaseServer",
          "team-a",
          "pg",
          {},
          {
            archive: {
              boundary: { at: "t", location: "l", objectStorageRef: "moved" },
              history: [
                { from: "t", serverName: "pg", objectStorageRef: "old" },
                { from: "t", serverName: "pg" },
              ],
            },
            recovery: {
              archive: { objectStorageRef: "source", serverName: "pg" },
            },
          },
        ),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "old"))).toEqual([
      {
        kind: "DatabaseServer",
        namespace: "team-a",
        name: "pg",
        fields: ["status.archive.history.objectStorageRef"],
      },
    ]);
    expect(
      usersOf(sources, objectStorage("team-a", "moved"))[0].fields,
    ).toEqual(["status.archive.boundary.objectStorageRef"]);
    expect(
      usersOf(sources, objectStorage("team-a", "source"))[0].fields,
    ).toEqual(["status.recovery.archive.objectStorageRef"]);
  });

  it("names no preset for a user whose status alone names the contract", () => {
    const sources = sourcesWith({
      databasePresets: [
        {
          kind: "DatabaseServerPreset",
          metadata: { name: "pg-small" },
          spec: { server: { archive: { objectStorageRef: "wal" } } },
        } as DatabaseServerPreset,
      ],
      databaseServers: [
        object<DatabaseServer>(
          "DatabaseServer",
          "team-a",
          "pg",
          { presetRef: "pg-small" },
          {
            archive: {
              history: [
                { from: "t", serverName: "pg", objectStorageRef: "old" },
              ],
            },
          },
        ),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "old"))[0].preset).toBe(
      undefined,
    );
  });

  it("counts the bucket of each backup", () => {
    const sources = sourcesWith({
      elasticsearchBackups: [
        object<LogicalBackupElasticsearch>(
          "LogicalBackupElasticsearch",
          "team-a",
          "es-backup",
          {},
          { storage: { bucketRef: "b", bucketLocation: "l", endpoint: "e" } },
        ),
      ],
      rdbmsBackups: [
        object<LogicalBackupRDBMS>(
          "LogicalBackupRDBMS",
          "team-a",
          "db-backup",
          {},
          {
            bucketRef: "b",
          },
        ),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "b"))).toEqual([
      {
        kind: "LogicalBackupElasticsearch",
        namespace: "team-a",
        name: "es-backup",
        fields: ["status.storage.bucketRef"],
      },
      {
        kind: "LogicalBackupRDBMS",
        namespace: "team-a",
        name: "db-backup",
        fields: ["status.bucketRef"],
      },
    ]);
  });

  it("counts the contract of a rollback that the server has not answered", () => {
    const server = (name: string, completedAt?: string) =>
      object<DatabaseServer>(
        "DatabaseServer",
        "team-a",
        name,
        {},
        {
          recovery: {
            requestID: "r1",
            contract: "postgres",
            requestedBy: "team-a/restore",
            targetTime: "2026-11-15T10:00:00Z",
            ...(completedAt && { completedAt }),
          },
        },
      );
    const sources = sourcesWith({
      databaseServers: [
        server("running"),
        server("answered", "2026-11-15T11:00:00Z"),
      ],
    });
    expect(
      usersOf(sources, {
        kind: "DatabaseServerConfig",
        namespace: "team-a",
        name: "postgres",
      }),
    ).toEqual([
      {
        kind: "DatabaseServer",
        namespace: "team-a",
        name: "running",
        fields: ["status.recovery.contract"],
      },
    ]);
  });

  it("counts the storage contract that an Elasticsearch backup pinned", () => {
    const sources = sourcesWith({
      elasticsearchBackups: [
        object<LogicalBackupElasticsearch>(
          "LogicalBackupElasticsearch",
          "team-a",
          "es-backup",
          {},
          {
            storage: {
              secondaryStorageConfig: "old-es",
              bucketRef: "b",
              bucketLocation: "l",
              endpoint: "e",
            },
          },
        ),
      ],
    });
    expect(
      usersOf(sources, {
        kind: "SecondaryStorageConfig",
        namespace: "team-a",
        name: "old-es",
      }),
    ).toEqual([
      {
        kind: "LogicalBackupElasticsearch",
        namespace: "team-a",
        name: "es-backup",
        fields: ["status.storage.secondaryStorageConfig"],
      },
    ]);
  });

  it("counts only the database server that a running point-in-time restore pinned", () => {
    const restore = (
      name: string,
      phase: NonNullable<PointInTimeRestore["status"]>["phase"],
    ) =>
      object<PointInTimeRestore>(
        "PointInTimeRestore",
        "team-a",
        name,
        {},
        {
          phase,
          storage: {
            secondaryStorageConfig: "rdbms",
            databaseConfig: "orders-db",
            databaseServerConfig: "postgres",
            databaseName: "orders",
            endpoint: "e",
          },
        },
      );
    const sources = sourcesWith({
      pointInTimeRestores: [
        restore("running", "RestoringDatabase"),
        restore("completed", "Completed"),
        restore("failed", "Failed"),
      ],
    });
    const fieldsOf = (ref: ContractRef) =>
      usersOf(sources, ref).map((u) => [u.kind, u.name, u.fields]);
    expect(
      fieldsOf({
        kind: "DatabaseServerConfig",
        namespace: "team-a",
        name: "postgres",
      }),
    ).toEqual([
      [
        "PointInTimeRestore",
        "running",
        ["status.storage.databaseServerConfig"],
      ],
    ]);
    expect(
      fieldsOf({
        kind: "SecondaryStorageConfig",
        namespace: "team-a",
        name: "rdbms",
      }),
    ).toEqual([]);
    expect(
      fieldsOf({
        kind: "DatabaseConfig",
        namespace: "team-a",
        name: "orders-db",
      }),
    ).toEqual([]);
  });

  it("counts the document storage that a claim sets before its cluster exists", () => {
    const sources = sourcesWith({
      clusterClaims: [
        object<CamundaClusterClaim>("CamundaClusterClaim", "team-a", "orders", {
          cluster: { documentStorageRef: "docs" },
        }),
      ],
    });
    expect(usersOf(sources, objectStorage("team-a", "docs"))).toEqual([
      {
        kind: "CamundaClusterClaim",
        namespace: "team-a",
        name: "orders",
        fields: ["spec.cluster.documentStorageRef"],
      },
    ]);
  });

  it("lists the database configs and databases of a database server config", () => {
    const sources = sourcesWith({
      databaseConfigs: [
        object<DatabaseConfig>("DatabaseConfig", "team-a", "orders-db", {
          serverRef: "postgres",
        }),
      ],
      databases: [
        object<Database>("Database", "team-a", "orders", {
          serverRef: "postgres",
        }),
      ],
    });
    expect(
      usersOf(sources, {
        kind: "DatabaseServerConfig",
        namespace: "team-a",
        name: "postgres",
      }).map((u) => [u.kind, u.name, u.fields]),
    ).toEqual([
      ["Database", "orders", ["spec.serverRef"]],
      ["DatabaseConfig", "orders-db", ["spec.serverRef"]],
    ]);
  });

  it("lists the secondary storage contracts of a database config", () => {
    const sources = sourcesWith({
      secondaryStorageConfigs: [
        object<SecondaryStorageConfig>(
          "SecondaryStorageConfig",
          "team-a",
          "rdbms",
          {
            type: "rdbms",
            rdbms: { databaseConfigRef: "orders-db" },
          },
        ),
      ],
    });
    expect(
      usersOf(sources, {
        kind: "DatabaseConfig",
        namespace: "team-a",
        name: "orders-db",
      }),
    ).toEqual([
      {
        kind: "SecondaryStorageConfig",
        namespace: "team-a",
        name: "rdbms",
        fields: ["spec.rdbms.databaseConfigRef"],
      },
    ]);
  });

  it.each([
    [
      { identityProvider: { keycloak: { databaseConfigRef: "db" } } },
      "spec.identityProvider.keycloak.databaseConfigRef",
    ],
    [
      { identity: { databaseConfigRef: "db" } },
      "spec.identity.databaseConfigRef",
    ],
    [
      { webModeler: { databaseConfigRef: "db" } },
      "spec.webModeler.databaseConfigRef",
    ],
  ])("lists a plane whose spec %j names a database config", (spec, field) => {
    const sources = sourcesWith({
      planes: [
        object<CamundaManagementCluster>(
          "CamundaManagementCluster",
          "platform",
          "plane-a",
          spec,
        ),
      ],
    });
    expect(
      usersOf(sources, {
        kind: "DatabaseConfig",
        namespace: "platform",
        name: "db",
      }),
    ).toEqual([
      {
        kind: "CamundaManagementCluster",
        namespace: "platform",
        name: "plane-a",
        fields: [field],
      },
    ]);
  });

  it("gives each user its Ready and its environment", () => {
    const sources = sourcesWith({
      environments: [
        {
          kind: "Environment",
          metadata: { name: "production" },
          spec: {},
        } as Environment,
      ],
      namespaces: [
        {
          metadata: {
            name: "team-a",
            labels: { [ENVIRONMENT_LABEL]: "production" },
          },
        },
      ],
      clusters: [
        object<CamundaCluster>(
          "CamundaCluster",
          "team-a",
          "billing",
          { storageRef: "shared-es" },
          {
            conditions: [
              {
                type: "Ready",
                status: "False",
                reason: "StorageAlreadyAttached",
                message: "the backend is attached to team-b/other",
                lastTransitionTime: "2026-01-01T00:00:00Z",
              },
            ],
          },
        ),
      ],
    });
    expect(
      usersOf(sources, {
        kind: "SecondaryStorageConfig",
        namespace: "team-a",
        name: "shared-es",
      }),
    ).toEqual([
      {
        kind: "CamundaCluster",
        namespace: "team-a",
        name: "billing",
        fields: ["spec.storageRef"],
        ready: {
          status: "False",
          reason: "StorageAlreadyAttached",
          message: "the backend is attached to team-b/other",
        },
        environment: "production",
      },
    ]);
  });

  it("ignores an empty reference", () => {
    const sources = sourcesWith({
      clusters: [
        object<CamundaCluster>("CamundaCluster", "team-a", "orders", {
          storageRef: "",
          backupStorageRef: "",
        }),
      ],
    });
    expect(contractUsage(sources).size).toBe(0);
  });

  it("sorts users by kind, namespace, and name", () => {
    const cluster = (namespace: string, name: string) =>
      object<CamundaCluster>("CamundaCluster", namespace, name, {
        backupStorageRef: "b",
      });
    const sources = sourcesWith({
      clusters: [cluster("team-a", "z"), cluster("team-a", "a")],
      elasticsearchClusters: [
        object<ElasticsearchCluster>("ElasticsearchCluster", "team-a", "es", {
          snapshotStorageRef: "b",
        }),
      ],
      rdbmsBackups: [
        object<LogicalBackupRDBMS>(
          "LogicalBackupRDBMS",
          "team-a",
          "bk",
          {},
          {
            bucketRef: "b",
          },
        ),
      ],
    });
    expect(
      usersOf(sources, objectStorage("team-a", "b")).map(
        (u) => `${u.kind} ${u.name}`,
      ),
    ).toEqual([
      "CamundaCluster a",
      "CamundaCluster z",
      "ElasticsearchCluster es",
      "LogicalBackupRDBMS bk",
    ]);
  });
});

describe("unreadUserKinds", () => {
  it("names the unread kinds that can use a contract of the kind", () => {
    expect(
      unreadUserKinds("ObjectStorageConfig", [
        "PluginRequest",
        "ElasticsearchClusterPreset",
        "CamundaCluster",
      ]),
    ).toEqual(["CamundaCluster", "ElasticsearchClusterPreset"]);
  });

  it("is empty when no unread kind can use a contract of the kind", () => {
    expect(
      unreadUserKinds("ManagementAuthConfig", ["CamundaCluster", "Namespace"]),
    ).toEqual([]);
  });

  it("names an unread point-in-time restore only for a database server contract", () => {
    const unread = ["PointInTimeRestore"];
    expect(unreadUserKinds("DatabaseServerConfig", unread)).toEqual(unread);
    expect(unreadUserKinds("SecondaryStorageConfig", unread)).toEqual([]);
    expect(unreadUserKinds("DatabaseConfig", unread)).toEqual([]);
  });

  // A new reference of contractUsage belongs in these sources too, or this
  // test misses its kind.
  it("knows every kind that contractUsage lists as a user", () => {
    const named = { storageRef: "s", backupStorageRef: "o" };
    const sources = sourcesWith({
      clusters: [
        object<CamundaCluster>("CamundaCluster", "ns", "c", {
          ...named,
          documentStorageRef: "o",
        }),
      ],
      clusterClaims: [
        object<CamundaClusterClaim>("CamundaClusterClaim", "ns", "cc", {
          cluster: { documentStorageRef: "o" },
          attachments: { optimize: { managementAuthConfigRef: "m" } },
        }),
      ],
      elasticsearchClusters: [
        object<ElasticsearchCluster>("ElasticsearchCluster", "ns", "es", {
          snapshotStorageRef: "o",
        }),
      ],
      databaseServers: [
        object<DatabaseServer>(
          "DatabaseServer",
          "ns",
          "db",
          { archive: { objectStorageRef: "o" } },
          {
            archive: { boundary: { objectStorageRef: "o" } },
            recovery: { contract: "dsc" },
          },
        ),
      ],
      databases: [
        object<Database>("Database", "ns", "d", { serverRef: "dsc" }),
      ],
      databaseConfigs: [
        object<DatabaseConfig>("DatabaseConfig", "ns", "dc", {
          serverRef: "dsc",
        }),
      ],
      secondaryStorageConfigs: [
        object<SecondaryStorageConfig>("SecondaryStorageConfig", "ns", "s", {
          rdbms: { databaseConfigRef: "dc" },
        }),
      ],
      optimizes: [
        object<CamundaOptimize>("CamundaOptimize", "ns", "o", {
          managementAuthRef: "m",
        }),
      ],
      planes: [
        object<CamundaManagementCluster>(
          "CamundaManagementCluster",
          "ns",
          "p",
          {
            identity: { databaseConfigRef: "dc" },
          },
        ),
      ],
      elasticsearchBackups: [
        object<LogicalBackupElasticsearch>(
          "LogicalBackupElasticsearch",
          "ns",
          "eb",
          {},
          { storage: { secondaryStorageConfig: "s", bucketRef: "o" } },
        ),
      ],
      rdbmsBackups: [
        object<LogicalBackupRDBMS>(
          "LogicalBackupRDBMS",
          "ns",
          "rb",
          {},
          { bucketRef: "o" },
        ),
      ],
      pointInTimeRestores: [
        object<PointInTimeRestore>(
          "PointInTimeRestore",
          "ns",
          "r",
          {},
          {
            storage: {
              secondaryStorageConfig: "s",
              databaseConfig: "dc",
              databaseServerConfig: "dsc",
            },
          },
        ),
      ],
    });
    const usage = contractUsage(sources);
    expect(usage.size).toBe(5);
    for (const [key, users] of usage) {
      const kind = key.split("/")[0] as ContractRef["kind"];
      const kinds = [...new Set(users.map((u) => u.kind))];
      expect(unreadUserKinds(kind, kinds)).toEqual(kinds.sort());
    }
  });
});
