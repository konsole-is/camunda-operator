import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { valueImports } from "../../../test/imports/scan";
import { parseQuantity } from "./quantity";

describe("parseQuantity", () => {
  it.each([
    ["250m", 0.25],
    ["2", 2],
    ["1500000n", 0.0015],
    ["10u", 0.00001],
    ["128Ki", 131072],
    ["1Gi", 1073741824],
    ["1G", 1e9],
    ["1e3", 1000],
    ["1E3", 1000],
    ["0.5", 0.5],
    ["+1k", 1000],
    ["-1", -1],
  ])("parses %s", (value, want) => {
    expect(parseQuantity(value)).toBeCloseTo(want, 12);
  });

  it.each(["1x", "", "Ki", "1.2.3", "1 Gi"])(
    "throws for %j and quotes it",
    (value) => {
      expect(() => parseQuantity(value)).toThrow(`"${value}"`);
    },
  );
});

it("quantity.ts imports no values", () =>
  expect(
    valueImports(
      readFileSync(new URL("./quantity.ts", import.meta.url), "utf8"),
    ),
  ).toEqual([]));
