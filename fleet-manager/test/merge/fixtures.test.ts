import { mkdir, mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { loadMergeFixtures, sourceProblems, type MergeKind } from "./fixtures";

async function fixtureDir(
  kind: MergeKind,
  files: Record<string, unknown>,
): Promise<string> {
  const dir = await mkdtemp(path.join(tmpdir(), "merge-fixtures-"));
  await mkdir(path.join(dir, kind));
  for (const [name, body] of Object.entries(files)) {
    await writeFile(path.join(dir, kind, name), JSON.stringify(body));
  }
  return dir;
}

const validCase = {
  schemaVersion: 1,
  name: "a case",
  rule: "`replicas`",
  spec: { replicas: 3 },
  merged: { replicas: 3 },
};

describe("loadMergeFixtures", () => {
  it("reads every case of the kind in file name order", async () => {
    const dir = await fixtureDir("databaseserver", {
      "b.json": { ...validCase, name: "second" },
      "a.json": { ...validCase, name: "first" },
    });

    const fixtures = loadMergeFixtures("databaseserver", dir);

    expect(fixtures.map((f) => f.name)).toEqual(["first", "second"]);
    expect(fixtures[0].file).toBe(path.join(dir, "databaseserver", "a.json"));
  });

  it("refuses a schemaVersion that it does not know and names the file", async () => {
    const dir = await fixtureDir("elasticsearchcluster", {
      "future.json": { ...validCase, schemaVersion: 2 },
    });

    expect(() => loadMergeFixtures("elasticsearchcluster", dir)).toThrow(
      /future\.json: schemaVersion 2 is not 1/,
    );
  });

  it("refuses a release in a case of a storage kind", async () => {
    const dir = await fixtureDir("databaseserver", {
      "release.json": { ...validCase, release: { version: "8.9.4" } },
    });

    expect(() => loadMergeFixtures("databaseserver", dir)).toThrow(
      /release\.json: a databaseserver case must not have a release/,
    );
  });

  it("refuses a field that the format does not have", async () => {
    const dir = await fixtureDir("camundacluster", {
      "extra.json": { ...validCase, expected: {} },
    });

    expect(() => loadMergeFixtures("camundacluster", dir)).toThrow(
      /extra\.json: unknown field expected/,
    );
  });

  it("refuses a case without merged", async () => {
    const { merged: _merged, ...withoutMerged } = validCase;
    const dir = await fixtureDir("camundacluster", {
      "unmerged.json": withoutMerged,
    });

    expect(() => loadMergeFixtures("camundacluster", dir)).toThrow(
      /unmerged\.json: merged is not an object/,
    );
  });
});

describe("sourceProblems", () => {
  it("accepts a source on a value or on one of its ancestors", () => {
    const spec = { replicas: 3, scheduling: { tolerations: [{ key: "a" }] } };

    expect(
      sourceProblems(spec, {
        "spec.replicas": "preset",
        "spec.scheduling": "overrides",
      }),
    ).toEqual([]);
  });

  it("reports a value without a source and a source without a value", () => {
    const spec = { replicas: 3, extraEnv: [{ name: "TZ", value: "UTC" }] };

    expect(sourceProblems(spec, { "spec.version": "release" })).toEqual([
      "spec.replicas has no source",
      "spec.extraEnv[name=TZ].name has no source",
      "spec.extraEnv[name=TZ].value has no source",
      "spec.version has a source but no value",
    ]);
  });
});
