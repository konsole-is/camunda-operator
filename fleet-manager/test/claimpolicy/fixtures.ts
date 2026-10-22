// Test-only: reads the claim policy cases of the operator in place. The
// format is in test/fixtures/claimpolicy/README.md.
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { equalJSON, type PolicyResult } from "@/lib/policy";

/** APP_RULES are the folders whose rules a page of the app applies. */
export const APP_RULES = [
  "environment-of",
  "check-allowed",
  "resolve",
  "inputs",
  "slot-fields",
  "check-slot-conflicts",
] as const;
export type AppRule = (typeof APP_RULES)[number];

/**
 * EXCLUDED are controller-only rules: no page of the app applies them. The
 * create flow offers only a registration that serves its section, so it
 * never meets a check-served refusal.
 */
export const EXCLUDED = [
  "order",
  "target-service-account",
  "check-served",
] as const;

export const FIXTURES_DIR = fileURLToPath(
  new URL("../../../test/fixtures/claimpolicy", import.meta.url),
);

export type FixtureResult =
  | { value: unknown }
  | { refusal: { reason: string; message: string } }
  | { error: string };

/** PolicyFixture is one case of test/fixtures/claimpolicy/<folder>/. */
export type PolicyFixture = {
  file: string;
  name: string;
  input: Record<string, unknown>;
  result: FixtureResult;
};

const FIELDS = new Set(["schemaVersion", "name", "input", "result"]);
const RESULT_FIELDS = ["value", "refusal", "error"];

/**
 * loadPolicyFixtures reads every *.json case of folder under dir, in file
 * name order. It throws an error that names the file when a case has another
 * schemaVersion, a field that the format does not have, no name or input, or
 * a result without exactly one of value, refusal, and error.
 */
export function loadPolicyFixtures(
  folder: string,
  dir: string = FIXTURES_DIR,
): PolicyFixture[] {
  const folderDir = path.join(dir, folder);
  return readdirSync(folderDir)
    .filter((name) => name.endsWith(".json"))
    .sort()
    .map((name) => {
      const file = path.join(folderDir, name);
      try {
        return {
          file,
          ...parseFixture(JSON.parse(readFileSync(file, "utf8"))),
        };
      } catch (error) {
        throw new Error(
          `${file}: ${error instanceof Error ? error.message : String(error)}`,
        );
      }
    });
}

/** fixtureFolders returns the names of the folders of dir, sorted. */
export function fixtureFolders(dir: string = FIXTURES_DIR): string[] {
  return readdirSync(dir, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();
}

/** unclassified returns the folders that are in neither APP_RULES nor EXCLUDED. */
export function unclassified(folders: readonly string[]): string[] {
  const known: readonly string[] = [...APP_RULES, ...EXCLUDED];
  return folders.filter((f) => !known.includes(f));
}

/** missingFolders returns the folders of APP_RULES and EXCLUDED that folders does not hold. */
export function missingFolders(folders: readonly string[]): string[] {
  return [...APP_RULES, ...EXCLUDED].filter((f) => !folders.includes(f));
}

/**
 * compareResult returns undefined when got is the result that want holds,
 * and otherwise a text that shows both. A value compares as JSON, with any
 * key order. A refusal compares by its reason only, and an error matches any
 * error, because their texts are the texts of the Go code.
 */
export function compareResult(
  got: PolicyResult<unknown>,
  want: FixtureResult,
): string | undefined {
  const have = JSON.parse(JSON.stringify(got)) as FixtureResult;
  const differs = () =>
    `the result is ${JSON.stringify(want)}, and the function gives ${JSON.stringify(have)}`;

  if ("value" in want) {
    return "value" in have && equalJSON(have.value, want.value)
      ? undefined
      : differs();
  }
  if ("refusal" in want) {
    return "refusal" in have && have.refusal.reason === want.refusal.reason
      ? undefined
      : differs();
  }
  return "error" in have ? undefined : differs();
}

function parseFixture(data: unknown): Omit<PolicyFixture, "file"> {
  if (!isObject(data)) throw new Error("the case is not an object");
  if (data.schemaVersion !== 1) {
    throw new Error(
      `schemaVersion ${String(data.schemaVersion)} is not 1, the only version that this test reads`,
    );
  }
  for (const key of Object.keys(data)) {
    if (!FIELDS.has(key)) throw new Error(`unknown field ${key}`);
  }
  if (typeof data.name !== "string" || data.name === "") {
    throw new Error("a case needs a name");
  }
  if (!isObject(data.input)) throw new Error("a case needs an input");
  const result = data.result;
  if (
    !isObject(result) ||
    Object.keys(result).length !== 1 ||
    !RESULT_FIELDS.includes(Object.keys(result)[0])
  ) {
    throw new Error("result needs exactly one of value, refusal, and error");
  }
  return {
    name: data.name,
    input: data.input,
    result: result as FixtureResult,
  };
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
