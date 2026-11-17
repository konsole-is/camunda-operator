import { afterEach, describe, expect, it, vi } from "vitest";
import { loadImagesTable } from "./images";

const failing = vi.hoisted(() => ({ bundled: false }));
vi.mock("./index", () => ({
  getBundledData: vi.fn(async () => {
    if (failing.bundled) throw new Error("no table");
    return { imagesTable: { schemaVersion: 1, minors: { "8.9": {} } } };
  }),
}));

afterEach(() => {
  failing.bundled = false;
});

describe("loadImagesTable", () => {
  it("gives the table of the bundled data", async () => {
    expect(await loadImagesTable()).toEqual({
      schemaVersion: 1,
      minors: { "8.9": {} },
    });
  });

  it("gives an empty table when the bundled data cannot be read", async () => {
    failing.bundled = true;
    expect(await loadImagesTable()).toEqual({ schemaVersion: 1, minors: {} });
  });
});
