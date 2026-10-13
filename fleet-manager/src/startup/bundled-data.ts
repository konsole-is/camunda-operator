// The image ships the images table and the catalog of official plugin
// charts. The instrumentation hook reads both when the server starts, so a
// broken image stops at once and not on the first page that needs a file.
//
// This module does not import "server-only". The instrumentation hook
// imports it, and Next.js does not compile the hook with the react-server
// condition, so that package throws there. It lives outside src/lib for the
// same reason.
import { readFile } from "node:fs/promises";
import path from "node:path";

/** ImageEntry is one image of one Camunda minor. */
export type ImageEntry = {
  repository?: string;
  tagPrefix?: string;
  version: string;
};

/** ImagesTable is pkg/images/table.json, which make generate writes. */
export type ImagesTable = {
  schemaVersion: 1;
  minors: Record<string, Record<string, ImageEntry>>;
};

/** CatalogChart is one official plugin chart that an admin can install. */
export type CatalogChart = {
  name: string;
  description: string;
  chart: string;
  installCommand: string;
};

/** Catalog is fleet-manager/catalog/official-plugins.json. */
export type Catalog = { schemaVersion: 1; charts: CatalogChart[] };

export type BundledData = {
  imagesTable: ImagesTable;
  imagesTablePath: string;
  catalog: Catalog;
  catalogPath: string;
};

// The paths in a checkout, relative to fleet-manager/, where next dev,
// next build, and the tests run. The image sets both variables.
const DEFAULT_IMAGES_TABLE = "../pkg/images/table.json";
const DEFAULT_CATALOG = "catalog/official-plugins.json";

const CHART_FIELDS = [
  "name",
  "description",
  "chart",
  "installCommand",
] as const;

/**
 * loadBundledData reads the files that FLEET_IMAGES_TABLE and FLEET_CATALOG
 * name, or the files of the checkout. It rejects with an error that names the
 * file when one is missing, is not JSON, or has another schemaVersion.
 */
export async function loadBundledData(
  env: Record<string, string | undefined> = process.env,
): Promise<BundledData> {
  // The image copies both files itself. Without the ignore comment, the
  // tracer of next build copies the whole project into the standalone output.
  const imagesTablePath = path.resolve(
    /*turbopackIgnore: true*/ env.FLEET_IMAGES_TABLE || DEFAULT_IMAGES_TABLE,
  );
  const catalogPath = path.resolve(
    /*turbopackIgnore: true*/ env.FLEET_CATALOG || DEFAULT_CATALOG,
  );

  return {
    imagesTable: await load(
      imagesTablePath,
      "the images table",
      parseImagesTable,
    ),
    imagesTablePath,
    catalog: await load(catalogPath, "the catalog", parseCatalog),
    catalogPath,
  };
}

async function load<T>(
  file: string,
  what: string,
  parse: (data: unknown) => T,
): Promise<T> {
  let text: string;
  try {
    text = await readFile(file, "utf8");
  } catch (error) {
    throw new Error(`reading ${what} ${file}: ${messageOf(error)}`);
  }
  try {
    return parse(JSON.parse(text));
  } catch (error) {
    throw new Error(`${what} ${file}: ${messageOf(error)}`);
  }
}

/** parseImagesTable throws when data is not an images table of schemaVersion 1. */
export function parseImagesTable(data: unknown): ImagesTable {
  const doc = record(data, "the document");
  checkSchemaVersion(doc);
  const minors = record(doc.minors, "minors");
  for (const [minor, images] of Object.entries(minors)) {
    for (const [name, entry] of Object.entries(
      record(images, `minors.${minor}`),
    )) {
      if (
        typeof record(entry, `minors.${minor}.${name}`).version !== "string"
      ) {
        throw new Error(`minors.${minor}.${name} has no version`);
      }
    }
  }
  return doc as ImagesTable;
}

/** parseCatalog throws when data is not a catalog of schemaVersion 1. */
export function parseCatalog(data: unknown): Catalog {
  const doc = record(data, "the document");
  checkSchemaVersion(doc);
  if (!Array.isArray(doc.charts)) {
    throw new Error("charts is not a list");
  }
  doc.charts.forEach((item, i) => {
    const chart = record(item, `charts[${i}]`);
    for (const field of CHART_FIELDS) {
      if (typeof chart[field] !== "string" || chart[field] === "") {
        throw new Error(`charts[${i}].${field} is not a non-empty string`);
      }
    }
  });
  return doc as Catalog;
}

/** describeBundledData returns the one start line that names both files. */
export function describeBundledData(data: BundledData): string {
  const minors = Object.keys(data.imagesTable.minors).sort().join(", ");
  return (
    `Read the images table ${data.imagesTablePath} (minors ${minors}) ` +
    `and the catalog ${data.catalogPath} (${data.catalog.charts.length} charts)`
  );
}

function checkSchemaVersion(doc: Record<string, unknown>): void {
  if (doc.schemaVersion !== 1) {
    throw new Error(
      `schemaVersion ${String(doc.schemaVersion)} is not 1, the only version that this app reads`,
    );
  }
}

function record(value: unknown, where: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error(`${where} is not an object`);
  }
  return value as Record<string, unknown>;
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
