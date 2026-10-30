import { describe, expect, it } from "vitest";
import { itemStep, pathFrom, walk } from "./walk";

describe("walk", () => {
  it("names a list item by its name and any other item by its index", () => {
    const nodes = walk(
      {
        extraEnv: [{ name: "TZ", value: "UTC" }],
        extraEnvFrom: [{ configMapRef: { name: "a" } }],
      },
      "spec",
    );
    expect([...nodes.keys()]).toEqual([
      "spec",
      "spec.extraEnv",
      "spec.extraEnv[name=TZ]",
      "spec.extraEnv[name=TZ].name",
      "spec.extraEnv[name=TZ].value",
      "spec.extraEnvFrom",
      "spec.extraEnvFrom[0]",
      "spec.extraEnvFrom[0].configMapRef",
      "spec.extraEnvFrom[0].configMapRef.name",
    ]);
    expect(nodes.get("spec.extraEnv[name=TZ].value")?.parent).toBe(
      "spec.extraEnv[name=TZ]",
    );
    expect(nodes.get("spec")?.parent).toBeUndefined();
  });

  it("marks scalars and empty blocks as leaves", () => {
    const nodes = walk({ a: 1, b: {}, c: [], d: { e: "x" } }, "spec");
    const leaf = (path: string) => nodes.get(path)?.leaf;
    expect(["spec.a", "spec.b", "spec.c", "spec.d.e"].map(leaf)).toEqual([
      true,
      true,
      true,
      true,
    ]);
    expect(leaf("spec.d")).toBe(false);
  });

  it("keeps a dot inside a map key in one segment", () => {
    const nodes = walk({ podLabels: { "a.b": "x" } }, "spec");
    const node = [...nodes.values()].find((n) => n.value === "x");
    expect(node?.steps).toEqual([{ field: "podLabels" }, { field: "a.b" }]);
  });

  it("skips a field without a value", () => {
    const nodes = walk({ a: undefined, b: null }, "spec");
    expect([...nodes.keys()]).toEqual(["spec", "spec.b"]);
  });
});

describe("pathFrom", () => {
  it("rebuilds a node path under another root", () => {
    const node = walk({ zeebe: { storageSize: "1Gi" } }, "spec").get(
      "spec.zeebe.storageSize",
    );
    expect(pathFrom("spec.cluster", node!.steps)).toBe(
      "spec.cluster.zeebe.storageSize",
    );
    expect(pathFrom("", node!.steps.slice(1))).toBe("storageSize");
  });

  it("rebuilds list items", () => {
    expect(
      pathFrom("x", [
        { field: "env" },
        { item: { name: "TZ" } },
        { field: "from" },
        { index: 2 },
      ]),
    ).toBe("x.env[name=TZ].from[2]");
  });
});

describe("itemStep", () => {
  it.each([
    [
      "a plugin selection by its section and registration",
      { section: "access", registration: "gateway-access", name: "x" },
      { item: { section: "access", registration: "gateway-access" } },
    ],
    [
      "an item with a string name by its name",
      { name: "LOG" },
      { item: { name: "LOG" } },
    ],
    ["any other item by its index", { value: 1 }, { index: 2 }],
    ["a scalar by its index", "a", { index: 2 }],
  ])("names %s", (_, item, step) => {
    expect(itemStep(item, 2)).toEqual(step);
  });
});
