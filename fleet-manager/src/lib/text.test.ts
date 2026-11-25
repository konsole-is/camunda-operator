import { describe, expect, it } from "vitest";
import { breakParts, listOf, messageOf } from "./text";

describe("listOf", () => {
  it.each([
    [[], ""],
    [["a"], "a"],
    [["a", "b"], "a and b"],
    [["a", "b", "c"], "a, b, and c"],
  ])("joins %j as %j", (items, text) => {
    expect(listOf(items)).toBe(text);
  });
});

describe("messageOf", () => {
  it("gives the message of an Error and any other value as text", () => {
    expect(messageOf(new Error("refused"))).toBe("refused");
    expect(messageOf("timeout")).toBe("timeout");
    expect(messageOf(503)).toBe("503");
  });
});

describe("breakParts", () => {
  it.each([
    ["SecondaryStorageConfig", ["Secondary", "Storage", "Config"]],
    [
      "https://login.example.com/realms/camunda",
      ["https:/", "/", "login.", "example.", "com/", "realms/", "camunda"],
    ],
    ["orders-storage, Ready", ["orders-storage, Ready"]],
    ["3/3", ["3/", "3"]],
    ["IAMRole", ["IAMRole"]],
    ["", [""]],
  ])("splits %j where a line can break", (text, parts) => {
    expect(breakParts(text)).toEqual(parts);
  });

  it("keeps every character", () => {
    const text = "s3://acme-backups/eu.west.1 SecondaryStorageConfig";
    expect(breakParts(text).join("")).toBe(text);
  });
});
