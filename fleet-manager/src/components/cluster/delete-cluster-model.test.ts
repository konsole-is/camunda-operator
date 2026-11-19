import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import {
  clusterDeletionChoice,
  type DeleteClusterData,
  deleteClusterModel,
  deleteProgress,
  initialPolicy,
  KEPT_ATTACHMENT,
  mergeTargets,
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
// made is a CamundaOptimize or a BackupSchedule of the cluster that the
// claim made and controls.
const made = (name: string) => ({
  metadata: {
    namespace: "team-a",
    name,
    ownerReferences: [{ controller: true, uid: "claim-uid" }],
  },
  ...of("orders"),
});

// claim is a claim that controls its cluster, unless status says otherwise.
const claim = (spec: object = {}, status: object = {}) =>
  cast("CamundaClusterClaim", {
    kind: "CamundaClusterClaim",
    metadata: { namespace: "team-a", name: "orders", uid: "claim-uid" },
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
      cluster: { name: "orders" },
      ...status,
    },
  });

// applied gives a cluster the status of a pass of the operator that
// applied its generation 3.
const applied = (
  rest: { metadata?: object; spec?: object } = {},
  reasons: { ready?: string; zeebe?: string } = {},
  observed = 3,
) => ({
  ...rest,
  metadata: { ...made("orders").metadata, ...rest.metadata, generation: 3 },
  status: {
    observedGeneration: observed,
    conditions: [
      {
        type: "Ready",
        status: "True",
        reason: reasons.ready ?? "Healthy",
        observedGeneration: observed,
      },
      {
        type: "ZeebeReady",
        status: "True",
        reason: reasons.zeebe ?? "Healthy",
        observedGeneration: observed,
      },
    ],
  },
});
const retainSpec = {
  zeebe: { persistentVolumeClaimRetentionPolicy: { whenDeleted: "Retain" } },
};
const retaining = object(
  "CamundaCluster",
  "orders",
  applied({ spec: retainSpec }),
);

const data = (over: Partial<DeleteClusterData> = {}): DeleteClusterData => ({
  claim: claim(),
  cluster: object("CamundaCluster", "orders", applied()),
  environments: [],
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
      ...made("orders-backup-storage"),
      status: {
        resources: [
          { kind: "S3Bucket", name: "orders-backups", onOrphan: "Kept" },
        ],
      },
    }),
  ],
  optimizes: [
    object("CamundaOptimize", "orders-optimize", made("orders-optimize")),
  ],
  schedules: [
    object("BackupSchedule", "orders-backups", made("orders-backups")),
    object("BackupSchedule", "payments-backups", of("payments")),
  ],
  backups: { elasticsearch: [], rdbms: [] },
  presets: [],
  releases: [],
  errors: [],
  ...over,
});

describe("deleteClusterModel", () => {
  it("lists Optimize and the schedule, then the cluster with its volumes, then the requests, then the claim", () => {
    const model = deleteClusterModel(data(), "Delete");
    expect(model).toMatchObject({ name: "orders", namespace: "team-a" });
    expect(model.targets.map((t) => t.label)).toEqual([
      "CamundaOptimize orders-optimize",
      "BackupSchedule orders-backups",
      "CamundaCluster orders",
      "Backup storage request orders-backup-storage",
      "CamundaClusterClaim orders",
    ]);
    expect(model.targets.map((t) => [t.kind, t.name])).toEqual([
      ["CamundaOptimize", "orders-optimize"],
      ["BackupSchedule", "orders-backups"],
      ["CamundaCluster", "orders"],
      ["PluginRequest", "orders-backup-storage"],
      ["CamundaClusterClaim", "orders"],
    ]);
    expect(model.targets[2].note).toBe("Volumes deleted");
  });

  it("a cluster that sets its volume policy in its own spec says so, and one that takes it from its preset says it is not known", () => {
    const note = (d: DeleteClusterData) =>
      deleteClusterModel(d, "Delete").targets.find(
        (t) => t.kind === "CamundaCluster",
      )?.note;
    expect(note(data({ cluster: retaining }))).toBe(
      "Volumes kept for a cluster of the same name",
    );
    expect(
      note(
        data({
          cluster: object(
            "CamundaCluster",
            "orders",
            applied({ spec: { presetRef: "retaining" } }),
          ),
          presets: [
            {
              metadata: { name: "retaining" },
              spec: { cluster: retaining.spec },
            } as unknown as Resource<"CamundaClusterPreset">,
          ],
        }),
      ),
    ).toBe(
      "Volumes not known: the cluster takes its volume policy from its preset, and the page cannot tell if the operator applied it",
    );
    expect(
      note(
        data({
          cluster: object(
            "CamundaCluster",
            "orders",
            applied({ spec: { presetRef: "plain" } }),
          ),
          presets: [
            {
              metadata: { name: "plain" },
              spec: { cluster: {} },
            } as unknown as Resource<"CamundaClusterPreset">,
          ],
        }),
      ),
    ).toBe(
      "Volumes not known: the cluster takes its volume policy from its preset, and the page cannot tell if the operator applied it",
    );
    expect(note(data())).toBe("Volumes deleted");
  });

  it("says the volumes are not known until the operator applied the current spec of the cluster", () => {
    const note = (cluster: object) =>
      deleteClusterModel(
        data({ cluster: object("CamundaCluster", "orders", cluster) }),
        "Delete",
      ).targets.find((t) => t.kind === "CamundaCluster")?.note;
    const pending =
      "Volumes not known: the operator has not applied the current spec of the cluster yet";
    const refused =
      "Volumes not known: the Ready condition of the cluster reports a problem that can block its spec";
    expect(note(applied({ spec: retainSpec }, {}, 2))).toBe(pending);
    expect(note(applied({}, {}, 2))).toBe(pending);
    expect(
      note({
        metadata: { ...made("orders").metadata, generation: 1 },
        spec: retainSpec,
      }),
    ).toBe(pending);
    for (const ready of [
      "InvalidReference",
      "MissingSecret",
      "VersionDowngradeRefused",
      "VersionSkipRefused",
    ])
      expect(note(applied({ spec: retainSpec }, { ready }))).toBe(refused);
    expect(note(applied({ spec: retainSpec }, { zeebe: "Error" }))).toBe(
      refused,
    );
    expect(
      note(
        applied(
          { spec: retainSpec },
          { ready: "SuspensionHeld", zeebe: "Suspended" },
        ),
      ),
    ).toBe("Volumes kept for a cluster of the same name");
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

  it("keeps an Optimize or a schedule of the cluster that the claim does not control, with the reason", () => {
    const model = deleteClusterModel(
      data({
        optimizes: [object("CamundaOptimize", "orders-optimize", of("orders"))],
        schedules: [
          object("BackupSchedule", "orders-backups", made("orders-backups")),
          object("BackupSchedule", "orders-hourly", of("orders")),
        ],
      }),
      "Delete",
    );
    expect(model.targets.map((t) => t.label)).toEqual([
      "BackupSchedule orders-backups",
      "CamundaCluster orders",
      "Backup storage request orders-backup-storage",
      "CamundaClusterClaim orders",
    ]);
    expect(model.kept.map((t) => [t.label, t.note])).toEqual([
      ["CamundaOptimize orders-optimize", KEPT_ATTACHMENT],
      ["BackupSchedule orders-hourly", KEPT_ATTACHMENT],
    ]);
  });

  it("keeps a live cluster that the claim does not control, also while status.cluster names it", () => {
    const model = deleteClusterModel(
      data({ cluster: object("CamundaCluster", "orders") }),
      "Delete",
    );
    expect(model.targets.map((t) => t.kind)).not.toContain("CamundaCluster");
    expect(model.kept.map((t) => [t.label, t.note])).toEqual([
      ["CamundaCluster orders", KEPT_ATTACHMENT],
    ]);
  });

  it("deletes a live cluster that the claim controls, also before status.cluster names it", () => {
    const model = deleteClusterModel(
      data({ claim: claim({}, { cluster: undefined }) }),
      "Delete",
    );
    expect(model.targets.map((t) => t.kind)).toContain("CamundaCluster");
    expect(model.kept).toEqual([]);
  });

  it("without a cluster to read, deletes the cluster that status.cluster names", () => {
    const model = deleteClusterModel(data({ cluster: undefined }), "Delete");
    expect(model.targets.map((t) => t.kind)).toContain("CamundaCluster");
  });

  it("leaves out a cluster that is gone and that the claim does not name", () => {
    const model = deleteClusterModel(
      data({ claim: claim({}, { cluster: undefined }), cluster: undefined }),
      "Delete",
    );
    expect([...model.targets, ...model.kept].map((t) => t.kind)).not.toContain(
      "CamundaCluster",
    );
  });

  it("a failed read of the requests leaves the fates and the warning not known", () => {
    const model = deleteClusterModel(data(), "Delete", ["PluginRequest"]);
    expect(model.fatesKnown).toBe(false);
    expect(model.warning).toBeUndefined();
    expect(model.unread).toEqual(["PluginRequest"]);
    expect(deleteClusterModel(data(), "Delete").fatesKnown).toBe(true);
    expect(
      deleteClusterModel(data(), "Delete", ["PluginRegistration"]).fatesKnown,
    ).toBe(false);
  });

  it("a failed read of the cluster leaves its volumes not known", () => {
    const model = deleteClusterModel(data({ cluster: retaining }), "Delete", [
      "CamundaCluster",
    ]);
    expect(model.targets.find((t) => t.kind === "CamundaCluster")?.note).toBe(
      "Volumes not known: the page could not read what decides them",
    );
  });

  it("takes the requests that the claim controls, whatever their labels", () => {
    const model = deleteClusterModel(
      data({
        requests: [
          object("PluginRequest", "orders-backup-storage", {
            metadata: {
              namespace: "team-a",
              name: "orders-backup-storage",
              ownerReferences: [{ controller: true, uid: "plane-uid" }],
            },
            status: {
              resources: [
                { kind: "S3Bucket", name: "other", onOrphan: "Kept" },
              ],
            },
          }),
        ],
      }),
      "Delete",
    );
    expect(model.groups[0].rows).toEqual([]);
    expect(model.targets.map((t) => t.kind)).not.toContain("PluginRequest");
    expect(
      deleteClusterModel(data(), "Delete").targets.map((t) => t.kind),
    ).toContain("PluginRequest");
  });

  it("labels a request that the claim status does not name yet by the section of its spec", () => {
    const model = deleteClusterModel(
      data({
        claim: claim({}, { requests: [] }),
        requests: [
          object("PluginRequest", "orders-backup-storage", {
            ...made("orders-backup-storage"),
            spec: { section: "backup-storage", registration: "aws-s3" },
          }),
        ],
      }),
      "Delete",
    );
    expect(model.targets.find((t) => t.kind === "PluginRequest")?.label).toBe(
      "Backup storage request orders-backup-storage",
    );
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
    expect(initialPolicy(clusterDeletionChoice(refused, []))).toBe("Delete");
    expect(deleteClusterModel(data(), "Delete").keepRefused).toBe(false);
  });
});

describe("clusterDeletionChoice and initialPolicy", () => {
  it("open on the policy of the claim, else on the policy that it recorded", () => {
    expect(
      initialPolicy(
        clusterDeletionChoice(
          data({ claim: claim({ deletionPolicy: "Orphan" }) }),
          [],
        ),
      ),
    ).toBe("Orphan");
    expect(initialPolicy(clusterDeletionChoice(data(), []))).toBe("Delete");
  });
});

describe("the deletion choice of a claim whose registration does not exist", () => {
  const gone = () =>
    data({
      claim: claim(
        { deletionPolicy: "Orphan" },
        { resolvedPolicy: { deletionPolicy: "Delete" } },
      ),
      registrations: [],
    });

  it("refuses Keep and names the selection", () => {
    const model = deleteClusterModel(gone(), "Orphan");
    expect(model.choice.keepAllowed).toBe(false);
    expect(model.choice.missing).toEqual([
      { section: "backup-storage", label: "aws-s3 (Backup storage)" },
    ]);
    expect(initialPolicy(clusterDeletionChoice(gone(), []))).toBe("Delete");
  });

  it("does not count the registration as missing while PluginRegistration is not read", () => {
    const model = deleteClusterModel(gone(), "Orphan", ["PluginRegistration"]);
    expect(model.choice.keepAllowed).toBe(true);
    expect(model.choice.missing).toBeUndefined();
  });
});

describe("mergeTargets", () => {
  it("keeps each target and adds the new ones in the order in which the kinds go", () => {
    const t = (
      kind: "CamundaCluster" | "PluginRequest" | "BackupSchedule",
      name: string,
    ) => ({
      kind,
      name,
      label: `${kind} ${name}`,
    });
    expect(
      mergeTargets(
        [t("CamundaCluster", "orders"), t("PluginRequest", "gone")],
        [
          t("PluginRequest", "orders-access"),
          t("BackupSchedule", "orders-backups"),
        ],
      ).map((x) => x.label),
    ).toEqual([
      "BackupSchedule orders-backups",
      "CamundaCluster orders",
      "PluginRequest gone",
      "PluginRequest orders-access",
    ]);
  });
});

describe("mergeTargets of a management plane", () => {
  it("keeps the order in which the delete of a plane claim goes: the plane, the requests, then the claim", () => {
    const t = (
      kind:
        | "CamundaManagementCluster"
        | "PluginRequest"
        | "CamundaManagementClusterClaim",
      name: string,
    ) => ({ kind, name, label: `${kind} ${name}` });
    expect(
      mergeTargets(
        [],
        [
          t("CamundaManagementClusterClaim", "plane-a"),
          t("PluginRequest", "plane-a-management-database"),
          t("CamundaManagementCluster", "plane-a"),
        ],
      ).map((x) => x.kind),
    ).toEqual([
      "CamundaManagementCluster",
      "PluginRequest",
      "CamundaManagementClusterClaim",
    ]);
  });
});

describe("mergeTargets refresh", () => {
  it("takes the note of a target from found and keeps a target that found lacks", () => {
    const t = (name: string, note?: string) => ({
      kind: "CamundaCluster" as const,
      name,
      label: `CamundaCluster ${name}`,
      ...(note && { note }),
    });
    expect(
      mergeTargets(
        [t("orders", "Volumes not known"), t("gone", "Volumes stay")],
        [t("orders", "Volumes are deleted")],
      ).map((x) => x.note),
    ).toEqual(["Volumes are deleted", "Volumes stay"]);
  });
});

describe("a cluster that the page cannot see", () => {
  it("has no volume note, so that the stored note stays", () => {
    const model = deleteClusterModel(data({ cluster: undefined }), "Delete");
    expect(
      model.targets.find((t) => t.kind === "CamundaCluster")?.note,
    ).toBeUndefined();
    expect(
      mergeTargets(
        [
          {
            kind: "CamundaCluster",
            name: "orders",
            label: "CamundaCluster orders",
            note: "Volumes kept for a cluster of the same name",
          },
        ],
        model.targets,
      ).find((t) => t.kind === "CamundaCluster")?.note,
    ).toBe("Volumes kept for a cluster of the same name");
  });
});

describe("deleteProgress", () => {
  it("a live object is Waiting, one being deleted is Deleting, and a gone one is Deleted, in target order", () => {
    const model = deleteClusterModel(data(), "Delete");
    const progress = deleteProgress(model.targets, {
      CamundaOptimize: [],
      BackupSchedule: [{ metadata: { name: "payments-backups" } }],
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
      ["BackupSchedule orders-backups", "Deleted"],
      ["CamundaCluster orders", "Deleting"],
      ["Backup storage request orders-backup-storage", "Waiting"],
      ["CamundaClusterClaim orders", "Deleting"],
    ]);
  });
});
