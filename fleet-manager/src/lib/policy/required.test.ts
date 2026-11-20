import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { BUILTIN_CONTROLLER } from "@/lib/labels";
import { declaresRelease, optionPicks, unsetRequired } from "./required";

function section(
  name: string,
  appliesTo: PluginSection["spec"]["appliesTo"],
  slot?: PluginSection["spec"]["slot"],
): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: {
      label: name,
      appliesTo,
      selection: "One",
      ...(slot && { slot }),
    },
  };
}

const sections = [
  section("backup-storage", ["CamundaClusterClaim"], "backupStorage"),
  section("plane-storage", ["CamundaManagementClusterClaim"]),
  section(
    "databases",
    ["CamundaManagementClusterClaim"],
    "managementDatabases",
  ),
  section("access", ["CamundaClusterClaim", "CamundaManagementClusterClaim"]),
];

const clusterPick = { section: "backup-storage", kind: "CamundaClusterClaim" };
const planePick = {
  section: "plane-storage",
  kind: "CamundaManagementClusterClaim",
};

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

// environmentWith allows store in the sections allowing, backup-storage by
// default, with parameters as its policy.
function environmentWith(
  parameters: NonNullable<Environment["spec"]["parameters"]>[string],
  allowing: string[] = ["backup-storage"],
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "production" },
    spec: {
      sections: Object.fromEntries(
        allowing.map((name) => [name, { allowed: ["store"] }]),
      ),
      parameters: { store: parameters },
    },
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

function unsetNames(
  r: PluginRegistration,
  environment: Environment | undefined,
): string[] {
  return unsetRequired(r, environment, sections).map((u) => u.name);
}

describe("unsetRequired", () => {
  it("gives each required parameter without a value in the order of the required list", () => {
    expect(unsetRequired(store, environmentWith({}), sections)).toEqual([
      { name: "bucket", picks: [clusterPick] },
      { name: "region", picks: [clusterPick] },
    ]);
  });

  it("gives nothing for a parameter that the Environment gives a value", () => {
    expect(
      unsetNames(store, environmentWith({ bucket: { value: "b" } })),
    ).toEqual(["region"]);
  });

  it("gives nothing for a parameter that the registration gives a value", () => {
    expect(
      unsetNames(withValues(store, { bucket: "b" }), environmentWith({})),
    ).toEqual(["region"]);
  });

  it("gives nothing for a parameter that a claim can change", () => {
    expect(
      unsetNames(store, environmentWith({ bucket: { claimEditable: true } })),
    ).toEqual(["region"]);
  });

  it("counts an empty string as a value", () => {
    expect(
      unsetNames(store, environmentWith({ bucket: { value: "" } })),
    ).toEqual(["region"]);
  });

  it("counts null as no value", () => {
    expect(
      unsetNames(
        withValues(store, { bucket: null }),
        environmentWith({ region: { value: null } }),
      ),
    ).toEqual(["bucket", "region"]);
  });

  it("ignores the claimEditable list of the registration under an Environment", () => {
    expect(
      unsetNames(withClaimEditable(store, ["bucket"]), environmentWith({})),
    ).toEqual(["bucket", "region"]);
  });

  it("reads the claimEditable list and the sections of the registration without an Environment", () => {
    expect(
      unsetRequired(withClaimEditable(store, ["bucket"]), undefined, sections),
    ).toEqual([{ name: "region", picks: [clusterPick] }]);
  });

  it("counts parameterValues that is not an object as no values", () => {
    expect(
      unsetNames(
        withValues(store, "b"),
        environmentWith({ region: { value: "eu" } }),
      ),
    ).toEqual(["bucket"]);
  });

  it("gives nothing for a schema without a required list", () => {
    expect(
      unsetNames(
        registration("plain", { parameters: { type: "object" } }),
        environmentWith({}),
      ),
    ).toEqual([]);
  });

  it("gives nothing for a registration without a parameter schema", () => {
    expect(unsetNames(registration("plain", {}), undefined)).toEqual([]);
  });

  it("gives nothing for an option that no section allows", () => {
    expect(unsetNames(store, environmentWith({}, []))).toEqual([]);
  });

  it("gives a section that does not exist a pick without a claim kind", () => {
    expect(
      unsetRequired(store, environmentWith({}, ["gone"]), sections),
    ).toEqual([
      { name: "bucket", picks: [{ section: "gone" }] },
      { name: "region", picks: [{ section: "gone" }] },
    ]);
  });

  it("skips releaseRef for a cluster claim when the schema declares it as a string", () => {
    expect(
      unsetNames(pluginWithRelease("string"), environmentWith({})),
    ).toEqual([]);
    expect(
      unsetNames(pluginWithRelease("object"), environmentWith({})),
    ).toEqual(["releaseRef"]);
    expect(unsetNames(builtinElasticsearch, undefined)).toEqual(["preset"]);
  });

  it("gives releaseRef for a management plane claim", () => {
    expect(
      unsetRequired(
        pluginWithRelease("string"),
        environmentWith({}, ["plane-storage"]),
        sections,
      ),
    ).toEqual([{ name: "releaseRef", picks: [planePick] }]);
  });

  it("gives only the picks without a value when the option serves both claim kinds", () => {
    expect(
      unsetRequired(
        pluginWithRelease("string"),
        environmentWith({}, ["backup-storage", "plane-storage"]),
        sections,
      ),
    ).toEqual([{ name: "releaseRef", picks: [planePick] }]);
    expect(
      unsetRequired(
        pluginWithRelease("string"),
        environmentWith({}, ["access"]),
        sections,
      ),
    ).toEqual([
      {
        name: "releaseRef",
        picks: [{ section: "access", kind: "CamundaManagementClusterClaim" }],
      },
    ]);
  });

  it("skips databases for a plane claim in a section with the slot managementDatabases", () => {
    const server = registration("store", {
      controller: { name: "db-plugin" },
      parameters: {
        type: "object",
        required: ["preset", "databases"],
        properties: {
          preset: { type: "string" },
          databases: { type: "array", items: { type: "string" } },
        },
      },
    });
    expect(unsetNames(server, environmentWith({}, ["databases"]))).toEqual([
      "preset",
    ]);
    expect(
      unsetRequired(
        server,
        environmentWith({ preset: { value: "small" } }, ["plane-storage"]),
        sections,
      ),
    ).toEqual([{ name: "databases", picks: [planePick] }]);
  });
});

describe("optionPicks", () => {
  it("gives each claim kind of each section that allows the option", () => {
    expect(
      optionPicks(
        store,
        environmentWith({}, ["access", "backup-storage"]),
        sections,
      ),
    ).toEqual([
      { section: "access", kind: "CamundaClusterClaim" },
      { section: "access", kind: "CamundaManagementClusterClaim" },
      clusterPick,
    ]);
  });
});

describe("declaresRelease", () => {
  it("is true for a top-level string property releaseRef only", () => {
    expect(declaresRelease(pluginWithRelease("string"))).toBe(true);
    expect(declaresRelease(pluginWithRelease("object"))).toBe(false);
    expect(declaresRelease(registration("plain", {}))).toBe(false);
  });
});
