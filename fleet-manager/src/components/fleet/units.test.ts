import { describe, expect, it } from "vitest";
import { formatAxis, formatBytes, formatCores } from "./units";

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

describe("formatAxis", () => {
  it.each([
    [12 * 2 ** 30, "bytes", "12 GiB"],
    [0.5, "cores", "500m"],
    [1500, "instances/s", "1.5K"],
    [1234, "instances/s", "1.23K"],
    [0.004, "instances/s", "0.004"],
    [0.25, undefined, "0.25"],
  ])("gives %s with the unit %s as %s", (value, unit, text) => {
    expect(formatAxis(value, unit)).toBe(text);
  });
});
