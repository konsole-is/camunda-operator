import { describe, expect, it, vi } from "vitest";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import * as schema from "@/lib/schema";
import { resolvedErrors } from "./checks";

vi.mock("@/lib/schema", async (actual) => {
  const real = await actual<typeof schema>();
  return { ...real, validate: vi.fn(real.validate) };
});

const store = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRegistration",
  metadata: { name: "store" },
  spec: {
    sections: ["storage"],
    mode: "Optional",
    label: "Store",
    deletionPolicies: ["Delete"],
    serviceAccount: { name: "store", namespace: "plugins" },
    parameters: {
      type: "object",
      required: ["owner"],
      properties: { owner: { type: "string" } },
    },
  },
} as PluginRegistration;

describe("resolvedErrors", () => {
  it("checks a parameter that is not known yet against the same schema object on each call", () => {
    const selections = [{ section: "storage", registration: "store" }];
    const draft = {
      kind: "CamundaClusterClaim" as const,
      picks: { storage: selections },
      namespace: "team-a",
      environment: undefined,
      sections: [],
      registrations: [store],
    };
    const later = () => ({ owner: undefined });

    resolvedErrors(draft, [], selections, later);
    resolvedErrors(draft, [], selections, later);

    const schemas = vi.mocked(schema.validate).mock.calls.map(([s]) => s);
    expect(schemas).toHaveLength(2);
    expect(schemas[1]).toBe(schemas[0]);
  });
});
