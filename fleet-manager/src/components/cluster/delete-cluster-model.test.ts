import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import {
  clusterDeletionChoice,
  type DeleteClusterData,
  deleteClusterModel,
  deleteProgress,
  initialPolicy,
} from "./delete-cluster-model";

const object = <K extends Parameters<typeof cast>[0]>(
  kind: K,
  name: string,
  rest: object = {},
) => cast(kind, { kind, metadata: { namespace: "team-a", name }, ...rest });
function cast<
  K extends
    | "CamundaClusterClaim"
    | "CamundaCluster"
    | "CamundaOptimize"
    | "BackupSchedule"
    | "LogicalBackupElasticsearch"
    | "LogicalBackupRDBMS"
    | "PluginSection"
    | "PluginRegistration"
    | "PluginRequest",
>(_kind: K, value: object): Resource<K> {
  return value as unknown as Resource<K>;
}
const of = (cluster: string) => ({ spec: { clusterRef: { name: cluster } } });

const claim = (spec: object = {}) =>
  object("CamundaClusterClaim", "orders", {
    apiVersion: "fleet.camunda.io/v1alpha1",
    spec: {
      plugins: [{ section: "backup-storage", registration: "aws-s3" }],
      ...spec,
    },
    status: {
      requests: [
        {
          section: "backup-storage",
          registration: "aws-s3",
          name: "orders-backup-storage",
        },
      ],
    },
  });

const data = (over: Partial<DeleteClusterData> = {}): DeleteClusterData => ({
  claim: claim(),
  cluster: object("CamundaCluster", "orders"),
  sections: [
    cast("PluginSection", {
      metadata: { name: "backup-storage" },
      spec: {
        label: "Backup storage",
        selection: "One",
        appliesTo: ["CamundaClusterClaim"],
        slot: "backupStorage",
      },
    }),
  ],
  registrations: [
    cast("PluginRegistration", {
      metadata: { name: "aws-s3" },
      spec: {
        label: "Amazon S3",
        sections: ["backup-storage"],
        deletionPolicies: ["Delete", "Orphan"],
      },
    }),
  ],
  requests: [
    object("PluginRequest", "orders-backup-storage", {
      status: {
        resources: [
          { kind: "S3Bucket", name: "orders-backups", onOrphan: "Kept" },
        ],
      },
    }),
  ],
  optimizes: [object("CamundaOptimize", "orders-optimize", of("orders"))],
  schedules: [
    object("BackupSchedule", "orders-daily", of("orders")),
    object("BackupSchedule", "payments-daily", of("payments")),
  ],
  backups: { elasticsearch: [], rdbms: [] },
  volumes: "Delete",
  ...over,
});

describe("deleteClusterModel", () => {
  it("lists Optimize and the schedule, then the cluster with its volumes, then the requests, then the claim", () => {
    const model = deleteClusterModel(data(), "Delete");
    expect(model).toMatchObject({ name: "orders", namespace: "team-a" });
    expect(model.targets.map((t) => t.label)).toEqual([
      "CamundaOptimize orders-optimize",
      "BackupSchedule orders-daily",
      "CamundaCluster orders",
      "Backup storage request orders-backup-storage",
      "CamundaClusterClaim orders",
    ]);
    expect(model.targets.map((t) => [t.kind, t.name])).toEqual([
      ["CamundaOptimize", "orders-optimize"],
      ["BackupSchedule", "orders-daily"],
      ["CamundaCluster", "orders"],
      ["PluginRequest", "orders-backup-storage"],
      ["CamundaClusterClaim", "orders"],
    ]);
    expect(model.targets[2].note).toBe("Volumes deleted");
  });

  it("a cluster that retains its volumes says so", () => {
    const model = deleteClusterModel(data({ volumes: "Retain" }), "Delete");
    expect(model.targets.find((t) => t.kind === "CamundaCluster")?.note).toBe(
      "Volumes kept for a cluster of the same name",
    );
  });

  it("leaves out an Optimize and a schedule that the cluster does not have", () => {
    const model = deleteClusterModel(
      data({
        optimizes: [
          object("CamundaOptimize", "payments-optimize", of("payments")),
        ],
        schedules: [object("BackupSchedule", "payments-daily", of("payments"))],
      }),
      "Delete",
    );
    expect(model.targets.map((t) => t.kind)).toEqual([
      "CamundaCluster",
      "PluginRequest",
      "CamundaClusterClaim",
    ]);
  });

  it("counts the backups of this cluster only", () => {
    const model = deleteClusterModel(
      data({
        backups: {
          elasticsearch: [
            object("LogicalBackupElasticsearch", "orders-1", of("orders")),
            object("LogicalBackupElasticsearch", "payments-1", of("payments")),
          ],
          rdbms: [object("LogicalBackupRDBMS", "orders-2", of("orders"))],
        },
      }),
      "Delete",
    );
    expect(model.backups).toBe(2);
  });

  it("the groups and the warning follow the picked policy", () => {
    const deleting = deleteClusterModel(data(), "Delete");
    expect(deleting.choice.policy).toBe("Delete");
    expect(deleting.groups[0].rows[0].fate).toBe("Deleted");
    expect(deleting.warning).toBe(
      "You cannot restore orders after this: the backup bucket goes too.",
    );
    const keeping = deleteClusterModel(data(), "Orphan");
    expect(keeping.choice.policy).toBe("Orphan");
    expect(keeping.groups[0].rows[0].fate).toBe("Kept");
    expect(keeping.warning).toBeUndefined();
  });

  it("a claim on Orphan with an option that cannot keep opens on Delete", () => {
    const refused = data({
      claim: claim({ deletionPolicy: "Orphan" }),
      registrations: [
        cast("PluginRegistration", {
          metadata: { name: "aws-s3" },
          spec: {
            label: "Amazon S3",
            sections: ["backup-storage"],
            deletionPolicies: ["Delete"],
          },
        }),
      ],
    });
    expect(deleteClusterModel(refused, "Delete").keepRefused).toBe(true);
    expect(initialPolicy(clusterDeletionChoice(refused))).toBe("Delete");
    expect(deleteClusterModel(data(), "Delete").keepRefused).toBe(false);
  });
});

describe("clusterDeletionChoice and initialPolicy", () => {
  it("open on the policy of the claim, else on the policy that it recorded", () => {
    expect(
      initialPolicy(
        clusterDeletionChoice(
          data({ claim: claim({ deletionPolicy: "Orphan" }) }),
        ),
      ),
    ).toBe("Orphan");
    expect(initialPolicy(clusterDeletionChoice(data()))).toBe("Delete");
  });
});

describe("deleteProgress", () => {
  it("a live object is Waiting, one being deleted is Deleting, and a gone one is Deleted, in target order", () => {
    const model = deleteClusterModel(data(), "Delete");
    const progress = deleteProgress(model.targets, {
      CamundaOptimize: [],
      BackupSchedule: [{ metadata: { name: "payments-daily" } }],
      CamundaCluster: [
        {
          metadata: {
            name: "orders",
            deletionTimestamp: "2026-11-15T10:00:00Z",
          },
        },
      ],
      PluginRequest: [{ metadata: { name: "orders-backup-storage" } }],
      CamundaClusterClaim: [
        {
          metadata: {
            name: "orders",
            deletionTimestamp: "2026-11-15T10:00:00Z",
          },
        },
      ],
    });
    expect(progress.map((p) => [p.target.label, p.state])).toEqual([
      ["CamundaOptimize orders-optimize", "Deleted"],
      ["BackupSchedule orders-daily", "Deleted"],
      ["CamundaCluster orders", "Deleting"],
      ["Backup storage request orders-backup-storage", "Waiting"],
      ["CamundaClusterClaim orders", "Deleting"],
    ]);
  });
});
