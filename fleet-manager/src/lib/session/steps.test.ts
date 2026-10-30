import { describe, expect, it } from "vitest";
import { valueAt, withValue, without } from "./steps";

const f = (field: string) => ({ field });

function object() {
  return {
    spec: {
      cluster: {
        zeebe: { replicas: 3 },
        extraEnv: [
          { name: "LOG", value: "info" },
          { name: "TZ", value: "UTC" },
        ],
        args: ["a", "b"],
      },
    },
  };
}

describe("valueAt", () => {
  it("reads a field, a keyed item, and an index", () => {
    const o = object();
    expect(
      valueAt(o, [f("spec"), f("cluster"), f("zeebe"), f("replicas")]),
    ).toBe(3);
    expect(
      valueAt(o, [
        f("spec"),
        f("cluster"),
        f("extraEnv"),
        { item: { name: "TZ" } },
        f("value"),
      ]),
    ).toBe("UTC");
    expect(valueAt(o, [f("spec"), f("cluster"), f("args"), { index: 1 }])).toBe(
      "b",
    );
  });

  it("is undefined for a missing value", () => {
    expect(
      valueAt(object(), [f("spec"), f("gateway"), f("replicas")]),
    ).toBeUndefined();
    expect(
      valueAt(object(), [
        f("spec"),
        f("cluster"),
        f("extraEnv"),
        { item: { name: "X" } },
      ]),
    ).toBeUndefined();
  });
});

describe("withValue", () => {
  it("sets a field and creates the missing objects", () => {
    const o = object();
    const after = withValue(
      o,
      [f("spec"), f("cluster"), f("gateway"), f("replicas")],
      2,
    );
    expect(after.spec.cluster).toMatchObject({ gateway: { replicas: 2 } });
    expect(o).toEqual(object());
  });

  it("replaces a keyed item", () => {
    const after = withValue(
      object(),
      [f("spec"), f("cluster"), f("extraEnv"), { item: { name: "TZ" } }],
      { name: "TZ", value: "Europe/Oslo" },
    );
    expect(after.spec.cluster.extraEnv).toEqual([
      { name: "LOG", value: "info" },
      { name: "TZ", value: "Europe/Oslo" },
    ]);
  });

  it("appends a missing keyed item with its keys", () => {
    const after = withValue(
      object(),
      [
        f("spec"),
        f("cluster"),
        f("extraEnv"),
        { item: { name: "NEW" } },
        f("value"),
      ],
      "x",
    );
    expect(after.spec.cluster.extraEnv).toEqual([
      { name: "LOG", value: "info" },
      { name: "TZ", value: "UTC" },
      { name: "NEW", value: "x" },
    ]);
  });

  it("sets an index", () => {
    const after = withValue(
      object(),
      [f("spec"), f("cluster"), f("args"), { index: 0 }],
      "z",
    );
    expect(after.spec.cluster.args).toEqual(["z", "b"]);
  });
});

describe("without", () => {
  it("removes a field and keeps its parent when it holds more", () => {
    const o = object();
    const after = without(o, [f("spec"), f("cluster"), f("args")]);
    expect(after.spec.cluster).toEqual({
      zeebe: { replicas: 3 },
      extraEnv: object().spec.cluster.extraEnv,
    });
    expect(o).toEqual(object());
  });

  it("removes a parent that becomes empty", () => {
    const after = without(object(), [
      f("spec"),
      f("cluster"),
      f("zeebe"),
      f("replicas"),
    ]);
    expect(after.spec.cluster).not.toHaveProperty("zeebe");
  });

  it("removes a keyed item and an index", () => {
    expect(
      without(object(), [
        f("spec"),
        f("cluster"),
        f("extraEnv"),
        { item: { name: "LOG" } },
      ]).spec.cluster.extraEnv,
    ).toEqual([{ name: "TZ", value: "UTC" }]);
    expect(
      without(object(), [f("spec"), f("cluster"), f("args"), { index: 0 }]).spec
        .cluster.args,
    ).toEqual(["b"]);
  });

  it("returns an equal object when the value is missing", () => {
    expect(without(object(), [f("spec"), f("gateway"), f("replicas")])).toEqual(
      object(),
    );
  });
});

describe("__proto__", () => {
  const steps = [f("parameters"), f("__proto__")];

  it("withValue sets __proto__ as an own field", () => {
    const after = withValue({ parameters: {} }, steps, { a: 1 });
    expect(Object.hasOwn(after.parameters, "__proto__")).toBe(true);
    expect(valueAt(after, steps)).toEqual({ a: 1 });
  });

  it("valueAt does not read an inherited field", () => {
    expect(valueAt({ parameters: {} }, steps)).toBeUndefined();
  });
});
