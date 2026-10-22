import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { resolve, sectionPolicy } from "./resolve";

// JSON.parse gives an own __proto__ key, as the API does.
const protoValues = JSON.parse('{"__proto__": "kept", "region": "eu"}');

const registration = {
  metadata: { name: "s3" },
  spec: {
    sections: ["backup-storage"],
    mode: "Default",
    parameterValues: protoValues,
  },
} as PluginRegistration;
const selection = { section: "backup-storage", registration: "s3" };

describe("resolve", () => {
  it("keeps a parameter named __proto__ in the snapshot and the request", () => {
    const got = resolve({
      registrations: [registration],
      selections: [selection],
    });

    expect(got).toHaveProperty("value");
    if (!("value" in got)) return;
    const policies = got.value.policy.parameters?.s3 ?? {};
    expect(Object.hasOwn(policies, "__proto__")).toBe(true);
    expect(
      Object.getOwnPropertyDescriptor(policies, "__proto__")?.value,
    ).toEqual({
      value: "kept",
    });
    const parameters = got.value.requests?.[0].parameters ?? {};
    expect(
      Object.getOwnPropertyDescriptor(parameters, "__proto__")?.value,
    ).toBe("kept");
  });

  it("keeps a parameter named __proto__ of a recorded snapshot in the request", () => {
    const recorded = JSON.parse(
      '{"parameters": {"s3": {"__proto__": {"value": "kept"}}}, "deletionPolicy": "Delete"}',
    );

    const got = resolve({ recorded, selections: [selection] });

    expect(got).toHaveProperty("value");
    if (!("value" in got)) return;
    const parameters = got.value.requests?.[0].parameters ?? {};
    expect(
      Object.getOwnPropertyDescriptor(parameters, "__proto__")?.value,
    ).toBe("kept");
  });
});

describe("sectionPolicy with a section named after a key of Object", () => {
  it("returns undefined when the Environment has no entry for it", () => {
    const environment = {
      metadata: { name: "prod" },
      spec: { sections: { dns: { allowed: ["route53"] } } },
    } as unknown as Environment;

    expect(sectionPolicy("constructor", environment, [])).toBeUndefined();
  });
});
