import { afterEach, describe, expect, it, vi } from "vitest";
import {
  dateTime,
  formatAxis,
  formatBytes,
  formatClock,
  formatCores,
  secondText,
} from "./units";

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

describe("formatClock", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("writes the UTC clock in any zone of the viewer", () => {
    // The zone has a half-hour offset, so a clock in the zone of the viewer cannot pass.
    vi.stubEnv("TZ", "Asia/Kolkata");
    expect(formatClock(Date.parse("2026-10-18T11:58:30Z"))).toBe("11:58");
  });
});

describe("dateTime", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("writes the UTC date and minute", () => {
    expect(dateTime("2026-10-18T11:58:31Z")).toBe("2026-10-18 11:58 UTC");
  });

  it("writes the same text in any zone of the viewer", () => {
    // The zone has a half-hour offset, so a time in the zone of the viewer cannot pass.
    vi.stubEnv("TZ", "Asia/Kolkata");
    expect(dateTime("2026-10-18T11:58:31Z")).toBe("2026-10-18 11:58 UTC");
  });

  it.each([undefined, "", "not a time"])("writes - for %j", (iso) => {
    expect(dateTime(iso)).toBe("-");
  });
});

describe("secondText", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("writes the UTC date and second in any zone of the viewer", () => {
    // The zone has a half-hour offset, so a time in the zone of the viewer cannot pass.
    vi.stubEnv("TZ", "Asia/Kolkata");
    expect(secondText("2026-11-10T09:30:15Z")).toBe("2026-11-10 09:30:15 UTC");
  });
});
