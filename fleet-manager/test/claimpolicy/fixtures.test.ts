import { mkdir, mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  checkAllowed,
  checkSlotConflicts,
  environmentOf,
  resolve,
  resolveInputs,
  slotFields,
  type PolicyResult,
} from "@/lib/policy";
import {
  APP_RULES,
  type AppRule,
  compareResult,
  EXCLUDED,
  fixtureFolders,
  loadPolicyFixtures,
  missingFolders,
  unclassified,
} from "./fixtures";

async function fixtureDir(
  folder: string,
  files: Record<string, unknown>,
): Promise<string> {
  const dir = await mkdtemp(path.join(tmpdir(), "claimpolicy-fixtures-"));
  await mkdir(path.join(dir, folder));
  for (const [name, body] of Object.entries(files)) {
    await writeFile(path.join(dir, folder, name), JSON.stringify(body));
  }
  return dir;
}

const validCase = {
  schemaVersion: 1,
  name: "a case",
  input: { a: 1 },
  result: { value: {} },
};

describe("loadPolicyFixtures", () => {
  it("reads every case of the folder in file name order", async () => {
    const dir = await fixtureDir("resolve", {
      "b.json": { ...validCase, name: "second" },
      "a.json": { ...validCase, name: "first" },
    });

    const fixtures = loadPolicyFixtures("resolve", dir);

    expect(fixtures.map((f) => f.name)).toEqual(["first", "second"]);
    expect(fixtures[0].file).toBe(path.join(dir, "resolve", "a.json"));
  });

  it("refuses a schemaVersion that it does not know and names the file", async () => {
    const dir = await fixtureDir("resolve", {
      "future.json": { ...validCase, schemaVersion: 2 },
    });

    expect(() => loadPolicyFixtures("resolve", dir)).toThrow(
      /future\.json: schemaVersion 2 is not 1/,
    );
  });

  it("refuses a field that the format does not have", async () => {
    const dir = await fixtureDir("resolve", {
      "extra.json": { ...validCase, expected: {} },
    });

    expect(() => loadPolicyFixtures("resolve", dir)).toThrow(
      /extra\.json: unknown field expected/,
    );
  });

  it("refuses a case without a name", async () => {
    const dir = await fixtureDir("resolve", {
      "unnamed.json": { ...validCase, name: "" },
    });

    expect(() => loadPolicyFixtures("resolve", dir)).toThrow(
      /unnamed\.json: a case needs a name/,
    );
  });

  it("refuses a case without an input", async () => {
    const withoutInput: Partial<typeof validCase> = { ...validCase };
    delete withoutInput.input;
    const dir = await fixtureDir("resolve", { "noinput.json": withoutInput });

    expect(() => loadPolicyFixtures("resolve", dir)).toThrow(
      /noinput\.json: a case needs an input/,
    );
  });

  it.each([
    ["no field", {}],
    ["two fields", { value: {}, error: "broken" }],
    ["another field", { output: {} }],
  ])("refuses a result with %s", async (_, result) => {
    const dir = await fixtureDir("resolve", {
      "result.json": { ...validCase, result },
    });

    expect(() => loadPolicyFixtures("resolve", dir)).toThrow(
      /result\.json: result needs exactly one of value, refusal, and error/,
    );
  });
});

describe("the folders of test/fixtures/claimpolicy", () => {
  it("classifies every folder of test/fixtures/claimpolicy", () => {
    expect(unclassified(fixtureFolders())).toEqual([]);
    expect(missingFolders(fixtureFolders())).toEqual([]);
  });

  it("names the excluded folders order, target-service-account, and check-served", () => {
    expect([...EXCLUDED]).toEqual([
      "order",
      "target-service-account",
      "check-served",
    ]);
  });

  it("fails on a folder that is in neither list", () => {
    expect(unclassified([...APP_RULES, ...EXCLUDED, "new-rule"])).toEqual([
      "new-rule",
    ]);
  });

  it("names a listed folder that does not exist", () => {
    expect(missingFolders(APP_RULES.filter((f) => f !== "inputs"))).toEqual([
      "inputs",
      ...EXCLUDED,
    ]);
  });
});

describe("compareResult", () => {
  it("finds the same value with another key order", () => {
    expect(
      compareResult(
        { value: { a: 1, b: [1, 2] } },
        { value: { b: [1, 2], a: 1 } },
      ),
    ).toBeUndefined();
  });

  it("finds two refusals with the same reason and other messages the same", () => {
    expect(
      compareResult(
        { refusal: { reason: "NotAllowed", message: "app text" } },
        { refusal: { reason: "NotAllowed", message: "Go text" } },
      ),
    ).toBeUndefined();
  });

  it("finds two errors with other texts the same", () => {
    expect(
      compareResult({ error: "app text" }, { error: "Go text" }),
    ).toBeUndefined();
  });

  it("drops an undefined field as JSON does", () => {
    expect(
      compareResult({ value: { a: undefined } }, { value: {} }),
    ).toBeUndefined();
  });

  it.each<[string, PolicyResult<unknown>, Parameters<typeof compareResult>[1]]>(
    [
      ["another value", { value: { a: 2 } }, { value: { a: 1 } }],
      [
        "another reason",
        { refusal: { reason: "NotAllowed", message: "" } },
        { refusal: { reason: "SlotConflict", message: "" } },
      ],
      [
        "a value where a refusal is expected",
        { value: {} },
        { refusal: { reason: "NotAllowed", message: "" } },
      ],
      [
        "an error where a value is expected",
        { error: "broken" },
        { value: {} },
      ],
    ],
  )("reports %s with both sides", (_, got, want) => {
    const text = compareResult(got, want);

    expect(text).toContain(JSON.stringify(want));
    expect(text).toContain(JSON.stringify(got));
  });
});

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- each runner reads the input of its own folder
type Runner = (input: any) => PolicyResult<unknown>;

const RUNNERS: Record<AppRule, Runner> = {
  "environment-of": (input) => {
    const got = environmentOf(input.namespace, input.environments ?? []);
    if (got.reason) {
      return { refusal: { reason: got.reason, message: got.message ?? "" } };
    }
    return {
      value: got.environment
        ? { environment: got.environment.metadata.name }
        : {},
    };
  },
  resolve: (input) => resolve(input),
  "check-allowed": (input) => checkAllowed(input),
  inputs: (input) => ({
    value: resolveInputs(
      input.registration,
      input.selections ?? [],
      input.requests ?? [],
    ),
  }),
  "slot-fields": (input) =>
    slotFields(
      input.kind,
      input.sections ?? [],
      input.selections ?? [],
      input.requests ?? [],
    ),
  "check-slot-conflicts": (input) =>
    checkSlotConflicts(
      input.sections ?? [],
      input.selections ?? [],
      input.overrides,
    ),
};

it("runs every app-rule folder", () => {
  expect(Object.keys(RUNNERS).sort()).toEqual([...APP_RULES].sort());
});

describe.each(Object.entries(RUNNERS))("the %s rule", (folder, run) => {
  const fixtures = loadPolicyFixtures(folder);

  it("has cases", () => {
    expect(fixtures.length).toBeGreaterThan(0);
  });

  it.each(fixtures.map((f) => [path.basename(f.file), f] as const))(
    "passes %s",
    (_, f) => {
      expect(compareResult(run(f.input), f.result), f.name).toBeUndefined();
    },
  );
});
