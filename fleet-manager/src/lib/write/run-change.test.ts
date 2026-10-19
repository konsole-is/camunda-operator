import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Writer } from "./types";

const fake = {
  mode: "direct" as const,
  preview: vi.fn<Writer["preview"]>(),
  apply: vi.fn<Writer["apply"]>(),
};
vi.mock("./writer", () => ({ getWriter: async () => fake }));

const { changeSchema, runChange } = await import("./run-change");

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
