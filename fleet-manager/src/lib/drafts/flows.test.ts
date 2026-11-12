import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import type { ClusterDraft } from "@/lib/clusters/draft";
import { storageErrors as clusterErrors } from "@/lib/clusters/storage";
import { EMPTY_ADMIN } from "@/lib/planes/components";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
} from "@/lib/planes/draft";
import {
  planeSectionViews,
  storageErrors as planeErrors,
} from "@/lib/planes/storage";
import type { BuiltinObjects, PluginSelection } from "@/lib/policy";

// Both create flows check their storage picks with the rules of this
// module. Each case feeds the same picks and the same live data to both, and
// both must refuse with the same messages.

const ready = {
  conditions: ["Accepted", "Ready"].map((type) => ({
    type,
    status: "True" as const,
    reason: "Test",
    message: "",
    lastTransitionTime: "2026-11-10T00:00:00Z",
  })),
};

function section(
  name: string,
  label: string,
  spec: Partial<PluginSection["spec"]>,
): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: { appliesTo: [], label, selection: "One", ...spec },
  };
}

function registration(
  name: string,
  label: string,
  spec: Partial<PluginRegistrationSpec>,
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: ["secondary-storage"],
      mode: "Optional",
      label,
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...spec,
    },
    status: ready,
  };
}

const BOTH = ["CamundaClusterClaim", "CamundaManagementClusterClaim"] as const;
const storage = section("secondary-storage", "Secondary storage", {
  appliesTo: [...BOTH],
});
const databases = section("management-database", "Databases", {
  appliesTo: ["CamundaManagementClusterClaim"],
  slot: "managementDatabases",
});

const REGISTRATIONS = [
  registration("existing-secondary-storage", "Existing secondary storage", {
    controller: { name: "camunda-operator", version: "0.0.0" },
    parameters: {
      type: "object",
      required: ["name"],
      properties: { name: { type: "string", minLength: 1, title: "Name" } },
    },
  }),
  registration("tiered", "Tiered storage", {
    parameters: {
      type: "object",
      properties: {
        tier: { type: "string", title: "Tier", enum: ["gold", "silver"] },
      },
    },
  }),
  registration("server", "New PostgreSQL server", {
    sections: ["management-database"],
  }),
];

function environment(
  allowed: string[],
  parameters: Record<string, unknown>,
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      sections: {
        "secondary-storage": { allowed },
        "management-database": { allowed: ["server"] },
      },
      parameters: parameters as Environment["spec"]["parameters"],
    },
  };
}

function config(name: string): SecondaryStorageConfig {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "SecondaryStorageConfig",
    metadata: { name, namespace: "team-a" },
    spec: { type: "rdbms" },
  } as SecondaryStorageConfig;
}

type Case = {
  name: string;
  pick: Omit<PluginSelection, "section">;
  allowed?: string[];
  parameters?: Record<string, unknown>;
  sections?: PluginSection[];
  objects?: BuiltinObjects;
  refusal: unknown[];
};

const NAMED = {
  "existing-secondary-storage": { name: { claimEditable: true } },
};
const ORDERS_LEFT = { SecondaryStorageConfig: [config("orders")] };
const ORDERS_GONE = { SecondaryStorageConfig: [config("other")] };

const CASES: Case[] = [
  {
    name: "a picked config that is still there passes",
    pick: {
      registration: "existing-secondary-storage",
      parameters: { name: "orders" },
    },
    parameters: NAMED,
    objects: ORDERS_LEFT,
    refusal: [],
  },
  {
    name: "a picked config that was deleted after the pick",
    pick: {
      registration: "existing-secondary-storage",
      parameters: { name: "orders" },
    },
    parameters: NAMED,
    objects: ORDERS_GONE,
    refusal: [
      "orders is not available for Name of Existing secondary storage. Choose another.",
    ],
  },
  {
    name: "a config behind an Environment alias that was deleted",
    pick: {
      registration: "existing-secondary-storage",
      parameters: { name: "fast" },
    },
    parameters: {
      "existing-secondary-storage": {
        name: {
          claimEditable: true,
          allowed: [{ name: "fast", label: "Fast", value: "orders" }],
        },
      },
    },
    objects: ORDERS_GONE,
    refusal: [
      "Pick another SecondaryStorageConfig for Name. orders is not in team-a.",
    ],
  },
  {
    name: "a config that a locked policy value names and that was deleted",
    pick: { registration: "existing-secondary-storage" },
    parameters: {
      "existing-secondary-storage": { name: { value: "orders" } },
    },
    objects: ORDERS_GONE,
    refusal: [
      "The SecondaryStorageConfig orders that the policy gives Name is not in team-a. Ask your admin to change the policy of the Environment.",
    ],
  },
  {
    name: "a choice that the Environment no longer allows",
    pick: { registration: "tiered", parameters: { tier: "silver" } },
    parameters: {
      tiered: {
        tier: {
          claimEditable: true,
          allowed: [{ name: "gold", label: "Gold", value: "gold" }],
        },
      },
    },
    refusal: [
      "silver is not available for Tier of Tiered storage. Choose another.",
    ],
  },
  {
    name: "a locked policy value that the schema of the registration refuses",
    pick: { registration: "tiered" },
    parameters: { tiered: { tier: { value: "bronze" } } },
    refusal: [
      expect.stringMatching(
        /^The policy gives Tiered storage a value that its schema refuses: .+\. Pick another option, or ask your admin to change the policy of the Environment\.$/,
      ),
    ],
  },
  {
    name: "an option that the Environment no longer allows",
    pick: { registration: "tiered" },
    allowed: ["existing-secondary-storage"],
    refusal: ["Choose another option for Secondary storage."],
  },
  {
    name: "a section that no longer applies to the claim",
    pick: { registration: "tiered" },
    sections: [section("secondary-storage", "Secondary storage", {})],
    refusal: ["Choose another option for secondary-storage."],
  },
];

function clusterRefusal(c: Case, live: Live): string[] {
  const draft: ClusterDraft = {
    namespace: { kind: "existing", name: "team-a" },
    claim: { metadata: { name: "orders" }, spec: { presetRef: "small" } },
    storage: { "secondary-storage": [c.pick as PluginSelection] },
  };
  return clusterErrors(
    draft,
    {
      environment: live.environment,
      sections: live.sections,
      registrations: REGISTRATIONS,
      objects: live.objects,
    },
    {},
    [],
  );
}

function planeRefusal(c: Case, live: Live): string[] {
  const draft: PlaneDraft = {
    ...EMPTY_PLANE_DRAFT,
    environment: "prod",
    name: "plane-a",
    namespace: { kind: "existing", name: "team-a" },
    mode: "keycloak",
    admin: { ...EMPTY_ADMIN, username: "admin" },
    storage: {
      "management-database": [
        { section: "management-database", registration: "server" },
      ],
      "secondary-storage": [c.pick as PluginSelection],
    },
    urls: {
      keycloak: "https://m.example.com/auth",
      identity: "https://m.example.com/identity",
    },
  };
  const ctx: PlaneContext = {
    environments: [live.environment],
    platformConfigs: [],
    takenNames: [],
    namespaces: [{ name: "team-a", environment: "prod" }],
    releases: [],
    images: { schemaVersion: 1, minors: {} },
    sections: [databases, ...live.sections],
    registrations: REGISTRATIONS,
    accessRequests: [],
    authConfigs: [],
    planeClaims: [],
  };
  return planeErrors(
    draft,
    ctx,
    {},
    planeSectionViews(draft, ctx, live.objects),
    live.objects,
  );
}

type Live = {
  environment: Environment;
  sections: PluginSection[];
  objects: BuiltinObjects;
};

describe("the cluster flow and the plane flow refuse the same stale picks", () => {
  it.each(CASES)("$name", (c) => {
    const pick = { section: "secondary-storage", ...c.pick };
    const live: Live = {
      environment: environment(
        c.allowed ?? ["existing-secondary-storage", "tiered"],
        c.parameters ?? {},
      ),
      sections: c.sections ?? [storage],
      objects: c.objects ?? {},
    };
    const withSection = { ...c, pick };

    expect(clusterRefusal(withSection, live)).toEqual(c.refusal);
    expect(planeRefusal(withSection, live)).toEqual(c.refusal);
  });
});
