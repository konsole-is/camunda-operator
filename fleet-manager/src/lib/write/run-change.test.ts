import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Writer } from "./types";

const fake = {
  mode: "direct" as const,
  preview: vi.fn<Writer["preview"]>(),
  apply: vi.fn<Writer["apply"]>(),
};
vi.mock("./writer", () => ({ getWriter: async () => fake }));

const {
  backupChangeSchema,
  changeSchema,
  claimChangeSchema,
  createChangeSchema,
  createClaim,
  runChange,
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
  fake.apply.mockReset();
  fake.preview.mockReset();
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
    const result = await runPreview(validChange, claimChangeSchema);

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

  it("accepts one claim of either kind to apply, with a precondition", () => {
    expect(
      claimChangeSchema.safeParse({
        intent: "x",
        apply: [claim],
        preconditions: { "CamundaClusterClaim/team-a/orders": "41" },
      }).success,
    ).toBe(true);
    expect(
      claimChangeSchema.safeParse({
        intent: "x",
        apply: [{ ...claim, kind: "CamundaManagementClusterClaim" }],
      }).success,
    ).toBe(true);
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
    expect(claimChangeSchema.safeParse(input).success).toBe(false);
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

  const other = { name: "orders-manual-2", namespace: "team-a" };
  it.each([
    ["no directOnly", created(backup(), { directOnly: undefined })],
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
      "a restore",
      created(
        backup("LogicalRestoreElasticsearch", {
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
