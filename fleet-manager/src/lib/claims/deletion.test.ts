import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { backupWarning, deleteClaimSteps, deletionGroups } from "./deletion";
import type { ResourceRow } from "./resources";

const claim = (
  spec: { deletionPolicy?: "Delete" | "Orphan" } = {},
  resolved?: "Delete" | "Orphan",
) => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: {
    namespace: "team-a",
    name: "orders",
    resourceVersion: "41",
    uid: "uid-1",
    generation: 3,
  },
  spec: { releaseRef: "camunda-8-9-1", ...spec },
  ...(resolved && { status: { resolvedPolicy: { deletionPolicy: resolved } } }),
});
const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  namespace: "team-a",
  name: "orders",
};
const key = "CamundaClusterClaim/team-a/orders";

describe("deleteClaimSteps", () => {
  it("removes the claim by its UID and generation, and writes no policy when the claim policy is the choice", () => {
    expect(
      deleteClaimSteps(
        claim({ deletionPolicy: "Orphan" }),
        "Orphan",
        "Delete the cluster orders",
      ),
    ).toEqual({
      remove: {
        intent: "Delete the cluster orders",
        apply: [],
        delete: [ref],
        uids: { [key]: "uid-1" },
        generations: { [key]: 3 },
      },
    });
  });

  it("reads the recorded policy, else Delete, when the claim sets none", () => {
    expect(
      deleteClaimSteps(claim({}, "Orphan"), "Orphan", "x").policy,
    ).toBeUndefined();
    expect(deleteClaimSteps(claim(), "Delete", "x").policy).toBeUndefined();
  });

  it("writes the policy first, as the whole claim with the resourceVersion that the page read", () => {
    const { policy } = deleteClaimSteps(
      claim(),
      "Orphan",
      "Delete the cluster orders",
    );
    expect(policy).toEqual({
      intent: "Set the deletion policy of orders to Orphan before the delete",
      apply: [
        {
          ...claim(),
          spec: { releaseRef: "camunda-8-9-1", deletionPolicy: "Orphan" },
        },
      ],
      preconditions: { [key]: "41" },
    });
  });

  it("removes a plane claim by its own kind", () => {
    const plane = { ...claim(), kind: "CamundaManagementClusterClaim" };
    expect(deleteClaimSteps(plane, "Delete", "x").remove).toEqual({
      intent: "x",
      apply: [],
      delete: [{ ...ref, kind: "CamundaManagementClusterClaim" }],
      uids: { "CamundaManagementClusterClaim/team-a/orders": "uid-1" },
      generations: { "CamundaManagementClusterClaim/team-a/orders": 3 },
    });
  });

  it("leaves out uids and generations for a claim without a UID and a generation", () => {
    const { metadata } = claim();
    const remove = deleteClaimSteps(
      {
        ...claim(),
        metadata: { ...metadata, uid: undefined, generation: undefined },
      },
      "Delete",
      "x",
    ).remove;
    expect(remove).toEqual({ intent: "x", apply: [], delete: [ref] });
  });
});

const section = (
  name: string,
  label: string,
  order: number,
  slot?: string,
  selection: "One" | "Many" = "One",
) =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: {
      label,
      order,
      selection,
      appliesTo: ["CamundaClusterClaim"],
      ...(slot && { slot }),
    },
  }) as unknown as Resource<"PluginSection">;

const registration = (name: string, deletionPolicies: string[]) =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: { label: name, deletionPolicies },
  }) as unknown as Resource<"PluginRegistration">;

const request = (
  name: string,
  resources?: ResourceRow[],
  controller = "claim-uid",
) =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRequest",
    metadata: {
      namespace: "team-a",
      name,
      ownerReferences: [{ controller: true, uid: controller }],
    },
    status: resources ? { resources } : {},
  }) as unknown as Resource<"PluginRequest">;

const sections = [
  section("backup-storage", "Backup storage", 30, "backupStorage"),
  section("encryption-key", "Encryption key", 10),
  section("volume-encryption", "Volume encryption", 20, "volumeEncryption"),
  section("secondary-storage", "Secondary storage", 40, "secondaryStorage"),
];
const registrations = [
  registration("aws-kms", ["Delete", "Orphan"]),
  registration("aws-s3", ["Delete", "Orphan"]),
  registration("ebs-plain", ["Delete"]),
  registration("existing-object-storage", ["Delete", "Orphan"]),
  registration("elasticsearch", ["Delete", "Orphan"]),
];
const key1: ResourceRow = {
  kind: "KMSKey",
  name: "orders-key",
  id: "arn:aws:kms:eu-west-1:1:key/1",
  onOrphan: "Kept",
};
const storageClass: ResourceRow = {
  kind: "StorageClass",
  name: "orders-encrypted",
  onOrphan: "Deleted",
};
const bucket = (onOrphan: "Kept" | "Deleted" = "Kept"): ResourceRow => ({
  kind: "S3Bucket",
  name: "orders-backups",
  onOrphan,
});

const claimOf = (
  plugins: { section: string; registration: string }[],
  names: Record<string, string> = {},
) => ({
  metadata: { namespace: "team-a", uid: "claim-uid" },
  spec: { plugins },
  status: {
    requests: plugins.flatMap((p) =>
      names[p.section] ? [{ ...p, name: names[p.section] }] : [],
    ),
  },
});

const aws = claimOf(
  [
    { section: "backup-storage", registration: "aws-s3" },
    { section: "volume-encryption", registration: "ebs-plain" },
    { section: "encryption-key", registration: "aws-kms" },
  ],
  {
    "backup-storage": "orders-backup-storage",
    "volume-encryption": "orders-volume-encryption",
    "encryption-key": "orders-encryption-key",
  },
);
const awsRequests = (backup: ResourceRow[] = [bucket()]) => [
  request("orders-encryption-key", [key1]),
  request("orders-volume-encryption", [storageClass]),
  request("orders-backup-storage", backup),
];
const groupsOf = (
  policy: "Delete" | "Orphan",
  requests = awsRequests(),
  c: ReturnType<typeof claimOf> = aws,
) => deletionGroups({ claim: c, sections, registrations, requests, policy });

describe("deletionGroups", () => {
  it("groups the rows by section in section order, and the key under Encryption key", () => {
    const groups = groupsOf("Delete");
    expect(groups.map((g) => g.section)).toEqual([
      "encryption-key",
      "volume-encryption",
      "backup-storage",
    ]);
    expect(groups[0]).toMatchObject({
      id: "section:encryption-key",
      label: "Encryption key",
      registration: "aws-kms",
      rows: [{ ...key1, fate: "Deleted" }],
    });
    expect(groups[1]).toMatchObject({
      label: "Volume encryption",
      slot: "volumeEncryption",
      rows: [{ ...storageClass, fate: "Deleted" }],
    });
    expect(groups[2].slot).toBe("backupStorage");
    expect(groups[0].slot).toBeUndefined();
  });

  it("gives each row its fate under the policy", () => {
    const fates = (policy: "Delete" | "Orphan") =>
      groupsOf(policy).flatMap((g) => g.rows.map((r) => [r.kind, r.fate]));
    expect(fates("Orphan")).toEqual([
      ["KMSKey", "Kept"],
      ["StorageClass", "Deleted"],
      ["S3Bucket", "Kept"],
    ]);
    expect(fates("Delete")).toEqual([
      ["KMSKey", "Deleted"],
      ["StorageClass", "Deleted"],
      ["S3Bucket", "Deleted"],
    ]);
  });

  it("marks an option whose registration cannot keep", () => {
    const groups = groupsOf("Orphan");
    expect(
      Object.fromEntries(groups.map((g) => [g.registration, g.alwaysDeletes])),
    ).toEqual({ "aws-kms": false, "ebs-plain": true, "aws-s3": false });
  });

  it("a built-in storage request keeps its ElasticsearchCluster under Orphan", () => {
    const c = claimOf(
      [{ section: "secondary-storage", registration: "elasticsearch" }],
      { "secondary-storage": "orders-secondary-storage" },
    );
    const es: ResourceRow = {
      kind: "ElasticsearchCluster",
      name: "orders-es",
      onOrphan: "Kept",
    };
    expect(
      groupsOf("Orphan", [request("orders-secondary-storage", [es])], c)[0]
        .rows,
    ).toEqual([{ ...es, fate: "Kept" }]);
  });

  it("a selection without a request, or a request without rows, has no rows", () => {
    const c = claimOf(
      [
        { section: "encryption-key", registration: "aws-kms" },
        { section: "backup-storage", registration: "aws-s3" },
      ],
      { "backup-storage": "orders-backup-storage" },
    );
    const groups = groupsOf("Delete", [request("orders-backup-storage")], c);
    expect(groups.map((g) => g.rows)).toEqual([[], []]);
  });

  it("takes the request of a selection by its section and registration while the claim status does not name it", () => {
    const c = claimOf([{ section: "backup-storage", registration: "aws-s3" }]);
    const backup = {
      ...request("orders-backup-storage", [bucket()]),
      spec: { section: "backup-storage", registration: "aws-s3" },
    } as unknown as Resource<"PluginRequest">;
    const groups = groupsOf("Delete", [backup], c);
    expect(groups[0].rows).toEqual([{ ...bucket(), fate: "Deleted" }]);
    expect(backupWarning("orders", groups)).toBeDefined();
  });

  it("gives the rows of a request that is being deleted the fate of its own policy, whatever the pick", () => {
    const terminating = {
      ...request("orders-backup-storage", [bucket()]),
      metadata: {
        ...request("orders-backup-storage").metadata,
        deletionTimestamp: "2026-11-16T10:00:00Z",
      },
      spec: {
        section: "backup-storage",
        registration: "aws-s3",
        deletionPolicy: "Delete",
      },
    } as unknown as Resource<"PluginRequest">;
    const requests = [
      request("orders-encryption-key", [key1]),
      request("orders-volume-encryption", [storageClass]),
      terminating,
    ];
    const groups = groupsOf("Orphan", requests);
    expect(groups.map((g) => g.rows.map((r) => r.fate))).toEqual([
      ["Kept"],
      ["Deleted"],
      ["Deleted"],
    ]);
    expect(backupWarning("orders", groups)).toBeDefined();
  });

  it("takes no request that the claim does not control, also of the same section and registration", () => {
    const c = claimOf([{ section: "backup-storage", registration: "aws-s3" }], {
      "backup-storage": "orders-backup-storage",
    });
    const other = (name: string) =>
      ({
        ...request(name, [bucket()], "other-claim-uid"),
        spec: { section: "backup-storage", registration: "aws-s3" },
      }) as unknown as Resource<"PluginRequest">;
    expect(
      groupsOf("Delete", [other("orders-backup-storage")], c)[0].rows,
    ).toEqual([]);
    const unnamed = claimOf([
      { section: "backup-storage", registration: "aws-s3" },
    ]);
    const groups = groupsOf(
      "Delete",
      [other("billing-backup-storage")],
      unnamed,
    );
    expect(groups[0].rows).toEqual([]);
    expect(backupWarning("orders", groups)).toBeUndefined();
  });
});

describe("backupWarning", () => {
  it("warns under Delete when the backup request made a bucket", () => {
    expect(backupWarning("orders", groupsOf("Delete"))).toBe(
      "You cannot restore orders after this: the backup bucket goes too.",
    );
  });

  it("does not warn under Orphan when the bucket is kept", () => {
    expect(backupWarning("orders", groupsOf("Orphan"))).toBeUndefined();
  });

  it("warns under Orphan when the plugin deletes the bucket also then", () => {
    expect(
      backupWarning(
        "orders",
        groupsOf("Orphan", awsRequests([bucket("Deleted")])),
      ),
    ).toBe("You cannot restore orders after this: the backup bucket goes too.");
  });

  it("does not warn for an existing object storage contract, which made no rows", () => {
    const c = claimOf(
      [{ section: "backup-storage", registration: "existing-object-storage" }],
      { "backup-storage": "orders-backup-storage" },
    );
    expect(
      backupWarning(
        "orders",
        groupsOf("Delete", [request("orders-backup-storage")], c),
      ),
    ).toBeUndefined();
  });
});
