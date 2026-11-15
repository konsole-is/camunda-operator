import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import { BUILTIN_CONTROLLER } from "@/lib/labels";
import { declaresRelease, unsetRequired } from "./required";

function registration(
  name: string,
  spec: Partial<PluginRegistrationSpec>,
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: ["backup-storage"],
      mode: "Optional",
      label: `${name} label`,
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...spec,
    },
  };
}

function environmentWith(
  parameters: NonNullable<Environment["spec"]["parameters"]>[string],
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "production" },
    spec: { parameters: { store: parameters } },
  };
}

const store = registration("store", {
  parameters: {
    type: "object",
    required: ["bucket", "region"],
    properties: {
      bucket: { type: "string", minLength: 1 },
      region: { type: "string" },
      tier: { type: "string" },
    },
  },
});

function withValues(
  r: PluginRegistration,
  parameterValues: unknown,
): PluginRegistration {
  return {
    ...r,
    spec: {
      ...r.spec,
      parameterValues:
        parameterValues as PluginRegistrationSpec["parameterValues"],
    },
  };
}

function withClaimEditable(
  r: PluginRegistration,
  claimEditable: string[],
): PluginRegistration {
  return { ...r, spec: { ...r.spec, claimEditable } };
}

// The schema of the built-in elasticsearch registration in
// internal/pluginstartup/schemas.go.
const builtinElasticsearch = registration("elasticsearch", {
  controller: { name: BUILTIN_CONTROLLER },
  parameters: {
    type: "object",
    required: ["preset", "releaseRef"],
    properties: {
      preset: { type: "string", minLength: 1 },
      releaseRef: { type: "string", minLength: 1 },
    },
  },
});

function pluginWithRelease(type: string): PluginRegistration {
  return registration("store", {
    controller: { name: "s3-plugin" },
    parameters: {
      type: "object",
      required: ["releaseRef"],
      properties: { releaseRef: { type } },
    },
  });
}

describe("unsetRequired", () => {
  it("gives each required parameter without a value in the order of the required list", () => {
    expect(unsetRequired(store, environmentWith({}))).toEqual([
      "bucket",
      "region",
    ]);
  });

  it("gives nothing for a parameter that the Environment gives a value", () => {
    expect(
      unsetRequired(store, environmentWith({ bucket: { value: "b" } })),
    ).toEqual(["region"]);
  });

  it("gives nothing for a parameter that the registration gives a value", () => {
    expect(
      unsetRequired(withValues(store, { bucket: "b" }), environmentWith({})),
    ).toEqual(["region"]);
  });

  it("gives nothing for a parameter that a claim can change", () => {
    expect(
      unsetRequired(
        store,
        environmentWith({ bucket: { claimEditable: true } }),
      ),
    ).toEqual(["region"]);
  });

  it("counts an empty string as a value", () => {
    expect(
      unsetRequired(store, environmentWith({ bucket: { value: "" } })),
    ).toEqual(["region"]);
  });

  it("counts null as no value", () => {
    expect(
      unsetRequired(
        withValues(store, { bucket: null }),
        environmentWith({ region: { value: null } }),
      ),
    ).toEqual(["bucket", "region"]);
  });

  it("ignores the claimEditable list of the registration under an Environment", () => {
    expect(
      unsetRequired(withClaimEditable(store, ["bucket"]), environmentWith({})),
    ).toEqual(["bucket", "region"]);
  });

  it("reads the claimEditable list of the registration without an Environment", () => {
    expect(
      unsetRequired(withClaimEditable(store, ["bucket"]), undefined),
    ).toEqual(["region"]);
  });

  it("counts parameterValues that is not an object as no values", () => {
    expect(
      unsetRequired(
        withValues(store, "b"),
        environmentWith({ region: { value: "eu" } }),
      ),
    ).toEqual(["bucket"]);
  });

  it("gives nothing for a schema without a required list", () => {
    expect(
      unsetRequired(
        registration("plain", { parameters: { type: "object" } }),
        environmentWith({}),
      ),
    ).toEqual([]);
  });

  it("gives nothing for a registration without a parameter schema", () => {
    expect(unsetRequired(registration("plain", {}), undefined)).toEqual([]);
  });

  it("skips the hidden parameters of a built-in registration", () => {
    expect(unsetRequired(builtinElasticsearch, environmentWith({}))).toEqual([
      "preset",
    ]);
    const managementServer = registration("management-database-server", {
      controller: { name: BUILTIN_CONTROLLER },
      parameters: {
        type: "object",
        required: ["preset", "databases"],
        properties: {
          preset: { type: "string" },
          databases: { type: "array", items: { type: "string" } },
        },
      },
    });
    expect(unsetRequired(managementServer, undefined)).toEqual(["preset"]);
  });

  it("skips releaseRef when the schema declares it as a string", () => {
    expect(
      unsetRequired(pluginWithRelease("string"), environmentWith({})),
    ).toEqual([]);
    expect(
      unsetRequired(pluginWithRelease("object"), environmentWith({})),
    ).toEqual(["releaseRef"]);
  });
});

describe("declaresRelease", () => {
  it("is true for a top-level string property releaseRef only", () => {
    expect(declaresRelease(pluginWithRelease("string"))).toBe(true);
    expect(declaresRelease(pluginWithRelease("object"))).toBe(false);
    expect(declaresRelease(registration("plain", {}))).toBe(false);
  });
});
