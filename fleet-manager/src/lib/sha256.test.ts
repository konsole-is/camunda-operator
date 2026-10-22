import { createHash } from "node:crypto";
import { describe, expect, it } from "vitest";
import { sha256Hex } from "./sha256";

describe("sha256Hex", () => {
  it.each([
    ["", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"],
    ["abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"],
  ])("hashes %j to the published digest", (text, want) => {
    expect(sha256Hex(text)).toBe(want);
  });

  // 55, 56 and 64 bytes sit on the edges of the padding into a second block.
  it.each([
    "uid-orders-pg",
    "a".repeat(55),
    "a".repeat(56),
    "a".repeat(64),
    "a".repeat(1000),
    "8f7c1e2a-4b3d-4e5f-9a6b-7c8d9e0f1a2b",
    "ünïcødé",
  ])("matches node:crypto for %j", (text) => {
    expect(sha256Hex(text)).toBe(
      createHash("sha256").update(text).digest("hex"),
    );
  });
});
