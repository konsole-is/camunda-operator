import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BundledData } from "@/startup/bundled-data";

const load = vi.hoisted(() => vi.fn<() => Promise<BundledData>>());
vi.mock("@/startup/bundled-data", () => ({ loadBundledData: load }));

const data: BundledData = {
  imagesTable: { schemaVersion: 1, minors: {} },
  imagesTablePath: "/table.json",
  catalog: { schemaVersion: 1, charts: [] },
  catalogPath: "/catalog.json",
};

// The cache is module state, so each test imports a fresh copy.
beforeEach(() => {
  vi.resetModules();
  load.mockReset();
});

describe("getBundledData", () => {
  it("reads the bundled data once and returns the same result to each caller", async () => {
    load.mockResolvedValue(data);
    const { getBundledData } = await import("./index");
    await expect(getBundledData()).resolves.toBe(data);
    await expect(getBundledData()).resolves.toBe(data);
    expect(load).toHaveBeenCalledTimes(1);
  });

  it("shares one read between callers that ask before it settles", async () => {
    load.mockReturnValue(
      new Promise((resolve) => setTimeout(() => resolve(data), 0)),
    );
    const { getBundledData } = await import("./index");
    await Promise.all([getBundledData(), getBundledData()]);
    expect(load).toHaveBeenCalledTimes(1);
  });

  it("does not keep a failed read, so the next call reads again", async () => {
    load.mockRejectedValueOnce(new Error("reading the images table x: ENOENT"));
    load.mockResolvedValueOnce(data);
    const { getBundledData } = await import("./index");
    await expect(getBundledData()).rejects.toThrow("ENOENT");
    await expect(getBundledData()).resolves.toBe(data);
    expect(load).toHaveBeenCalledTimes(2);
  });
});
