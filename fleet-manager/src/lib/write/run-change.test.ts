import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Writer } from "./types";

const fake = {
  mode: "direct" as const,
  preview: vi.fn<Writer["preview"]>(),
  apply: vi.fn<Writer["apply"]>(),
};
vi.mock("./writer", () => ({ getWriter: async () => fake }));

const { changeSchema, claimChangeSchema, runChange, runPreview } =
  await import("./run-change");

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
  ])("refuses %s", (_, input) => {
    expect(claimChangeSchema.safeParse(input).success).toBe(false);
  });
});
