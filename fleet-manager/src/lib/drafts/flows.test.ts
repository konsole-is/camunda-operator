import { describe, expect, it } from "vitest";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
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
  registration("elasticsearch", "New Elasticsearch cluster", {
    controller: { name: "camunda-operator", version: "0.0.0" },
    parameters: {
      type: "object",
      properties: { preset: { type: "string", title: "Preset" } },
    },
  }),
  registration("server", "New PostgreSQL server", {
    sections: ["management-database"],
  }),
];

function environment(c: Case): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      sections: {
        "secondary-storage": {
          allowed: c.allowed ?? ["existing-secondary-storage", "tiered"],
          ...(c.required && { required: true }),
        },
        "management-database": { allowed: ["server"] },
      },
      presets: { elasticsearchCluster: c.presets ?? ["small", "large"] },
      parameters: (c.parameters ?? {}) as Environment["spec"]["parameters"],
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

// picks are the picks of the secondary storage section. allowed, required,
// presets, and parameters set the Environment.
type Case = {
  name: string;
  picks: Omit<PluginSelection, "section">[];
  allowed?: string[];
  required?: boolean;
  presets?: string[];
  parameters?: Record<string, unknown>;
  sections?: PluginSection[];
  objects?: BuiltinObjects;
  refusal: unknown[];
};

function preset(name: string): ElasticsearchClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "ElasticsearchClusterPreset",
    metadata: { name },
    spec: { cluster: { replicas: 1 } },
  } as ElasticsearchClusterPreset;
}

const NAMED = {
  "existing-secondary-storage": { name: { claimEditable: true } },
};
const EDITABLE_PRESET = { elasticsearch: { preset: { claimEditable: true } } };
const ORDERS_LEFT = { SecondaryStorageConfig: [config("orders")] };
const ORDERS_GONE = { SecondaryStorageConfig: [config("other")] };

const CASES: Case[] = [
  {
    name: "a picked config that is still there passes",
    picks: [
      {
        registration: "existing-secondary-storage",
        parameters: { name: "orders" },
      },
    ],
    parameters: NAMED,
    objects: ORDERS_LEFT,
    refusal: [],
  },
  {
    name: "a picked config that was deleted after the pick",
    picks: [
      {
        registration: "existing-secondary-storage",
        parameters: { name: "orders" },
      },
    ],
    parameters: NAMED,
    objects: ORDERS_GONE,
    refusal: [
      "orders is not available for Name of Existing secondary storage. Choose another.",
    ],
  },
  {
    name: "a config behind an Environment alias that was deleted",
    picks: [
      {
        registration: "existing-secondary-storage",
        parameters: { name: "fast" },
      },
    ],
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
    picks: [{ registration: "existing-secondary-storage" }],
    parameters: {
      "existing-secondary-storage": { name: { value: "orders" } },
    },
    objects: ORDERS_GONE,
    refusal: [
      "The SecondaryStorageConfig orders that the policy gives Name is not in team-a. Ask your admin to change the policy of the Environment.",
    ],
  },
  {
    name: "a preset behind an Environment alias that was deleted",
    picks: [{ registration: "elasticsearch", parameters: { preset: "fast" } }],
    allowed: ["elasticsearch"],
    parameters: {
      elasticsearch: {
        preset: {
          claimEditable: true,
          allowed: [{ name: "fast", label: "Fast", value: "small" }],
        },
      },
    },
    objects: { ElasticsearchClusterPreset: [preset("large")] },
    refusal: [
      "Pick another ElasticsearchClusterPreset for Preset. small does not exist.",
    ],
  },
  {
    name: "a choice that the Environment no longer allows",
    picks: [{ registration: "tiered", parameters: { tier: "silver" } }],
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
    picks: [{ registration: "tiered" }],
    parameters: { tiered: { tier: { value: "bronze" } } },
    refusal: [
      expect.stringMatching(
        /^The policy gives Tiered storage a value that its schema refuses: .+\. Pick another option, or ask your admin to change the policy of the Environment\.$/,
      ),
    ],
  },
  {
    name: "an option that the Environment no longer allows",
    picks: [{ registration: "tiered" }],
    allowed: ["existing-secondary-storage"],
    refusal: ["Choose another option for Secondary storage."],
  },
  {
    name: "a section that no longer applies to the claim",
    picks: [{ registration: "tiered" }],
    sections: [section("secondary-storage", "Secondary storage", {})],
    refusal: ["Choose another option for secondary-storage."],
  },
  {
    name: "a policy value outside the allowed names that the claim does not set passes",
    picks: [{ registration: "tiered" }],
    parameters: {
      tiered: {
        tier: {
          claimEditable: true,
          value: "silver",
          allowed: [{ name: "gold", label: "Gold", value: "gold" }],
        },
      },
    },
    refusal: [],
  },
  {
    name: "a value of the claim that names no choice",
    picks: [{ registration: "tiered", parameters: { tier: { level: 1 } } }],
    parameters: {
      tiered: {
        tier: {
          claimEditable: true,
          allowed: [{ name: "gold", label: "Gold", value: "gold" }],
        },
      },
    },
    refusal: ["Fix the Tier of Tiered storage."],
  },
  {
    name: "an empty allowed list of the Environment refuses no value",
    picks: [{ registration: "tiered", parameters: { tier: "gold" } }],
    parameters: { tiered: { tier: { claimEditable: true, allowed: [] } } },
    refusal: [],
  },
  {
    name: "a second pick in a section that now takes one",
    picks: [
      { registration: "tiered" },
      {
        registration: "existing-secondary-storage",
        parameters: { name: "orders" },
      },
    ],
    parameters: NAMED,
    objects: ORDERS_LEFT,
    refusal: ["Choose one option for Secondary storage."],
  },
  {
    name: "a preset that the Environment list dropped",
    picks: [{ registration: "elasticsearch", parameters: { preset: "large" } }],
    allowed: ["elasticsearch"],
    presets: ["small"],
    parameters: EDITABLE_PRESET,
    objects: { ElasticsearchClusterPreset: [preset("small"), preset("large")] },
    refusal: [
      "large is not available for Preset of New Elasticsearch cluster. Choose another.",
    ],
  },
  {
    name: "a picked preset that was deleted",
    picks: [{ registration: "elasticsearch", parameters: { preset: "large" } }],
    allowed: ["elasticsearch"],
    parameters: EDITABLE_PRESET,
    objects: { ElasticsearchClusterPreset: [preset("small")] },
    refusal: [
      "large is not available for Preset of New Elasticsearch cluster. Choose another.",
    ],
  },
  {
    name: "a required section that is gone",
    picks: [],
    required: true,
    sections: [],
    refusal: [
      'section "secondary-storage" is required, and the claim selects nothing in it',
    ],
  },
];

function clusterRefusal(c: Case, live: Live): string[] {
  const draft: ClusterDraft = {
    namespace: { kind: "existing", name: "team-a" },
    claim: { metadata: { name: "orders" }, spec: { presetRef: "small" } },
    storage: { "secondary-storage": picksOf(c) },
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
      "secondary-storage": picksOf(c),
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

function picksOf(c: Case): PluginSelection[] {
  return c.picks.map((p) => ({ section: "secondary-storage", ...p }));
}

type Live = {
  environment: Environment;
  sections: PluginSection[];
  objects: BuiltinObjects;
};

describe("the cluster flow and the plane flow refuse the same stale picks", () => {
  it.each(CASES)("$name", (c) => {
    const live: Live = {
      environment: environment(c),
      sections: c.sections ?? [storage],
      objects: c.objects ?? {},
    };

    expect(clusterRefusal(c, live)).toEqual(c.refusal);
    expect(planeRefusal(c, live)).toEqual(c.refusal);
  });
});
