import { beforeEach, describe, expect, it, vi } from "vitest";
import { deleteClaimSteps } from "@/lib/claims/deletion";
import type { KubeObject, ObjectRef, WriteMode, Writer } from "./types";

const fake = {
  mode: "direct" as WriteMode,
  preview: vi.fn<Writer["preview"]>(),
  apply: vi.fn<Writer["apply"]>(),
};
// readLive answers with the objects of reads, one read after the other,
// and keeps the last one.
let reads: (KubeObject | undefined)[] = [];
const readLive = vi.fn<(ref: ObjectRef) => Promise<KubeObject | undefined>>(
  async () => (reads.length > 1 ? reads.shift() : reads[0]),
);
// listed are the objects that listObjects reads, of every kind.
let listed: KubeObject[] = [];
const listObjects = vi.fn(async (kind: string, namespace: string) =>
  listed.filter((o) => o.kind === kind && o.metadata.namespace === namespace),
);
vi.mock("./writer", () => ({
  getWriter: async () => fake,
  readLive,
  listObjects,
}));

const {
  adminChangeSchema,
  backupChangeSchema,
  changeSchema,
  claimBatchChangeSchema,
  claimChangeSchema,
  CLAIM_DELETE_ATTEMPTS,
  claimDeleteSchema,
  createChangeSchema,
  createClaim,
  namespaceLabelChangeSchema,
  previewClaimDelete,
  runChange,
  runClaimDelete,
  runPreview,
} = await import("./run-change");

const validChange = {
  intent: "Create the environment prod",
  apply: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "prod" },
      spec: {},
    },
  ],
};

const planeClaim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: { name: "plane-a", namespace: "platform" },
  spec: {},
};
const planeKey = "CamundaManagementClusterClaim/platform/plane-a";

beforeEach(() => {
  fake.mode = "direct";
  fake.apply.mockReset();
  fake.preview.mockReset();
  readLive.mockClear();
  reads = [];
  listObjects.mockClear();
  listed = [];
});

describe("runChange", () => {
  it("applies a valid change with the writer and returns its result", async () => {
    fake.apply.mockResolvedValue({ ok: true, mode: "direct" });

    expect(await runChange(validChange)).toEqual({ ok: true, mode: "direct" });
    expect(fake.apply).toHaveBeenCalledWith(validChange);
  });

  it("refuses input that does not match the schema and writes nothing", async () => {
    const result = await runChange({
      intent: "",
      apply: [{ kind: "Environment", metadata: { name: "prod" } }],
    });

    expect(result).toMatchObject({
      ok: false,
      errors: expect.arrayContaining([
        expect.objectContaining({ code: "Invalid", path: "intent" }),
        expect.objectContaining({
          code: "Invalid",
          path: "apply.0.apiVersion",
        }),
      ]),
    });
    expect(fake.apply).not.toHaveBeenCalled();
  });

  it("refuses a change with nothing to apply or delete", async () => {
    const result = await runChange({ intent: "Nothing", apply: [] });

    expect(result).toMatchObject({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect(fake.apply).not.toHaveBeenCalled();
  });

  it("uses the schema of the action", async () => {
    const onlyEnvironments = changeSchema.refine(
      (c) => c.apply.every((o) => o.kind === "Environment"),
      { message: "only environments", path: ["apply"] },
    );

    expect(
      await runChange(
        {
          intent: "x",
          apply: [
            { apiVersion: "v1", kind: "Namespace", metadata: { name: "a" } },
          ],
        },
        onlyEnvironments,
      ),
    ).toEqual({
      ok: false,
      errors: [
        { code: "Invalid", message: "only environments", path: "apply" },
      ],
    });
    expect(fake.apply).not.toHaveBeenCalled();
  });

  it("returns Unavailable when the writer throws", async () => {
    fake.apply.mockRejectedValue(new Error("boom"));

    expect(await runChange(validChange)).toEqual({
      ok: false,
      errors: [{ code: "Unavailable", message: "boom" }],
    });
  });
});

describe("runPreview", () => {
  it("previews a valid change with the writer and returns its preview", async () => {
    const preview = { objects: [], errors: [] };
    fake.preview.mockResolvedValue(preview);

    expect(await runPreview(validChange)).toEqual(preview);
    expect(fake.preview).toHaveBeenCalledWith(validChange);
  });

  it("refuses input that the schema refuses and previews nothing", async () => {
    const result = await runPreview({ intent: "", apply: [] });

    expect(result.objects).toEqual([]);
    expect(result.errors.length).toBeGreaterThan(0);
    expect(result.errors.every((e) => e.code === "Invalid")).toBe(true);
    expect(fake.preview).not.toHaveBeenCalled();
  });

  it("uses the schema of the action", async () => {
    const result = await runPreview(
      validChange,
      claimChangeSchema("CamundaClusterClaim"),
    );

    expect(result.errors).toEqual([
      expect.objectContaining({ code: "Invalid" }),
    ]);
    expect(fake.preview).not.toHaveBeenCalled();
  });

  it("returns Unavailable when the writer throws", async () => {
    fake.preview.mockRejectedValue(new Error("settings unreadable"));

    expect(await runPreview(validChange)).toEqual({
      objects: [],
      errors: [{ code: "Unavailable", message: "settings unreadable" }],
    });
  });
});

describe("claimChangeSchema", () => {
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a" },
    spec: {},
  };

  it("accepts one claim of its kind to apply, with a precondition, and refuses a claim of the other kind", () => {
    expect(
      claimChangeSchema("CamundaClusterClaim").safeParse({
        intent: "x",
        apply: [claim],
        preconditions: { "CamundaClusterClaim/team-a/orders": "41" },
      }).success,
    ).toBe(true);
    const plane = { ...claim, kind: "CamundaManagementClusterClaim" };
    expect(
      claimChangeSchema("CamundaManagementClusterClaim").safeParse({
        intent: "x",
        apply: [plane],
      }).success,
    ).toBe(true);
    expect(
      claimChangeSchema("CamundaClusterClaim").safeParse({
        intent: "x",
        apply: [plane],
      }).success,
    ).toBe(false);
    expect(
      claimChangeSchema("CamundaManagementClusterClaim").safeParse({
        intent: "x",
        apply: [claim],
      }).success,
    ).toBe(false);
  });

  it.each([
    [
      "another kind",
      { intent: "x", apply: [{ ...claim, kind: "Environment" }] },
    ],
    ["two objects", { intent: "x", apply: [claim, claim] }],
    ["no object", { intent: "x", apply: [] }],
    [
      "a delete",
      {
        intent: "x",
        apply: [claim],
        delete: [
          { apiVersion: "v1", kind: "Secret", namespace: "a", name: "b" },
        ],
      },
    ],
    ["force", { intent: "x", apply: [claim], force: true }],
    ["directOnly", { intent: "x", apply: [claim], directOnly: true }],
    [
      "no namespace",
      { intent: "x", apply: [{ ...claim, metadata: { name: "orders" } }] },
    ],
    [
      "create",
      {
        intent: "x",
        apply: [claim],
        create: ["CamundaClusterClaim/team-a/orders"],
      },
    ],
  ])("refuses %s", (_, input) => {
    expect(
      claimChangeSchema("CamundaClusterClaim").safeParse(input).success,
    ).toBe(false);
  });
});

describe("claimDeleteSchema", () => {
  const ref = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    namespace: "team-a",
    name: "orders",
  };
  const remove = {
    intent: "Delete the cluster orders",
    apply: [],
    delete: [ref],
    uids: { "CamundaClusterClaim/team-a/orders": "uid-1" },
    generations: { "CamundaClusterClaim/team-a/orders": 3 },
  };

  it("accepts one delete of a claim of its kind with its UID and generation, and refuses a claim of the other kind", () => {
    expect(
      claimDeleteSchema("CamundaClusterClaim").safeParse(remove).success,
    ).toBe(true);
    const planeKey = "CamundaManagementClusterClaim/team-a/orders";
    const plane = {
      ...remove,
      delete: [{ ...ref, kind: "CamundaManagementClusterClaim" }],
      uids: { [planeKey]: "uid-1" },
      generations: { [planeKey]: 3 },
    };
    expect(
      claimDeleteSchema("CamundaManagementClusterClaim").safeParse(plane)
        .success,
    ).toBe(true);
    expect(
      claimDeleteSchema("CamundaClusterClaim").safeParse(plane).success,
    ).toBe(false);
    expect(
      claimDeleteSchema("CamundaManagementClusterClaim").safeParse(remove)
        .success,
    ).toBe(false);
  });

  it.each([
    ["no UID", { ...remove, uids: undefined }],
    ["no generation", { ...remove, generations: undefined }],
    [
      "a generation that is not a positive integer",
      {
        ...remove,
        generations: { "CamundaClusterClaim/team-a/orders": 0 },
      },
    ],
    [
      "a resourceVersion precondition",
      {
        ...remove,
        preconditions: { "CamundaClusterClaim/team-a/orders": "41" },
      },
    ],
    ["another kind", { ...remove, delete: [{ ...ref, kind: "Environment" }] }],
    [
      "another apiVersion",
      { ...remove, delete: [{ ...ref, apiVersion: "core.camunda.io/v1" }] },
    ],
    ["no namespace", { ...remove, delete: [{ ...ref, namespace: undefined }] }],
    [
      "the name ..",
      {
        ...remove,
        delete: [{ ...ref, name: ".." }],
        uids: { "CamundaClusterClaim/team-a/..": "uid-1" },
        generations: { "CamundaClusterClaim/team-a/..": 3 },
      },
    ],
    [
      "the namespace .",
      {
        ...remove,
        delete: [{ ...ref, namespace: "." }],
        uids: { "CamundaClusterClaim/./orders": "uid-1" },
        generations: { "CamundaClusterClaim/./orders": 3 },
      },
    ],
    ["two deletes", { ...remove, delete: [ref, { ...ref, name: "payments" }] }],
    [
      "an apply",
      {
        ...remove,
        apply: [
          {
            apiVersion: ref.apiVersion,
            kind: ref.kind,
            metadata: { name: "orders", namespace: "team-a" },
          },
        ],
      },
    ],
    [
      "a generation of another object",
      {
        ...remove,
        generations: { "CamundaClusterClaim/team-a/payments": 3 },
      },
    ],
    ["create", { ...remove, create: ["CamundaClusterClaim/team-a/orders"] }],
    ["force", { ...remove, force: true }],
    ["directOnly", { ...remove, directOnly: true }],
    [
      "a UID of another object",
      { ...remove, uids: { "CamundaClusterClaim/team-a/payments": "uid-2" } },
    ],
  ])("refuses %s", (_, change) => {
    expect(
      claimDeleteSchema("CamundaClusterClaim").safeParse(change).success,
    ).toBe(false);
  });

  it("claimChangeSchema still refuses the delete", () => {
    expect(
      claimChangeSchema("CamundaClusterClaim").safeParse(remove).success,
    ).toBe(false);
  });

  it("accepts the remove step of deleteClaimSteps, and claimChangeSchema accepts its policy step", () => {
    const steps = deleteClaimSteps(
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaClusterClaim",
        metadata: {
          namespace: "team-a",
          name: "orders",
          resourceVersion: "41",
          uid: "uid-1",
          generation: 3,
        },
        spec: {},
      },
      "Orphan",
      "Delete the cluster orders",
    );
    expect(
      claimDeleteSchema("CamundaClusterClaim").safeParse(steps.remove).success,
    ).toBe(true);
    expect(
      claimChangeSchema("CamundaClusterClaim").safeParse(steps.policy).success,
    ).toBe(true);
  });
});

describe("runClaimDelete and previewClaimDelete", () => {
  const key = "CamundaClusterClaim/team-a/orders";
  const ref = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    namespace: "team-a",
    name: "orders",
  };
  const remove = {
    intent: "Delete the cluster orders",
    apply: [],
    delete: [ref],
    uids: { [key]: "uid-1" },
    generations: { [key]: 3 },
  };
  const live = (
    resourceVersion: string,
    over: { uid?: string; generation?: number } = {},
  ): KubeObject => ({
    apiVersion: ref.apiVersion,
    kind: ref.kind,
    metadata: {
      name: "orders",
      namespace: "team-a",
      uid: "uid-1",
      generation: 3,
      resourceVersion,
      ...over,
    },
  });
  const stale = {
    ok: false as const,
    errors: [{ code: "Stale" as const, message: "Changed.", ref }],
  };
  // set is a live claim of generation 3 that the operator set up and read:
  // it has the cleanup finalizer, a recorded policy, its own policy, and
  // a Ready condition.
  const set = (
    policy: "Delete" | "Orphan",
    over: { observedGeneration?: number; finalizers?: string[] } = {},
  ): KubeObject => {
    const c = live("45");
    return {
      ...c,
      metadata: {
        ...c.metadata,
        finalizers: over.finalizers ?? ["fleet.camunda.io/cleanup"],
      },
      spec: { deletionPolicy: policy },
      status: {
        resolvedPolicy: { deletionPolicy: "Delete" },
        ...("observedGeneration" in over
          ? over.observedGeneration !== undefined && {
              observedGeneration: over.observedGeneration,
            }
          : { observedGeneration: 3 }),
        conditions: [
          {
            type: "Ready",
            status: "False",
            reason: "InvalidReference",
            message: "CamundaRelease r1 does not exist",
          },
        ],
      },
    };
  };
  const request = (
    name: string,
    policy: "Delete" | "Orphan",
    over: {
      uid?: string;
      deletionTimestamp?: string;
      controller?: boolean;
    } = {},
  ): KubeObject => ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRequest",
    metadata: {
      name,
      namespace: "team-a",
      ownerReferences: [
        { controller: over.controller ?? true, uid: over.uid ?? "uid-1" },
      ],
      ...(over.deletionTimestamp && {
        deletionTimestamp: over.deletionTimestamp,
      }),
    },
    spec: { deletionPolicy: policy },
  });

  it("direct mode deletes a claim with the cleanup finalizer whose requests still have the old policy, also while its precheck fails", async () => {
    reads = [set("Orphan")];
    listed = [request("orders-backup-storage", "Delete")];
    fake.apply.mockResolvedValue({ ok: true, mode: "direct" });

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: true,
      mode: "direct",
    });
  });

  it("Git mode refuses while a request of the claim does not have its policy, names it, and gives the Ready reason of the claim", async () => {
    fake.mode = "git";
    reads = [set("Orphan")];
    listed = [
      request("orders-backup-storage", "Delete"),
      request("orders-access", "Orphan"),
    ];

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: false,
      errors: [
        {
          code: "Stale",
          message:
            "The request orders-backup-storage does not have the deletion policy of the claim. The operator does not update it while the claim reports InvalidReference: CamundaRelease r1 does not exist",
          ref,
        },
      ],
    });
    expect(listObjects).toHaveBeenCalledWith("PluginRequest", "team-a");
    expect(fake.apply).not.toHaveBeenCalled();
  });

  it("names only the requests of the claim that lack its policy, not requests of other claims, requests being deleted, or requests without the controller flag", async () => {
    fake.mode = "git";
    reads = [set("Orphan")];
    listed = [
      request("orders-backup-storage", "Delete"),
      request("orders-access", "Delete"),
      request("payments-backup-storage", "Delete", { uid: "uid-2" }),
      request("orders-old", "Delete", {
        deletionTimestamp: "2026-11-16T10:00:00Z",
      }),
      request("orders-borrowed", "Delete", { controller: false }),
    ];

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toMatchObject({
      ok: false,
      errors: [
        {
          message: expect.stringMatching(
            /^The requests orders-backup-storage, orders-access do not have/,
          ),
        },
      ],
    });
  });

  it.each([
    ["has no observedGeneration but a recorded policy", undefined],
    ["has an older observedGeneration", 2],
  ])(
    "Git mode refuses while the claim %s, also with no request",
    async (_, observedGeneration) => {
      fake.mode = "git";
      reads = [set("Orphan", { observedGeneration })];

      expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
        ok: false,
        errors: [
          {
            code: "Stale",
            message:
              "The operator has not read the deletion policy of the CamundaClusterClaim orders yet. Try again in a moment.",
            ref,
          },
        ],
      });
      expect(fake.apply).not.toHaveBeenCalled();
    },
  );

  it("a claim without the cleanup finalizer and without a recorded policy gets the request check in direct mode too", async () => {
    const bare = live("45");
    reads = [{ ...bare, spec: { deletionPolicy: "Orphan" } }];
    listed = [request("orders-backup-storage", "Delete")];

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toMatchObject({
      ok: false,
      errors: [{ code: "Stale" }],
    });
    expect(fake.apply).not.toHaveBeenCalled();
  });

  it("gives Unavailable and deletes nothing when the requests cannot be read", async () => {
    fake.mode = "git";
    reads = [set("Orphan")];
    listObjects.mockRejectedValueOnce(new Error("forbidden"));

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: false,
      errors: [{ code: "Unavailable", message: "forbidden" }],
    });
    expect(fake.apply).not.toHaveBeenCalled();
  });

  it("the preview does not wait for the requests, so a policy write that corrects them can follow it", async () => {
    // An option that cannot keep holds its request at Delete under a claim
    // that says Orphan.
    fake.mode = "git";
    reads = [set("Orphan")];
    listed = [request("orders-backup-storage", "Delete")];
    fake.preview.mockResolvedValue({ objects: [], errors: [] });

    expect(await previewClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      objects: [],
      errors: [],
    });
    expect(fake.preview).toHaveBeenCalledWith(remove);
  });

  it("refuses, and the preview too, in both modes a claim that lost its cleanup finalizer after the operator set it up", async () => {
    reads = [set("Orphan", { finalizers: [] })];
    const refusal = {
      code: "Invalid",
      message:
        "The CamundaClusterClaim orders has no finalizer fleet.camunda.io/cleanup, so the operator cannot give the deletion policy to its options. Add the finalizer back, then delete the claim again.",
      ref,
    };
    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: false,
      errors: [refusal],
    });
    expect(await previewClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      objects: [],
      errors: [refusal],
    });
    fake.mode = "git";
    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: false,
      errors: [refusal],
    });
    expect(fake.apply).not.toHaveBeenCalled();
    expect(fake.preview).not.toHaveBeenCalled();
  });

  it("does not ask for the finalizer of a claim that is already being deleted", async () => {
    const deleting = set("Orphan", { finalizers: [] });
    deleting.metadata = {
      ...deleting.metadata,
      deletionTimestamp: "2026-11-16T10:00:00Z",
    };
    reads = [deleting];
    fake.apply.mockResolvedValue({ ok: true, mode: "direct" });

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: true,
      mode: "direct",
    });
  });

  it("deletes at the resourceVersion of a fresh read when only the status changed since the review", async () => {
    reads = [live("45")];
    fake.apply.mockResolvedValue({ ok: true, mode: "direct" });

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(readLive).toHaveBeenCalledWith(ref);
    expect(fake.apply).toHaveBeenCalledTimes(1);
    expect(fake.apply).toHaveBeenCalledWith({
      ...remove,
      preconditions: { [key]: "45" },
    });
  });

  it.each([
    ["the spec changed", live("45", { generation: 4 })],
    ["the claim was made again", live("45", { uid: "uid-2" })],
    ["the claim is gone", undefined],
  ])("refuses with Stale and deletes nothing when %s", async (_, object) => {
    reads = [object];

    const result = await runClaimDelete(remove, "CamundaClusterClaim");

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale", ref })],
    });
    expect(fake.apply).not.toHaveBeenCalled();
  });

  it("reads again after a conflict and deletes at the newer resourceVersion", async () => {
    reads = [live("45"), live("46")];
    fake.apply
      .mockResolvedValueOnce(stale)
      .mockResolvedValueOnce({ ok: true, mode: "direct" });

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fake.apply.mock.calls.map(([c]) => c.preconditions)).toEqual([
      { [key]: "45" },
      { [key]: "46" },
    ]);
  });

  it("refuses with Stale after a conflict when the spec changed since the review", async () => {
    reads = [live("45"), live("46", { generation: 4 })];
    fake.apply.mockResolvedValue(stale);

    const result = await runClaimDelete(remove, "CamundaClusterClaim");

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale", ref })],
    });
    expect(fake.apply).toHaveBeenCalledTimes(1);
  });

  it("stops after CLAIM_DELETE_ATTEMPTS conflicts and says that the claim kept changing", async () => {
    reads = [live("45")];
    fake.apply.mockResolvedValue(stale);

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: false,
      errors: [
        {
          code: "Stale",
          message:
            "The CamundaClusterClaim orders kept changing during the delete. Delete it again.",
          ref,
        },
      ],
    });
    expect(fake.apply).toHaveBeenCalledTimes(CLAIM_DELETE_ATTEMPTS);
  });

  it("returns a refusal that is not Stale at once", async () => {
    reads = [live("45")];
    const forbidden = {
      ok: false as const,
      errors: [{ code: "Forbidden" as const, message: "No.", ref }],
    };
    fake.apply.mockResolvedValue(forbidden);

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual(
      forbidden,
    );
    expect(fake.apply).toHaveBeenCalledTimes(1);
  });

  it("Git mode sends the change once without a resourceVersion, so the Git writer checks the generation", async () => {
    fake.mode = "git";
    reads = [live("45")];
    fake.apply.mockResolvedValue(stale);

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual(stale);
    expect(fake.apply).toHaveBeenCalledTimes(1);
    expect(fake.apply).toHaveBeenCalledWith(remove);
  });

  it.each([
    ["no generation", { ...remove, generations: undefined }],
    [
      "a name that is not a path segment",
      {
        ...remove,
        delete: [{ ...ref, name: ".." }],
        uids: { "CamundaClusterClaim/team-a/..": "uid-1" },
        generations: { "CamundaClusterClaim/team-a/..": 3 },
      },
    ],
  ])("refuses input with %s as Invalid and reads nothing", async (_, input) => {
    expect(await runClaimDelete(input, "CamundaClusterClaim")).toMatchObject({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect(
      await previewClaimDelete(input, "CamundaClusterClaim"),
    ).toMatchObject({
      objects: [],
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect(readLive).not.toHaveBeenCalled();
  });

  it("returns Unavailable when the read fails", async () => {
    readLive.mockRejectedValueOnce(new Error("boom"));

    expect(await runClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      ok: false,
      errors: [{ code: "Unavailable", message: "boom" }],
    });
  });

  it("the preview checks the claim as the delete does and dry-runs the change", async () => {
    reads = [live("45")];
    fake.preview.mockResolvedValue({ objects: [], errors: [] });

    expect(await previewClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      objects: [],
      errors: [],
    });
    expect(fake.preview).toHaveBeenCalledWith(remove);

    fake.preview.mockClear();
    reads = [live("46", { generation: 4 })];
    expect(await previewClaimDelete(remove, "CamundaClusterClaim")).toEqual({
      objects: [],
      errors: [expect.objectContaining({ code: "Stale", ref })],
    });
    expect(fake.preview).not.toHaveBeenCalled();
  });
});

describe("changeSchema and create", () => {
  const env = validChange.apply[0];

  it("keeps create when each key names an applied object", () => {
    const parsed = changeSchema.safeParse({
      ...validChange,
      create: ["Environment//prod"],
    });

    expect(parsed.success && parsed.data.create).toEqual(["Environment//prod"]);
  });

  it("refuses a create key that names no applied object", () => {
    expect(
      changeSchema.safeParse({ ...validChange, create: ["Environment//dev"] })
        .success,
    ).toBe(false);
  });

  it("keeps the UID of a deleted object", () => {
    const parsed = changeSchema.safeParse({
      intent: "x",
      apply: [],
      delete: [{ apiVersion: env.apiVersion, kind: env.kind, name: "prod" }],
      uids: { "Environment//prod": "uid-1" },
    });

    expect(parsed.success && parsed.data.uids).toEqual({
      "Environment//prod": "uid-1",
    });
  });

  it.each([
    ["an applied object", { apply: [env] }],
    ["no object of the change", { apply: [env], delete: [] }],
  ])("refuses a UID for %s", (_, over) => {
    expect(
      changeSchema.safeParse({
        intent: "x",
        ...over,
        uids: { "Environment//prod": "uid-1" },
      }).success,
    ).toBe(false);
  });

  it("refuses an empty UID", () => {
    expect(
      changeSchema.safeParse({
        intent: "x",
        apply: [],
        delete: [{ apiVersion: env.apiVersion, kind: env.kind, name: "prod" }],
        uids: { "Environment//prod": "" },
      }).success,
    ).toBe(false);
  });

  it("refuses a key in create and in preconditions", () => {
    expect(
      changeSchema.safeParse({
        intent: "x",
        apply: [env],
        create: ["Environment//prod"],
        preconditions: { "Environment//prod": "4" },
      }).success,
    ).toBe(false);
  });
});

describe("createChangeSchema", () => {
  const ns = {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: "platform" },
  };
  const nsKey = "Namespace//platform";

  it("accepts one claim of either kind that create lists", () => {
    expect(
      createChangeSchema.safeParse({
        intent: "x",
        apply: [planeClaim],
        create: [planeKey],
      }).success,
    ).toBe(true);
    expect(
      createChangeSchema.safeParse({
        intent: "x",
        apply: [{ ...planeClaim, kind: "CamundaClusterClaim" }],
        create: ["CamundaClusterClaim/platform/plane-a"],
      }).success,
    ).toBe(true);
  });

  it("accepts a new Namespace before its claim, both in create", () => {
    expect(
      createChangeSchema.safeParse({
        intent: "x",
        apply: [ns, planeClaim],
        create: [nsKey, planeKey],
      }).success,
    ).toBe(true);
  });

  it.each([
    ["no create", { intent: "x", apply: [planeClaim] }],
    [
      "create without the claim",
      { intent: "x", apply: [ns, planeClaim], create: [nsKey] },
    ],
    [
      "create without the Namespace",
      { intent: "x", apply: [ns, planeClaim], create: [planeKey] },
    ],
    [
      "the Namespace after the claim",
      { intent: "x", apply: [planeClaim, ns], create: [nsKey, planeKey] },
    ],
    [
      "a Namespace of another name",
      {
        intent: "x",
        apply: [{ ...ns, metadata: { name: "other" } }, planeClaim],
        create: ["Namespace//other", planeKey],
      },
    ],
    [
      "another kind",
      {
        intent: "x",
        apply: [
          { ...planeClaim, kind: "Environment", metadata: { name: "e" } },
        ],
        create: ["Environment//e"],
      },
    ],
    [
      "a claim without a namespace",
      {
        intent: "x",
        apply: [{ ...planeClaim, metadata: { name: "plane-a" } }],
        create: ["CamundaManagementClusterClaim//plane-a"],
      },
    ],
    [
      "two claims",
      {
        intent: "x",
        apply: [
          planeClaim,
          { ...planeClaim, metadata: { name: "b", namespace: "platform" } },
        ],
        create: [planeKey, "CamundaManagementClusterClaim/platform/b"],
      },
    ],
    [
      "a precondition",
      {
        intent: "x",
        apply: [planeClaim],
        create: [planeKey],
        preconditions: { "Namespace//platform": "3" },
      },
    ],
    [
      "a delete",
      {
        intent: "x",
        apply: [planeClaim],
        create: [planeKey],
        delete: [
          { apiVersion: "v1", kind: "Secret", namespace: "a", name: "b" },
        ],
      },
    ],
    [
      "force",
      { intent: "x", apply: [planeClaim], create: [planeKey], force: true },
    ],
    [
      "directOnly",
      {
        intent: "x",
        apply: [planeClaim],
        create: [planeKey],
        directOnly: true,
      },
    ],
  ])("refuses %s", (_, input) => {
    expect(createChangeSchema.safeParse(input).success).toBe(false);
  });
});

describe("backupChangeSchema", () => {
  const core = "core.camunda.io/v1";
  const schema = backupChangeSchema({ namespace: "team-a", name: "orders" });
  const backup = (kind = "LogicalBackupElasticsearch", over: object = {}) => ({
    apiVersion: core,
    kind,
    metadata: { name: "orders-manual-1", namespace: "team-a" },
    spec: { clusterRef: { name: "orders" } },
    ...over,
  });
  const created = (object: { kind: string; metadata: object }, over = {}) => {
    const { name, namespace } = object.metadata as {
      name: string;
      namespace?: string;
    };
    return {
      intent: "Back up orders",
      apply: [object],
      create: [`${object.kind}/${namespace ?? ""}/${name}`],
      directOnly: true,
      ...over,
    };
  };
  const ref = (kind = "LogicalBackupElasticsearch", over: object = {}) => ({
    apiVersion: core,
    kind,
    namespace: "team-a",
    name: "orders-manual-1",
    ...over,
  });
  const deleted = (r: object, over = {}) => ({
    intent: "Delete backup orders-manual-1",
    apply: [],
    delete: [r],
    directOnly: true,
    ...over,
  });
  const ok = (c: unknown) => schema.safeParse(c).success;

  it.each(["LogicalBackupElasticsearch", "LogicalBackupRDBMS"])(
    "accepts one new %s of the cluster, straight to the cluster",
    (kind) => {
      expect(ok(created(backup(kind)))).toBe(true);
    },
  );

  it("accepts labels and annotations on a new backup", () => {
    const metadata = {
      name: "orders-manual-1",
      namespace: "team-a",
      labels: { team: "a" },
      annotations: { note: "before the upgrade" },
    };
    expect(ok(created(backup(undefined, { metadata })))).toBe(true);
  });

  it.each(["LogicalBackupElasticsearch", "LogicalBackupRDBMS"])(
    "accepts the delete of one %s in the namespace of the cluster",
    (kind) => {
      expect(ok(deleted(ref(kind)))).toBe(true);
    },
  );

  it("accepts the delete of a backup with the UID that the page read", () => {
    expect(
      ok(
        deleted(ref(), {
          uids: { "LogicalBackupElasticsearch/team-a/orders-manual-1": "u-1" },
        }),
      ),
    ).toBe(true);
  });

  const other = { name: "orders-manual-2", namespace: "team-a" };
  it.each([
    ["no directOnly", created(backup(), { directOnly: undefined })],
    [
      "a UID on a new backup",
      created(backup(), {
        uids: { "LogicalBackupElasticsearch/team-a/orders-manual-1": "u-1" },
      }),
    ],
    ["directOnly false", created(backup(), { directOnly: false })],
    ["force", created(backup(), { force: true })],
    ["a create without create", created(backup(), { create: undefined })],
    [
      "a precondition on a delete",
      deleted(ref(), {
        preconditions: {
          "LogicalBackupElasticsearch/team-a/orders-manual-1": "7",
        },
      }),
    ],
    [
      "two new backups",
      {
        ...created(backup()),
        apply: [backup(), backup(undefined, { metadata: other })],
        create: [
          "LogicalBackupElasticsearch/team-a/orders-manual-1",
          "LogicalBackupElasticsearch/team-a/orders-manual-2",
        ],
      },
    ],
    ["a create and a delete", created(backup(), { delete: [ref()] })],
    [
      "two deletes",
      deleted(ref(), { delete: [ref(), ref(undefined, { name: "b" })] }),
    ],
    [
      "a backup with the spec of a restore",
      created(
        backup(undefined, {
          spec: {
            backupRef: { name: "orders-manual-1" },
            targetClusterRef: { name: "orders" },
          },
        }),
      ),
    ],
    ["a claim", created(backup("CamundaClusterClaim"))],
    ["another apiVersion", created(backup(undefined, { apiVersion: "v1" }))],
    [
      "a metadata field other than name, namespace, labels, and annotations",
      created(
        backup(undefined, {
          metadata: {
            name: "orders-manual-1",
            namespace: "team-a",
            finalizers: ["x"],
          },
        }),
      ),
    ],
    [
      "a field next to spec",
      created(backup(undefined, { status: { phase: "Completed" } })),
    ],
    [
      "a spec field next to clusterRef",
      created(
        backup("LogicalBackupRDBMS", {
          spec: { clusterRef: { name: "orders" }, extraEnv: [] },
        }),
      ),
    ],
    [
      "a backup of another cluster",
      created(backup(undefined, { spec: { clusterRef: { name: "billing" } } })),
    ],
    [
      "a backup in another namespace",
      created(
        backup(undefined, {
          metadata: { name: "orders-manual-1", namespace: "team-b" },
        }),
      ),
    ],
    [
      "a backup without a namespace",
      created(backup(undefined, { metadata: { name: "orders-manual-1" } })),
    ],
    ["the delete of a restore", deleted(ref("LogicalRestoreRDBMS"))],
    [
      "the delete of a backup in another namespace",
      deleted(ref(undefined, { namespace: "team-b" })),
    ],
    [
      "the delete of a backup with another apiVersion",
      deleted(ref(undefined, { apiVersion: "v1" })),
    ],
  ])("refuses %s", (_, change) => {
    expect(ok(change)).toBe(false);
  });
});

describe("backupChangeSchema, restores", () => {
  const core = "core.camunda.io/v1";
  const schema = backupChangeSchema({ namespace: "team-a", name: "orders" });
  const logical = (kind = "LogicalRestoreRDBMS", over: object = {}) => ({
    apiVersion: core,
    kind,
    metadata: { name: "orders-restore-1", namespace: "team-a" },
    spec: {
      backupRef: { name: "orders-manual-1" },
      targetClusterRef: { name: "orders" },
    },
    ...over,
  });
  const pitr = (over: object = {}) => ({
    apiVersion: core,
    kind: "PointInTimeRestore",
    metadata: { name: "orders-pitr-1", namespace: "team-a" },
    spec: { clusterRef: { name: "orders" }, timestamp: "2026-11-10T09:30:00Z" },
    ...over,
  });
  const created = (object: { kind: string; metadata: object }, over = {}) => {
    const { name, namespace } = object.metadata as {
      name: string;
      namespace?: string;
    };
    return {
      intent: "Restore orders",
      apply: [object],
      create: [`${object.kind}/${namespace ?? ""}/${name}`],
      directOnly: true,
      ...over,
    };
  };
  const ok = (c: unknown) => schema.safeParse(c).success;

  it.each(["LogicalRestoreElasticsearch", "LogicalRestoreRDBMS"])(
    "accepts one new %s of a backup into the cluster, straight to the cluster",
    (kind) => {
      expect(ok(created(logical(kind)))).toBe(true);
    },
  );

  it("accepts one new PointInTimeRestore of the cluster, straight to the cluster", () => {
    expect(ok(created(pitr()))).toBe(true);
  });

  it.each<[string, unknown]>([
    ["no directOnly", created(logical(), { directOnly: undefined })],
    ["force", created(logical(), { force: true })],
    ["a restore without create", created(logical(), { create: undefined })],
    [
      "a restore into another cluster",
      created(
        logical(undefined, {
          spec: {
            backupRef: { name: "orders-manual-1" },
            targetClusterRef: { name: "billing" },
          },
        }),
      ),
    ],
    [
      "a restore without a backup",
      created(
        logical(undefined, {
          spec: {
            backupRef: { name: "" },
            targetClusterRef: { name: "orders" },
          },
        }),
      ),
    ],
    [
      "a spec field next to backupRef and targetClusterRef",
      created(
        logical(undefined, {
          spec: {
            backupRef: { name: "orders-manual-1" },
            targetClusterRef: { name: "orders" },
            suspend: false,
          },
        }),
      ),
    ],
    [
      "a logical restore with the spec of a backup",
      created(logical(undefined, { spec: { clusterRef: { name: "orders" } } })),
    ],
    [
      "a restore in another namespace",
      created(
        logical(undefined, {
          metadata: { name: "orders-restore-1", namespace: "team-b" },
        }),
      ),
    ],
    [
      "a point-in-time restore of another cluster",
      created(
        pitr({
          spec: {
            clusterRef: { name: "billing" },
            timestamp: "2026-11-10T09:30:00Z",
          },
        }),
      ),
    ],
    ...[
      "2026-11-10T09:30:00",
      "2026-11-10T09:30:00.5Z",
      "2026-11-10T09:30:00+01:00",
      "2026-13-10T09:30:00Z",
      "now",
    ].map((timestamp): [string, unknown] => [
      `a point in time of ${timestamp}`,
      created(pitr({ spec: { clusterRef: { name: "orders" }, timestamp } })),
    ]),
    ...["constructor", "toString"].map((kind): [string, unknown] => [
      `a restore of the kind ${kind}`,
      created(logical(kind)),
    ]),
    [
      "a point-in-time restore with a field next to spec",
      created(pitr({ status: {} })),
    ],
    [
      "the delete of a point-in-time restore",
      {
        intent: "x",
        apply: [],
        delete: [
          {
            apiVersion: core,
            kind: "PointInTimeRestore",
            namespace: "team-a",
            name: "orders-pitr-1",
          },
        ],
        directOnly: true,
      },
    ],
  ])("refuses %s", (_, change) => {
    expect(ok(change)).toBe(false);
  });
});

describe("createClaim", () => {
  it("writes a create change and refuses any other change without a write", async () => {
    fake.apply.mockResolvedValue({ ok: true, mode: "direct" });
    const change = {
      intent: "Create plane-a",
      apply: [planeClaim],
      create: [planeKey],
    };

    expect(await createClaim(change)).toEqual({ ok: true, mode: "direct" });
    expect(await createClaim(validChange)).toMatchObject({
      ok: false,
      errors: [{ code: "Invalid" }],
    });
    expect(fake.apply).toHaveBeenCalledTimes(1);
    expect(fake.apply).toHaveBeenCalledWith(change);
  });
});

const environmentObject = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "staging" },
  spec: {},
};
const environmentRef = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  name: "staging",
};
const label = (
  labels: Record<string, unknown>,
  extra: { metadata?: object; spec?: object } = {},
) => ({
  intent: "Add team-a to staging",
  apply: [
    {
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name: "team-a", labels, ...extra.metadata },
      ...(extra.spec && { spec: extra.spec }),
    },
  ],
  preconditions: { "Namespace//team-a": "7" },
  directOnly: true,
});
const STAGING = { "fleet.camunda.io/environment": "staging" };

describe("adminChangeSchema", () => {
  const schema = adminChangeSchema(["Environment"]);
  const update = {
    intent: "Save staging",
    apply: [environmentObject],
    preconditions: { "Environment//staging": "3" },
  };
  const remove = {
    intent: "Delete staging",
    apply: [],
    delete: [environmentRef],
    preconditions: { "Environment//staging": "3" },
  };
  const create = {
    intent: "Create staging",
    apply: [environmentObject],
    create: ["Environment//staging"],
  };
  const preset = {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { name: "orders", namespace: "team-a" },
    spec: {},
  };
  const clusters = adminChangeSchema(["CamundaCluster"], ["team-a"]);

  it.each([
    ["an update with its precondition", schema, update],
    [
      "an update with a kept field",
      schema,
      { ...update, keep: { "Environment//staging": ["spec.sections"] } },
    ],
    ["a delete with its precondition", schema, remove],
    [
      "a delete with its precondition and its UID",
      schema,
      { ...remove, uids: { "Environment//staging": "u-1" } },
    ],
    ["a create", schema, create],
    [
      "a namespaced object in a namespace that the page writes",
      clusters,
      {
        intent: "x",
        apply: [preset],
        preconditions: { "CamundaCluster/team-a/orders": "4" },
      },
    ],
  ])("accepts %s", (_, s, change) =>
    expect(s.safeParse(change).success).toBe(true),
  );

  it.each([
    [
      "an apply of another kind",
      schema,
      {
        ...update,
        apply: [{ ...environmentObject, kind: "CamundaRelease" }],
        preconditions: { "CamundaRelease//staging": "3" },
      },
    ],
    [
      "a delete of another kind",
      schema,
      {
        ...remove,
        delete: [{ ...environmentRef, kind: "Namespace", apiVersion: "v1" }],
        preconditions: { "Namespace//staging": "3" },
      },
    ],
    [
      "another apiVersion of the kind",
      schema,
      {
        ...update,
        apply: [{ ...environmentObject, apiVersion: "fleet.camunda.io/v1" }],
      },
    ],
    [
      "a namespace on a cluster-scoped kind",
      schema,
      {
        ...update,
        apply: [
          {
            ...environmentObject,
            metadata: { name: "staging", namespace: "team-a" },
          },
        ],
        preconditions: { "Environment/team-a/staging": "3" },
      },
    ],
    [
      "a namespace that the page does not write",
      clusters,
      {
        intent: "x",
        apply: [
          { ...preset, metadata: { name: "orders", namespace: "kube-system" } },
        ],
        preconditions: { "CamundaCluster/kube-system/orders": "4" },
      },
    ],
    [
      "an update without its precondition",
      schema,
      { ...update, preconditions: undefined },
    ],
    [
      "a delete without its precondition",
      schema,
      { ...remove, preconditions: undefined },
    ],
    ["force on an update", schema, { ...update, force: true }],
    [
      "a kept field of a create",
      schema,
      { ...create, keep: { "Environment//staging": ["spec.sections"] } },
    ],
    ["force on a create", schema, { ...create, force: true }],
    ["force on a delete", schema, { ...remove, force: true }],
    [
      "a metadata field outside name, namespace, labels, and annotations",
      schema,
      {
        ...update,
        apply: [
          {
            ...environmentObject,
            metadata: { name: "staging", finalizers: ["x"] },
          },
        ],
      },
    ],
    [
      "a status",
      schema,
      { ...update, apply: [{ ...environmentObject, status: {} }] },
    ],
    ["directOnly", schema, { ...update, directOnly: true }],
  ])("refuses %s", (_, s, change) =>
    expect(s.safeParse(change).success).toBe(false),
  );

  it("passes keep on to the writer", () => {
    const keep = { "Environment//staging": ["spec.sections"] };
    expect(schema.parse({ ...update, keep }).keep).toEqual(keep);
  });
});

describe("namespaceLabelChangeSchema", () => {
  it("accepts one Namespace with the environment label, the precondition, and directOnly", () =>
    expect(namespaceLabelChangeSchema.safeParse(label(STAGING)).success).toBe(
      true,
    ));

  it("accepts the other labels and the annotations that the app owns", () =>
    expect(
      namespaceLabelChangeSchema.safeParse(
        label(
          { ...STAGING, team: "a" },
          { metadata: { annotations: { note: "x" }, resourceVersion: "7" } },
        ),
      ).success,
    ).toBe(true));

  it("refuses a Namespace without the environment label", () =>
    expect(
      namespaceLabelChangeSchema.safeParse(label({ team: "a" })).success,
    ).toBe(false));

  it("refuses an empty environment label", () =>
    expect(
      namespaceLabelChangeSchema.safeParse(
        label({ "fleet.camunda.io/environment": "" }),
      ).success,
    ).toBe(false));

  it("refuses a field outside the labels and the annotations", () => {
    const refused = (change: object) => {
      const result = namespaceLabelChangeSchema.safeParse(change);
      expect(result.success).toBe(false);
      expect(result.error?.issues[0].message).toBe(
        "The change must write only the labels and annotations of one namespace.",
      );
    };
    refused(label(STAGING, { spec: {} }));
    refused(label(STAGING, { metadata: { finalizers: ["x"] } }));
    refused(label(STAGING, { metadata: { labels: { team: 1 } } }));
  });

  it("refuses a change without the resourceVersion that the page read", () =>
    expect(
      namespaceLabelChangeSchema.safeParse({
        ...label(STAGING),
        preconditions: undefined,
      }).success,
    ).toBe(false));

  it("refuses an object of another kind", () => {
    const change = label(STAGING);
    change.apply[0].kind = "ConfigMap";
    expect(namespaceLabelChangeSchema.safeParse(change).success).toBe(false);
  });

  it("refuses a change without directOnly", () =>
    expect(
      namespaceLabelChangeSchema.safeParse({
        ...label(STAGING),
        directOnly: undefined,
      }).success,
    ).toBe(false));

  it("refuses a delete and force", () => {
    expect(
      namespaceLabelChangeSchema.safeParse({ ...label(STAGING), force: true })
        .success,
    ).toBe(false);
    expect(
      namespaceLabelChangeSchema.safeParse({
        ...label(STAGING),
        delete: [environmentRef],
      }).success,
    ).toBe(false);
  });
});

describe("claimBatchChangeSchema", () => {
  const claim = (
    name: string,
    namespace = "team-a",
    kind = "CamundaClusterClaim",
  ) => ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind,
    metadata: { name, namespace },
    spec: { releaseRef: "camunda-8-9-1" },
  });
  const batch = (over: Record<string, unknown> = {}) => ({
    intent: "Move 2 clusters to the release camunda-8-9-1",
    apply: [claim("orders"), claim("payments", "team-b")],
    preconditions: {
      "CamundaClusterClaim/team-a/orders": "4",
      "CamundaClusterClaim/team-b/payments": "9",
    },
    ...over,
  });

  it("accepts cluster claims in many namespaces, each with its precondition", () => {
    expect(claimBatchChangeSchema.safeParse(batch()).success).toBe(true);
  });

  it("accepts one cluster claim", () => {
    expect(
      claimBatchChangeSchema.safeParse(
        batch({
          apply: [claim("orders")],
          preconditions: { "CamundaClusterClaim/team-a/orders": "4" },
        }),
      ).success,
    ).toBe(true);
  });

  it("refuses a claim without a precondition", () => {
    expect(
      claimBatchChangeSchema.safeParse(
        batch({ preconditions: { "CamundaClusterClaim/team-a/orders": "4" } }),
      ).success,
    ).toBe(false);
  });

  it("refuses the same claim twice", () => {
    expect(
      claimBatchChangeSchema.safeParse(
        batch({ apply: [claim("orders"), claim("orders")] }),
      ).success,
    ).toBe(false);
  });

  it.each([
    [
      "a plane claim",
      {
        apply: [claim("plane", "team-a", "CamundaManagementClusterClaim")],
        preconditions: { "CamundaManagementClusterClaim/team-a/plane": "1" },
      },
    ],
    [
      "a CamundaCluster",
      {
        apply: [
          {
            ...claim("orders"),
            apiVersion: "core.camunda.io/v1",
            kind: "CamundaCluster",
          },
        ],
        preconditions: { "CamundaCluster/team-a/orders": "1" },
      },
    ],
    [
      "a claim of another apiVersion",
      {
        apply: [{ ...claim("orders"), apiVersion: "fleet.camunda.io/v1" }],
        preconditions: { "CamundaClusterClaim/team-a/orders": "1" },
      },
    ],
    [
      "a claim without a namespace",
      {
        apply: [{ ...claim("orders"), metadata: { name: "orders" } }],
        preconditions: { "CamundaClusterClaim//orders": "1" },
      },
    ],
    [
      "a delete",
      {
        delete: [
          {
            apiVersion: "fleet.camunda.io/v1alpha1",
            kind: "CamundaClusterClaim",
            namespace: "team-a",
            name: "old",
          },
        ],
      },
    ],
    ["create", { create: ["CamundaClusterClaim/team-a/orders"] }],
    ["force", { force: true }],
    ["directOnly", { directOnly: true }],
  ])("refuses %s", (_, over) => {
    expect(claimBatchChangeSchema.safeParse(batch(over)).success).toBe(false);
  });
});

describe("take-over lists", () => {
  const envKey = "Environment//prod";
  const update = {
    ...validChange,
    preconditions: { [envKey]: "7" },
  };
  const issues = (
    schema: { safeParse(input: unknown): unknown },
    input: object,
  ) => {
    const result = schema.safeParse(input) as {
      success: boolean;
      error?: { issues: { message: string; path: PropertyKey[] }[] };
    };
    return result.success
      ? []
      : result.error!.issues.map((i) => ({
          message: i.message,
          path: i.path.join("."),
        }));
  };

  it("changeSchema keeps takeOver and remove", () => {
    const input = {
      ...update,
      takeOver: { [envKey]: ["spec.displayName"] },
      remove: { [envKey]: ['spec.presets.camundaCluster[="b"]'] },
    };
    expect(changeSchema.parse(input)).toEqual(input);
  });

  it("refuses a list without a precondition, also on a created object", () => {
    expect(
      issues(changeSchema, {
        ...validChange,
        takeOver: { [envKey]: ["spec.displayName"] },
      }),
    ).toEqual([
      {
        message: "A take-over names an object that the change updates.",
        path: "takeOver",
      },
    ]);
    expect(
      issues(changeSchema, {
        ...validChange,
        create: [envKey],
        remove: { [envKey]: ["spec.displayName"] },
      }),
    ).toEqual([
      {
        message: "A take-over names an object that the change updates.",
        path: "remove",
      },
    ]);
    expect(
      issues(changeSchema, {
        ...update,
        takeOver: { "Environment//other": ["spec.displayName"] },
      }),
    ).toEqual([
      {
        message: "A take-over names an object that the change updates.",
        path: "takeOver",
      },
    ]);
  });

  it("refuses force beside a list", () => {
    expect(
      issues(changeSchema, {
        ...update,
        force: true,
        remove: { [envKey]: ["spec.displayName"] },
      }),
    ).toEqual([
      { message: "force and a take-over do not go together.", path: "force" },
    ]);
  });

  it("refuses a field listed twice", () => {
    expect(
      issues(changeSchema, {
        ...update,
        takeOver: { [envKey]: ["spec.displayName", "spec.displayName"] },
      }),
    ).toEqual([
      { message: "A take-over lists each field once.", path: "takeOver" },
    ]);
  });

  it("refuses an empty list and an empty path", () => {
    expect(
      changeSchema.safeParse({ ...update, takeOver: { [envKey]: [] } }).success,
    ).toBe(false);
    expect(
      changeSchema.safeParse({ ...update, remove: { [envKey]: [""] } }).success,
    ).toBe(false);
  });

  it("claimChangeSchema accepts a take-over of its claim", () => {
    expect(
      claimChangeSchema("CamundaManagementClusterClaim").safeParse({
        intent: "Move the plane",
        apply: [planeClaim],
        preconditions: { [planeKey]: "3" },
        takeOver: { [planeKey]: ["spec.versions.identity"] },
        remove: { [planeKey]: ["spec.deletionPolicy"] },
      }).success,
    ).toBe(true);
  });

  it("claimBatchChangeSchema accepts takeOver and refuses remove", () => {
    const claim = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-a" },
      spec: { releaseRef: "camunda-8-9-1" },
    };
    const key = "CamundaClusterClaim/team-a/orders";
    const move = {
      intent: "Move 1 cluster",
      apply: [claim],
      preconditions: { [key]: "4" },
    };
    expect(
      claimBatchChangeSchema.safeParse({
        ...move,
        takeOver: { [key]: ["spec.releaseRef"] },
      }).success,
    ).toBe(true);
    expect(
      claimBatchChangeSchema.safeParse({
        ...move,
        remove: { [key]: ["spec.allowVersionDowngrade"] },
      }).success,
    ).toBe(false);
  });

  it("createChangeSchema, claimDeleteSchema, backupChangeSchema, and namespaceLabelChangeSchema refuse a list", () => {
    const claimRef = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      namespace: "team-a",
      name: "orders",
    };
    const claimKey = "CamundaClusterClaim/team-a/orders";
    const remove = {
      intent: "Delete the cluster orders",
      apply: [],
      delete: [claimRef],
      uids: { [claimKey]: "uid-1" },
      generations: { [claimKey]: 3 },
    };
    const backupKey = "LogicalBackupElasticsearch/team-a/orders-manual-1";
    const backup = {
      intent: "Back up orders",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "LogicalBackupElasticsearch",
          metadata: { name: "orders-manual-1", namespace: "team-a" },
          spec: { clusterRef: { name: "orders" } },
        },
      ],
      create: [backupKey],
      directOnly: true,
    };
    const create = {
      intent: "Create the plane",
      apply: [planeClaim],
      create: [planeKey],
    };
    const backups = backupChangeSchema({ namespace: "team-a", name: "orders" });
    const cases: [
      { safeParse(input: unknown): { success: boolean } },
      object,
      string,
    ][] = [
      [createChangeSchema, create, planeKey],
      [claimDeleteSchema("CamundaClusterClaim"), remove, claimKey],
      [backups, backup, backupKey],
      [namespaceLabelChangeSchema, label(STAGING), "Namespace//team-a"],
    ];
    for (const [schema, change, key] of cases) {
      expect(schema.safeParse(change).success).toBe(true);
      for (const list of ["takeOver", "remove"])
        expect(
          schema.safeParse({ ...change, [list]: { [key]: ["spec.x"] } })
            .success,
        ).toBe(false);
    }
  });

  it("adminChangeSchema accepts both lists on an update", () => {
    expect(
      adminChangeSchema(["Environment"]).safeParse({
        ...update,
        takeOver: { [envKey]: ["spec.displayName"] },
        remove: { [envKey]: ["spec.color"] },
      }).success,
    ).toBe(true);
  });
});
