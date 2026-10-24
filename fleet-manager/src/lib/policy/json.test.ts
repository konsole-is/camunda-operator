import { describe, expect, it } from "vitest";
import { equalJSON } from "./json";

describe("equalJSON", () => {
  it("finds two objects with other key order equal", () => {
    expect(equalJSON({ a: 1, b: [1, 2] }, { b: [1, 2], a: 1 })).toBe(true);
  });

  it("finds an own __proto__ key and another key different, in both orders", () => {
    const proto = JSON.parse('{"__proto__": {}}');

    expect(equalJSON(proto, { region: {} })).toBe(false);
    expect(equalJSON({ region: {} }, proto)).toBe(false);
  });

  it("finds two own __proto__ keys with the same value equal", () => {
    expect(
      equalJSON(JSON.parse('{"__proto__": 1}'), JSON.parse('{"__proto__": 1}')),
    ).toBe(true);
  });
});
