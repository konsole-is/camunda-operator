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
    expect(data.catalog.schemaVersion).toBe(1);
    expect(data.catalog.charts.length).toBeGreaterThan(0);
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
            sections: ["cloud-identity"],
            chart: "oci://example/aws",
            version: "0.1.0",
            repository: "https://example/aws",
            installCommand:
              "helm install aws oci://example/aws --version 0.1.0",
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

  it("refuses an empty version", () => {
    expect(() =>
      parseImagesTable({
        schemaVersion: 1,
        minors: { "8.9": { camunda: { version: "" } } },
      }),
    ).toThrow("minors.8.9.camunda has no version");
  });

  it("refuses a repository or a tag prefix that is not a string", () => {
    expect(() =>
      parseImagesTable({
        schemaVersion: 1,
        minors: { "8.9": { camunda: { repository: 1, version: "8.9.0" } } },
      }),
    ).toThrow("minors.8.9.camunda.repository is not a string");
    expect(() =>
      parseImagesTable({
        schemaVersion: 1,
        minors: { "8.9": { keycloak: { tagPrefix: [], version: "26.4" } } },
      }),
    ).toThrow("minors.8.9.keycloak.tagPrefix is not a string");
  });
});

const validChart = {
  name: "aws",
  description: "d",
  sections: ["cloud-identity"],
  chart: "oci://x",
  version: "0.1.0",
  repository: "https://x",
  installCommand: "helm install aws oci://x --version 0.1.0",
};

describe("parseCatalog", () => {
  it("refuses a catalog without schemaVersion", () => {
    expect(() => parseCatalog({ charts: [] })).toThrow(
      "schemaVersion undefined is not 1",
    );
  });

  it("refuses a chart without an install command", () => {
    const chart: Record<string, unknown> = { ...validChart };
    delete chart.installCommand;

    expect(() => parseCatalog({ schemaVersion: 1, charts: [chart] })).toThrow(
      "charts[0].installCommand is not a non-empty string",
    );
  });

  it.each(["version", "repository"])("refuses a chart without %s", (field) => {
    const chart: Record<string, unknown> = { ...validChart };
    delete chart[field];

    expect(() => parseCatalog({ schemaVersion: 1, charts: [chart] })).toThrow(
      `charts[0].${field} is not a non-empty string`,
    );
  });

  it.each([
    ["without sections", undefined],
    ["with an empty list of sections", []],
    ["with a section that is not a string", [1]],
    ["with an empty section name", [""]],
  ])("refuses a chart %s", (_, sections) => {
    expect(() =>
      parseCatalog({ schemaVersion: 1, charts: [{ ...validChart, sections }] }),
    ).toThrow("charts[0].sections is not a non-empty list of section names");
  });

  it("reads a chart of the full shape", () => {
    expect(
      parseCatalog({ schemaVersion: 1, charts: [validChart] }).charts,
    ).toEqual([validChart]);
  });
});

describe("describeBundledData", () => {
  it("names both files on one line", async () => {
    const data = await loadBundledData({});

    expect(describeBundledData(data)).toMatch(
      new RegExp(
        String.raw`^Read the images table \S+table\.json \(minors \d+\.\d+.*\) and the catalog \S+official-plugins\.json \(${data.catalog.charts.length} charts\)$`,
      ),
    );
  });
});
