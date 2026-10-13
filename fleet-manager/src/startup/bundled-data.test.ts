import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  describeBundledData,
  loadBundledData,
  parseCatalog,
  parseImagesTable,
} from "./bundled-data";

async function tempFile(content: string): Promise<string> {
  const dir = await mkdtemp(path.join(tmpdir(), "bundled-data-"));
  const file = path.join(dir, "data.json");
  await writeFile(file, content);
  return file;
}

describe("loadBundledData", () => {
  it("reads the images table and the catalog of the checkout", async () => {
    const data = await loadBundledData({});

    expect(data.imagesTable.schemaVersion).toBe(1);
    expect(Object.keys(data.imagesTable.minors).length).toBeGreaterThan(0);
    expect(data.imagesTablePath).toMatch(/pkg\/images\/table\.json$/);
    expect(data.catalog).toEqual({ schemaVersion: 1, charts: [] });
    expect(data.catalogPath).toMatch(/catalog\/official-plugins\.json$/);
  });

  it("reads the files that the environment names", async () => {
    const catalog = await tempFile(
      JSON.stringify({
        schemaVersion: 1,
        charts: [
          {
            name: "aws",
            description: "AWS options",
            chart: "oci://example/aws",
            installCommand: "helm install aws oci://example/aws",
          },
        ],
      }),
    );

    const data = await loadBundledData({ FLEET_CATALOG: catalog });

    expect(data.catalogPath).toBe(catalog);
    expect(data.catalog.charts.map((c) => c.name)).toEqual(["aws"]);
  });

  it("names the missing file", async () => {
    await expect(
      loadBundledData({ FLEET_IMAGES_TABLE: "/nonexistent/table.json" }),
    ).rejects.toThrow("reading the images table /nonexistent/table.json");
  });

  it("names the file that is not JSON", async () => {
    const file = await tempFile("{");

    await expect(loadBundledData({ FLEET_CATALOG: file })).rejects.toThrow(
      `the catalog ${file}:`,
    );
  });
});

describe("parseImagesTable", () => {
  it("refuses a schemaVersion that it does not know", () => {
    expect(() => parseImagesTable({ schemaVersion: 2, minors: {} })).toThrow(
      "schemaVersion 2 is not 1",
    );
  });

  it("refuses an image without a version", () => {
    expect(() =>
      parseImagesTable({
        schemaVersion: 1,
        minors: { "8.9": { camunda: { repository: "camunda/camunda" } } },
      }),
    ).toThrow("minors.8.9.camunda has no version");
  });
});

describe("parseCatalog", () => {
  it("refuses a catalog without schemaVersion", () => {
    expect(() => parseCatalog({ charts: [] })).toThrow(
      "schemaVersion undefined is not 1",
    );
  });

  it("refuses a chart without an install command", () => {
    expect(() =>
      parseCatalog({
        schemaVersion: 1,
        charts: [{ name: "aws", description: "d", chart: "oci://x" }],
      }),
    ).toThrow("charts[0].installCommand is not a non-empty string");
  });
});

describe("describeBundledData", () => {
  it("names both files on one line", async () => {
    const line = describeBundledData(await loadBundledData({}));

    expect(line).toMatch(
      /^Read the images table \S+table\.json \(minors \d+\.\d+.*\) and the catalog \S+official-plugins\.json \(0 charts\)$/,
    );
  });
});
