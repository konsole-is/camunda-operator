// Test-only: reads the merge cases of the operator in place. The format is in
// test/fixtures/merge/README.md.
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { child, indexItem, keyedItem } from "@/lib/merge/path";

export type MergeKind =
  "camundacluster" | "elasticsearchcluster" | "databaseserver";

type JsonObject = Record<string, unknown>;

/** MergeFixture is one case of test/fixtures/merge/<kind>/. */
export type MergeFixture = {
  file: string;
  name: string;
  rule: string;
  preset?: JsonObject;
  release?: JsonObject;
  spec: JsonObject;
  merged: JsonObject;
};

export const FIXTURES_DIR = fileURLToPath(
  new URL("../../../test/fixtures/merge", import.meta.url),
);

const FIELDS = new Set([
  "schemaVersion",
  "name",
  "rule",
  "preset",
  "release",
  "spec",
  "merged",
]);

/**
 * loadMergeFixtures reads every *.json case of kind under dir, in file name
 * order. It throws an error that names the file when a case has another
 * schemaVersion, a field that the format does not have, no name or rule, no
 * spec or merged object, or a release in a kind other than camundacluster.
 */
export function loadMergeFixtures(
  kind: MergeKind,
  dir: string = FIXTURES_DIR,
): MergeFixture[] {
  const kindDir = path.join(dir, kind);
  return readdirSync(kindDir)
    .filter((name) => name.endsWith(".json"))
    .sort()
    .map((name) => {
      const file = path.join(kindDir, name);
      try {
        return {
          file,
          ...parseFixture(JSON.parse(readFileSync(file, "utf8")), kind),
        };
      } catch (error) {
        throw new Error(
          `${file}: ${error instanceof Error ? error.message : String(error)}`,
        );
      }
    });
}

function parseFixture(
  data: unknown,
  kind: MergeKind,
): Omit<MergeFixture, "file"> {
  const doc = object(data, "the case");
  if (doc.schemaVersion !== 1) {
    throw new Error(
      `schemaVersion ${String(doc.schemaVersion)} is not 1, the only version that this test reads`,
    );
  }
  for (const key of Object.keys(doc)) {
    if (!FIELDS.has(key)) throw new Error(`unknown field ${key}`);
  }
  if (
    typeof doc.name !== "string" ||
    doc.name === "" ||
    typeof doc.rule !== "string" ||
    doc.rule === ""
  ) {
    throw new Error("a case needs a name and a rule");
  }
  if (doc.release !== undefined && kind !== "camundacluster") {
    throw new Error(`a ${kind} case must not have a release`);
  }
  return {
    name: doc.name,
    rule: doc.rule,
    preset: doc.preset === undefined ? undefined : object(doc.preset, "preset"),
    release:
      doc.release === undefined ? undefined : object(doc.release, "release"),
    spec: object(doc.spec, "spec"),
    merged: object(doc.merged, "merged"),
  };
}

/**
 * sourceProblems checks a source map against the spec that it describes.
 * Every scalar of spec needs a source on its own path or on the path of an
 * ancestor, and every source needs a value at its path. A list item that has
 * a string name has the path [name=<name>], any other item its index.
 */
export function sourceProblems(
  spec: JsonObject,
  source: Record<string, string>,
): string[] {
  const nodes = new Set<string>();
  const problems: string[] = [];

  const visit = (value: unknown, at: string, ancestors: string[]): void => {
    nodes.add(at);
    const chain = [...ancestors, at];
    if (Array.isArray(value)) {
      value.forEach((item, i) => {
        const name =
          isObject(item) && typeof item.name === "string"
            ? item.name
            : undefined;
        visit(
          item,
          name === undefined ? indexItem(at, i) : keyedItem(at, { name }),
          chain,
        );
      });
    } else if (isObject(value)) {
      for (const [key, item] of Object.entries(value))
        visit(item, child(at, key), chain);
    } else if (!chain.some((p) => p in source)) {
      problems.push(`${at} has no source`);
    }
  };
  visit(spec, "spec", []);

  for (const at of Object.keys(source)) {
    if (!nodes.has(at)) problems.push(`${at} has a source but no value`);
  }
  return problems;
}

function object(value: unknown, what: string): JsonObject {
  if (!isObject(value)) throw new Error(`${what} is not an object`);
  return value;
}

function isObject(value: unknown): value is JsonObject {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
