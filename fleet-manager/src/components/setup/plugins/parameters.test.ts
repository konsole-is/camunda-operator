import { describe, expect, it } from "vitest";
import {
  fleetEnvironment,
  pluginRegistration,
  registrationSources,
} from "./fixtures";
import { parameterRows } from "./parameters";

const kms = pluginRegistration("aws-kms", {
  parameters: {
    type: "object",
    properties: {
      deletionWindowDays: {
        type: "integer",
        minimum: 7,
        maximum: 30,
        description: "Days before a key is deleted",
      },
      multiRegion: { type: "boolean" },
      keySpec: { type: "string", enum: ["SYMMETRIC_DEFAULT", "RSA_2048"] },
    },
  },
  parameterValues: { deletionWindowDays: 30, multiRegion: false },
  claimEditable: ["multiRegion"],
});

const production = fleetEnvironment("production", {
  displayName: "Production",
  parameters: {
    "aws-kms": {
      deletionWindowDays: { value: 14 },
      keySpec: { claimEditable: true },
    },
  },
});

const valueOf = (result: ReturnType<typeof parameterRows>) => {
  // A guard of the result union, not a test failure of its own.
  if (!("value" in result)) throw new Error(JSON.stringify(result));
  return result.value;
};

describe("parameterRows", () => {
  it("gives type, range, values, default, and claimEditable without an environment", () => {
    expect(parameterRows(kms, [], [])).toEqual({
      value: [
        expect.objectContaining({
          name: "deletionWindowDays",
          description: "Days before a key is deleted",
          type: "integer",
          minimum: 7,
          maximum: 30,
          fallback: 30,
          claimEditable: false,
          environments: [],
        }),
        expect.objectContaining({
          name: "keySpec",
          type: "string",
          values: ["SYMMETRIC_DEFAULT", "RSA_2048"],
          claimEditable: false,
        }),
        expect.objectContaining({
          name: "multiRegion",
          type: "boolean",
          fallback: false,
          claimEditable: true,
        }),
      ],
    });
  });

  it("gives the value and claimEditable of each environment", () => {
    const rows = valueOf(parameterRows(kms, [production], []));
    const byName = Object.fromEntries(
      rows.map((r) => [r.name, r.environments]),
    );
    expect(byName.deletionWindowDays).toEqual([
      { environment: "Production", value: 14, claimEditable: false },
    ]);
    expect(byName.keySpec).toEqual([
      { environment: "Production", value: undefined, claimEditable: true },
    ]);
    expect(byName.multiRegion).toEqual([
      { environment: "Production", value: false, claimEditable: false },
    ]);
  });

  it("names an environment without a display name by its name", () => {
    const rows = valueOf(parameterRows(kms, [fleetEnvironment("staging")], []));
    expect(rows[0].environments?.[0].environment).toBe("staging");
  });

  it("gives no environment values while Environment is not read", () => {
    const rows = valueOf(parameterRows(kms, [production], ["Environment"]));
    expect(rows.every((r) => r.environments === undefined)).toBe(true);
  });

  it("leaves out the parameters that the claim controller sets", () => {
    const es = registrationSources().registrations.find(
      (r) => r.metadata.name === "elasticsearch",
    )!;
    const rows = valueOf(parameterRows(es, [], []));
    expect(rows.map((r) => r.name)).toEqual(["overrides", "preset"]);
  });

  it("gives a row for a value without a schema", () => {
    const rows = valueOf(
      parameterRows(
        pluginRegistration("x", { parameterValues: { region: "eu-west-1" } }),
        [],
        [],
      ),
    );
    expect(rows).toEqual([
      {
        name: "region",
        fallback: "eu-west-1",
        claimEditable: false,
        environments: [],
      },
    ]);
  });

  it("returns the error of parameterValues that is not an object", () => {
    expect(
      parameterRows(
        pluginRegistration("bad", { parameterValues: [1] }),
        [],
        [],
      ),
    ).toHaveProperty("error");
  });

  it("returns an error for a schema that is not an object", () => {
    expect(
      parameterRows(
        pluginRegistration("bad", { parameters: "object" }),
        [],
        [],
      ),
    ).toEqual({ error: "The parameter schema is not an object." });
    expect(
      parameterRows(
        pluginRegistration("bad", { parameters: { properties: [] } }),
        [],
        [],
      ),
    ).toEqual({
      error: "The properties of the parameter schema are not an object.",
    });
  });

  it("gives an error row for a property schema that is not an object", () => {
    const rows = valueOf(
      parameterRows(
        pluginRegistration("bad", {
          parameters: { properties: { size: "big" } },
        }),
        [],
        [],
      ),
    );
    expect(rows).toEqual([
      {
        name: "size",
        error: "The schema of this parameter is not an object.",
        claimEditable: false,
        environments: [],
      },
    ]);
  });
});
