import { describe, expect, it } from "vitest";
import { formatBytes, formatCores } from "./units";

describe("formatCores", () => {
  it.each([
    [0.25, "250m"],
    [0.0005, "0.5m"],
    [1, "1"],
    [2.5, "2.5"],
  ])("gives %s cores as %s", (cores, text) => {
    expect(formatCores(cores)).toBe(text);
  });
});

describe("formatBytes", () => {
  it.each([
    [512, "512 B"],
    [1536, "1.5 KiB"],
    [2 * 2 ** 30, "2 GiB"],
    [300 * 2 ** 20, "300 MiB"],
  ])("gives %s bytes as %s", (bytes, text) => {
    expect(formatBytes(bytes)).toBe(text);
  });
});
