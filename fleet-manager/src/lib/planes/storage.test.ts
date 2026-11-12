import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { goneSections } from "@/lib/drafts";
import { EMPTY_ADMIN } from "./components";
import { EMPTY_PLANE_DRAFT, type PlaneContext, type PlaneDraft } from "./draft";
import {
  DATABASE_SUFFIX,
  contractNameErrors,
  contractNameOf,
  databaseConfigErrors,
  databaseSection,
  managementHost,
  planeDeletion,
  planeSectionViews,
  planeStorageDraft,
  storageErrors,
} from "./storage";
import type { Running } from "./versions";

const sections = [
  {
    metadata: { name: "management-database" },
    spec: {
      label: "Databases",
      selection: "One",
      slot: "managementDatabases",
      appliesTo: ["CamundaManagementClusterClaim"],
    },
  },
  {
    metadata: { name: "access" },
    spec: {
      label: "Access",
      selection: "One",
      slot: "externalAccess",
      emptyOptionLabel: "No access from outside",
      appliesTo: ["CamundaClusterClaim", "CamundaManagementClusterClaim"],
    },
  },
] as PluginSection[];

const registration = (
  name: string,
  section: string,
  label: string,
  spec: object = {},
) =>
  ({
    metadata: { name },
    spec: {
      sections: [section],
      mode: "Optional",
      label,
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...spec,
    },
  }) as PluginRegistration;

const gateway = registration("gateway", "access", "Gateway", {
  parameterValues: {
    managementHostnamePattern: "{name}.{namespace}.example.com",
  },
});

const registrations = [
  gateway,
  registration(
    "management-database-server",
    "management-database",
    "New PostgreSQL server",
  ),
  registration(
    "existing-database-configs",
    "management-database",
    "Existing databases",
  ),
];

// production is the Environment of the tests. Unless spec has sections,
// its sections allow each registration of registrations.
const production = (spec: Record<string, unknown> = {}) =>
  ({
    metadata: { name: "production" },
    spec: {
      sections: {
        "management-database": {
          allowed: ["management-database-server", "existing-database-configs"],
        },
        access: { allowed: ["gateway"] },
      },
      ...spec,
    },
  }) as unknown as Environment;

const request = (namespace: string, claim: string, host: string) =>
  ({
    metadata: { namespace, name: `${claim}-access` },
    spec: {
      claimRef: { kind: "CamundaClusterClaim", name: claim },
      section: "access",
      registration: "gateway",
    },
    status: { outputs: { managementHost: host } },
  }) as unknown as PluginRequest;

const plane = (namespace: string, name: string, contract?: string) =>
  ({
    metadata: { namespace, name },
    spec: {
      environments: ["production"],
      management: {},
      versions: { identity: "8.9.10" },
      ...(contract && { managementAuthConfigName: contract }),
    },
  }) as unknown as CamundaManagementClusterClaim;

const ctx: PlaneContext = {
  environments: [production()],
  platformConfigs: [],
  takenNames: [],
  namespaces: [{ name: "platform", environment: "production" }],
  releases: [],
  images: { schemaVersion: 1, minors: {} },
  sections,
  registrations,
  accessRequests: [],
  authConfigs: [],
  planeClaims: [],
};

const database = {
  section: "management-database",
  registration: "management-database-server",
};
const withDatabase = { "management-database": [database] };
const gatewayPick = { section: "access", registration: "gateway" };
const urls = {
  keycloak: "https://m.example.com/auth",
  identity: "https://m.example.com/identity",
};

const draft = (over: Partial<PlaneDraft> = {}): PlaneDraft => ({
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "keycloak",
  admin: { ...EMPTY_ADMIN, username: "admin" },
  storage: { "management-database": [database] },
  urls,
  ...over,
});

const all: Running = { keycloak: true, console: true, webModeler: true };
const configs = (parameters: Record<string, string>) => ({
  section: "management-database",
  registration: "existing-database-configs",
  parameters,
});

describe("databaseSection", () => {
  it("is the section whose slot is managementDatabases", () => {
    expect(databaseSection(sections)?.metadata.name).toBe(
      "management-database",
    );
    expect(databaseSection([sections[1]])).toBeUndefined();
  });
});

describe("databaseConfigErrors", () => {
  it("refuses two equal DatabaseConfig names", () => {
    expect(
      databaseConfigErrors(
        configs({ keycloak: "db", identity: "db", webModeler: "modeler" }),
        all,
      ),
    ).toEqual([
      "Pick a different DatabaseConfig for each app. Keycloak and Management Identity both use db.",
    ]);
  });

  it("refuses an equal name of an app that does not run, as the request holds it", () => {
    expect(
      databaseConfigErrors(
        configs({ keycloak: "db", identity: "db", webModeler: "modeler" }),
        { ...all, keycloak: false },
      ),
    ).toEqual([
      "Pick a different DatabaseConfig for each app. Keycloak and Management Identity both use db.",
    ]);
  });

  it("checks no other registration", () => {
    expect(
      databaseConfigErrors(
        { ...configs({ keycloak: "db", identity: "db" }), registration: "x" },
        all,
      ),
    ).toEqual([]);
  });
});

describe("databaseConfigErrors and missing names", () => {
  it("asks for the DatabaseConfig of each app that runs", () => {
    expect(
      databaseConfigErrors(configs({ identity: "id-db" }), {
        ...all,
        webModeler: false,
      }),
    ).toEqual(["Pick a DatabaseConfig for Keycloak."]);
  });
});

describe("managementHost", () => {
  it("renders managementHostnamePattern of the access selection", () => {
    expect(
      managementHost(
        draft({ storage: { ...withDatabase, access: [gatewayPick] } }),
        ctx,
      ),
    ).toEqual({
      host: "plane-a.platform.example.com",
      pattern: "{name}.{namespace}.example.com",
    });
  });

  it("is undefined without an access selection", () => {
    expect(managementHost(draft(), ctx)).toBeUndefined();
  });
});

describe("contractNameErrors", () => {
  it("an empty contract name checks the claim name", () => {
    expect(contractNameOf(draft())).toBe("plane-a");
    expect(
      contractNameErrors(draft(), { ...ctx, authConfigs: ["plane-a"] }),
    ).toEqual(["A contract plane-a exists already. Enter another name."]);
  });

  it("refuses a name that is not lower case letters, digits, and dashes", () => {
    expect(contractNameErrors(draft({ contractName: "Ops_A" }), ctx)).toEqual([
      "The contract name must be lower case letters, digits, and dashes.",
    ]);
  });

  it("refuses a name that a ManagementAuthConfig has", () => {
    expect(
      contractNameErrors(draft({ contractName: "ops" }), {
        ...ctx,
        authConfigs: ["ops"],
      }),
    ).toEqual(["A contract ops exists already. Enter another name."]);
  });

  it("refuses a name that another plane claim writes, by its contract name or its name", () => {
    const planeClaims = [
      plane("team-b", "plane-b", "ops"),
      plane("team-c", "ops-c"),
    ];
    expect(
      contractNameErrors(draft({ contractName: "ops" }), {
        ...ctx,
        authConfigs: ["ops"],
        planeClaims,
      }),
    ).toEqual([
      "The plane team-b/plane-b writes the contract ops. Enter another name.",
    ]);
    expect(
      contractNameErrors(draft({ contractName: "ops-c" }), {
        ...ctx,
        planeClaims,
      }),
    ).toEqual([
      "The plane team-c/ops-c writes the contract ops-c. Enter another name.",
    ]);
  });

  it("passes a free name", () => {
    expect(contractNameErrors(draft({ contractName: "ops" }), ctx)).toEqual([]);
  });
});

describe("storageErrors", () => {
  it("passes a draft with a database option, its URLs, and a free contract", () => {
    expect(storageErrors(draft(), ctx, {}, {})).toEqual([]);
  });

  it("passes an option that does not support Orphan, the default of the Environment, because the claim then deletes", () => {
    const orphan = {
      ...ctx,
      environments: [production({ deletionPolicy: "Orphan" })],
    };
    expect(storageErrors(draft(), orphan, {}, {})).toEqual([]);
  });

  it("asks for a database option", () => {
    expect(storageErrors(draft({ storage: {} }), ctx, {}, {})).toEqual([
      "Pick a database option.",
    ]);
  });

  it("asks to fix the settings of a shown section whose card reported a field error", () => {
    const databases = {
      name: "management-database",
      label: "Databases",
      selection: "One" as const,
      required: true,
      options: [
        {
          registration: "management-database-server",
          label: "New PostgreSQL server",
          isDefault: true,
          parameters: [],
        },
      ],
    };
    const reported = {
      "management-database": [
        { path: "management-database-server/preset", message: "required" },
      ],
      "dns-zone": [{ path: "route53/zone", message: "required" }],
    };
    expect(storageErrors(draft(), ctx, reported, { databases })).toEqual([
      "Fix the Databases settings.",
    ]);
  });

  it("asks for the URLs without an access selection", () => {
    expect(storageErrors(draft({ urls: {} }), ctx, {}, {})).toEqual([
      "Enter the Keycloak URL.",
      "Enter the Management Identity URL.",
    ]);
  });

  it("a taken management host blocks the step and names the claim that holds it", () => {
    const withAccess = draft({
      storage: { ...withDatabase, access: [gatewayPick] },
      urls: {},
    });
    expect(
      storageErrors(
        withAccess,
        {
          ...ctx,
          accessRequests: [
            request("team-b", "plane-b", "plane-a.platform.example.com"),
          ],
        },
        {},
        {},
      ),
    ).toEqual([
      "The host plane-a.platform.example.com is taken by team-b/plane-b. Choose another name.",
    ]);
  });

  it("a cluster's access request holds the host too", () => {
    const withAccess = draft({
      storage: { ...withDatabase, access: [gatewayPick] },
    });
    const cluster = request(
      "team-c",
      "orders",
      "https://plane-a.platform.example.com",
    );
    expect(
      storageErrors(withAccess, { ...ctx, accessRequests: [cluster] }, {}, {}),
    ).toEqual([
      "The host plane-a.platform.example.com is taken by team-c/orders. Choose another name.",
    ]);
  });

  it("refuses two equal DatabaseConfigs and a taken contract", () => {
    const d = draft({
      storage: {
        "management-database": [configs({ keycloak: "db", identity: "db" })],
      },
      contractName: "ops",
    });
    expect(storageErrors(d, { ...ctx, authConfigs: ["ops"] }, {}, {})).toEqual([
      "Pick a different DatabaseConfig for each app. Keycloak and Management Identity both use db.",
      "A contract ops exists already. Enter another name.",
    ]);
  });

  describe("and an app that does not run", () => {
    const environment = (keycloak: Record<string, unknown>) =>
      production({
        parameters: {
          "existing-database-configs": {
            keycloak,
            identity: { claimEditable: true },
          },
        },
      });
    const errorsOf = (keycloak: Record<string, unknown>, picked: object) =>
      storageErrors(
        draft({
          mode: "oidc",
          storage: { "management-database": [configs({ ...picked })] },
        }),
        { ...ctx, environments: [environment(keycloak)] },
        {},
        {},
      );

    it("ignores the picked name of that app, which the claim leaves out", () => {
      expect(
        errorsOf({ claimEditable: true }, { keycloak: "db", identity: "db" }),
      ).toEqual([]);
    });

    it("refuses a name that the policy gives that app and a running app picks", () => {
      expect(
        errorsOf({ claimEditable: true, value: "db" }, { identity: "db" }),
      ).toEqual([
        "Pick a different DatabaseConfig for each app. Keycloak and Management Identity both use db.",
      ]);
    });
  });

  it("checks the DatabaseConfig names that the policy gives, not only the picked ones", () => {
    const environment = production({
      parameters: {
        "existing-database-configs": {
          keycloak: { claimEditable: true, value: "kc-db" },
          identity: { claimEditable: true, value: "id-db" },
        },
      },
    });
    const d = draft({
      storage: { "management-database": [configs({ identity: "kc-db" })] },
    });
    expect(
      storageErrors(d, { ...ctx, environments: [environment] }, {}, {}),
    ).toEqual([
      "Pick a different DatabaseConfig for each app. Keycloak and Management Identity both use kc-db.",
    ]);
  });

  it("asks for an option of a required section that has none", () => {
    const access = {
      name: "access",
      label: "Access",
      selection: "One" as const,
      required: true,
      options: [],
    };
    expect(storageErrors(draft(), ctx, {}, { access })).toEqual([
      "Choose an option for Access.",
    ]);
  });

  it("refuses a selection whose option the policy no longer offers, or disables", () => {
    const databases = {
      name: "management-database",
      label: "Databases",
      selection: "One" as const,
      required: true,
      options: [],
    };
    expect(storageErrors(draft(), ctx, {}, { databases })).toEqual([
      "Choose another option for Databases.",
    ]);
    const disabled = {
      ...databases,
      options: [
        {
          registration: "management-database-server",
          label: "New PostgreSQL server",
          isDefault: true,
          disabledReason: "Unavailable right now",
          parameters: [],
        },
      ],
    };
    expect(storageErrors(draft(), ctx, {}, { databases: disabled })).toEqual([
      "Choose another option for Databases.",
    ]);
  });
});

describe("storageErrors and a policy that changed after step 4", () => {
  const databases = {
    name: "management-database",
    label: "Databases",
    selection: "One" as const,
    required: true,
    options: [
      {
        registration: "management-database-server",
        label: "New PostgreSQL server",
        isDefault: true,
        parameters: [],
      },
    ],
  };

  it("refuses a pick of a section that no PluginSection offers any more", () => {
    const d = draft({ storage: { ...withDatabase, access: [gatewayPick] } });
    const gone = { ...ctx, sections: [sections[0]] };
    expect(storageErrors(d, gone, {}, { databases })).toEqual([
      "Choose another option for access.",
    ]);
  });

  it("refuses a parameter whose choice the Environment no longer allows, with the refusal of the policy", () => {
    const environment = production({
      parameters: {
        gateway: {
          tier: {
            claimEditable: true,
            allowed: [{ name: "gold", label: "Gold", value: "gold" }],
          },
        },
      },
    });
    const d = draft({
      storage: {
        ...withDatabase,
        access: [{ ...gatewayPick, parameters: { tier: "silver" } }],
      },
    });
    expect(
      storageErrors(d, { ...ctx, environments: [environment] }, {}, {}),
    ).toEqual([
      'parameter "tier" of registration "gateway" has the name "silver", which is not one of gold',
    ]);
  });
});

describe("planeDeletion", () => {
  const orphan = {
    ...ctx,
    environments: [production({ deletionPolicy: "Orphan" })],
  };
  const keeping = {
    ...orphan,
    registrations: registrations.map((r) => ({
      ...r,
      spec: { ...r.spec, deletionPolicies: ["Delete", "Orphan"] },
    })) as PluginRegistration[],
  };

  it("takes the default of the Environment while every pick can keep", () => {
    expect(planeDeletion(draft(), keeping, {})).toMatchObject({
      defaultPolicy: "Orphan",
      keepAllowed: true,
      policy: "Orphan",
    });
  });

  it("does not offer Keep and deletes when the default keeps and a pick supports only Delete", () => {
    expect(planeDeletion(draft(), orphan, {})).toEqual({
      defaultPolicy: "Orphan",
      keepAllowed: false,
      alwaysDeleted: [
        {
          section: "management-database",
          label: "New PostgreSQL server (Databases)",
        },
      ],
      policy: "Delete",
    });
  });

  it("takes the pick of the draft over the default", () => {
    expect(
      planeDeletion(draft({ deletionPolicy: "Delete" }), keeping, {}).policy,
    ).toBe("Delete");
  });
});

describe("planeSectionViews", () => {
  const ready = {
    conditions: [
      { type: "Accepted", status: "True" },
      { type: "Ready", status: "True" },
    ],
  };
  const behindGateway = {
    metadata: { name: "behind-gateway" },
    spec: {
      sections: ["management-database"],
      mode: "Optional",
      label: "Database behind the gateway",
      deletionPolicies: ["Delete"],
      serviceAccount: { name: "db", namespace: "plugins" },
      inputs: [{ name: "host", from: "access/managementHost", required: true }],
    },
    status: ready,
  } as unknown as PluginRegistration;
  const environment = {
    metadata: { name: "production" },
    spec: {
      sections: {
        "management-database": {
          required: true,
          allowed: ["behind-gateway"],
          default: "behind-gateway",
        },
        access: { allowed: ["gateway"] },
      },
    },
  } as unknown as Environment;
  const live: PlaneContext = {
    ...ctx,
    environments: [environment],
    registrations: [
      { ...gateway, status: ready } as PluginRegistration,
      behindGateway,
    ],
  };

  it("asks to keep one option of a section that now takes one selection", () => {
    const c: PlaneContext = {
      ...ctx,
      sections: [
        ...sections,
        {
          metadata: { name: "dns-zone" },
          spec: {
            label: "DNS zone",
            selection: "One",
            appliesTo: ["CamundaManagementClusterClaim"],
          },
        } as unknown as PluginSection,
      ],
    };
    const option = (registration: string) => ({
      registration,
      label: registration,
      isDefault: false,
      parameters: [],
    });
    const dnsZone = {
      name: "dns-zone",
      label: "DNS zone",
      selection: "One" as const,
      required: false,
      options: [option("route53"), option("azure-dns")],
    };
    const d = draft({
      storage: {
        ...withDatabase,
        "dns-zone": [
          { section: "dns-zone", registration: "route53" },
          { section: "dns-zone", registration: "azure-dns" },
        ],
      },
    });
    expect(storageErrors(d, c, {}, { others: [dnsZone] })).toEqual([
      "Choose one option for DNS zone.",
    ]);
  });

  it("treats a section named constructor that has no pick as empty", () => {
    const c: PlaneContext = {
      ...live,
      sections: [
        ...sections,
        {
          metadata: { name: "constructor" },
          spec: {
            label: "Constructor",
            selection: "One",
            appliesTo: ["CamundaManagementClusterClaim"],
          },
        } as unknown as PluginSection,
      ],
    };
    const d = draft({ storage: {} });
    expect(() =>
      storageErrors(d, c, {}, planeSectionViews(d, c, {})),
    ).not.toThrow();
  });

  it("refuses a pick of a section that no longer applies to a plane claim", () => {
    const c: PlaneContext = {
      ...ctx,
      sections: [
        ...sections,
        {
          metadata: { name: "dns-zone" },
          spec: {
            label: "DNS zone",
            selection: "One",
            appliesTo: ["CamundaClusterClaim"],
          },
        } as unknown as PluginSection,
      ],
    };
    const d = draft({
      storage: {
        ...withDatabase,
        "dns-zone": [{ section: "dns-zone", registration: "dns" }],
      },
    });
    expect(goneSections(planeStorageDraft(d, c))).toEqual(["dns-zone"]);
    expect(storageErrors(d, c, {}, {})).toEqual([
      "Choose another option for dns-zone.",
    ]);
  });

  it("adds each other section that applies to a plane, by order, and asks for a required one", () => {
    const section = (name: string, label: string, appliesTo: string[]) =>
      ({
        metadata: { name },
        spec: { label, selection: "One", appliesTo, order: 5 },
      }) as unknown as PluginSection;
    const c: PlaneContext = {
      ...live,
      sections: [
        ...sections,
        section("dns-zone", "DNS zone", ["CamundaManagementClusterClaim"]),
        section("backup-storage", "Backup storage", ["CamundaClusterClaim"]),
      ],
      registrations: [
        ...live.registrations,
        { ...registration("dns", "dns-zone", "Route 53"), status: ready },
      ] as PluginRegistration[],
      environments: [
        production({
          sections: {
            ...environment.spec.sections,
            "dns-zone": { required: true, allowed: ["dns"] },
          },
        }),
      ],
    };
    const d = draft({
      storage: {
        "management-database": [
          { section: "management-database", registration: "behind-gateway" },
        ],
        access: [gatewayPick],
        "dns-zone": [],
      },
    });
    const views = planeSectionViews(d, c, {});
    expect(views.others?.map((v) => v.name)).toEqual(["dns-zone"]);
    expect(storageErrors(d, c, {}, views)).toEqual([
      "Choose an option for DNS zone.",
    ]);
  });

  it("disables an option whose input section has no pick, and the step refuses that pick", () => {
    const d = draft({
      storage: {
        "management-database": [
          { section: "management-database", registration: "behind-gateway" },
        ],
        access: [],
      },
    });
    const views = planeSectionViews(d, live, {});
    expect(views.databases?.options[0].disabledReason).toBe("Needs an access");
    expect(storageErrors(d, live, {}, views)).toContain(
      "Choose another option for Databases.",
    );
  });

  it("enables the option once its input section has a pick", () => {
    const d = draft({
      storage: {
        "management-database": [
          { section: "management-database", registration: "behind-gateway" },
        ],
        access: [gatewayPick],
      },
    });
    expect(
      planeSectionViews(d, live, {}).databases?.options[0].disabledReason,
    ).toBeUndefined();
  });
});

describe("storageErrors and a live choice that is gone", () => {
  const ready = {
    conditions: [
      { type: "Accepted", status: "True" },
      { type: "Ready", status: "True" },
    ],
  };
  const builtin = (
    name: string,
    label: string,
    properties: Record<string, unknown>,
    required: string[] = [],
  ) =>
    ({
      metadata: { name },
      spec: {
        sections: ["management-database"],
        mode: "Optional",
        label,
        deletionPolicies: ["Delete"],
        serviceAccount: { name: "camunda", namespace: "camunda-system" },
        controller: { name: "camunda-operator", version: "0.0.0" },
        parameters: { type: "object", properties, required },
      },
      status: ready,
    }) as unknown as PluginRegistration;
  const app = (title: string) => ({ type: "string", title });
  const environment = (spec: Record<string, unknown>) =>
    ({
      metadata: { name: "production" },
      spec: {
        sections: {
          "management-database": {
            required: true,
            allowed: [
              "existing-database-configs",
              "management-database-server",
            ],
          },
        },
        ...spec,
      },
    }) as unknown as Environment;
  const editable = { claimEditable: true };
  const live = (spec: Record<string, unknown>): PlaneContext => ({
    ...ctx,
    environments: [environment(spec)],
    registrations: [
      builtin("existing-database-configs", "Existing databases", {
        keycloak: app("Keycloak"),
        identity: app("Management Identity"),
        webModeler: app("Web Modeler"),
      }),
      builtin(
        "management-database-server",
        "New PostgreSQL server",
        {
          preset: { type: "string", title: "Preset" },
          version: {
            type: "string",
            title: "PostgreSQL version",
            pattern: "^\\d+$",
          },
          zones: {
            type: "array",
            title: "Zones",
            maxItems: 1,
            items: { type: "string" },
          },
        },
        ["preset"],
      ),
    ],
  });
  const configSpec = {
    parameters: {
      "existing-database-configs": {
        keycloak: editable,
        identity: editable,
        webModeler: editable,
      },
    },
  };
  const presetSpec = {
    parameters: { "management-database-server": { preset: editable } },
  };
  const config = (name: string) => ({
    metadata: { namespace: "platform", name },
    spec: {},
  });
  const preset = (name: string) => ({
    metadata: { name },
    spec: { server: { instances: 1 } },
  });
  const errorsOf = (d: PlaneDraft, c: PlaneContext, objects: object) =>
    storageErrors(d, c, {}, planeSectionViews(d, c, objects), objects);
  const keycloakOnly = { mode: "keycloak" as const };

  it("refuses a picked DatabaseConfig that is gone while another one is left, and names it", () => {
    const d = draft({
      ...keycloakOnly,
      storage: {
        "management-database": [
          configs({ keycloak: "kc-db", identity: "id-db" }),
        ],
      },
    });
    const c = live(configSpec);
    expect(
      errorsOf(d, c, {
        DatabaseConfig: [config("kc-db"), config("id-db"), config("other")],
      }),
    ).toEqual([]);
    expect(
      errorsOf(d, c, { DatabaseConfig: [config("id-db"), config("other")] }),
    ).toEqual([
      "kc-db is not available for Keycloak of Existing databases. Choose another.",
    ]);
  });

  it("refuses a DatabaseConfig that the policy gives and that is gone", () => {
    const d = draft({
      ...keycloakOnly,
      storage: { "management-database": [configs({ identity: "id-db" })] },
    });
    const c = live({
      parameters: {
        "existing-database-configs": {
          ...configSpec.parameters["existing-database-configs"],
          keycloak: { claimEditable: true, value: "kc-db" },
        },
      },
    });
    expect(
      errorsOf(d, c, { DatabaseConfig: [config("id-db"), config("other")] }),
    ).toEqual([
      "kc-db is not available for Keycloak of Existing databases. Choose another.",
    ]);
  });

  it("refuses a picked preset that the Environment no longer lists while another one is left", () => {
    const d = draft({
      storage: {
        "management-database": [
          { ...database, parameters: { preset: "small" } },
        ],
      },
    });
    const objects = {
      DatabaseServerPreset: [preset("small"), preset("large")],
    };
    expect(
      errorsOf(
        d,
        live({
          ...presetSpec,
          presets: { databaseServer: ["small", "large"] },
        }),
        objects,
      ),
    ).toEqual([]);
    expect(
      errorsOf(
        d,
        live({ ...presetSpec, presets: { databaseServer: ["large"] } }),
        objects,
      ),
    ).toEqual([
      "small is not available for Preset of New PostgreSQL server. Choose another.",
    ]);
  });

  it("asks for a required preset that the selection leaves out once the Environment gives no value", () => {
    const d = draft({ storage: { "management-database": [database] } });
    const objects = { DatabaseServerPreset: [preset("large")] };
    const withValue = {
      parameters: {
        "management-database-server": {
          preset: { claimEditable: true, value: "large" },
        },
      },
      presets: { databaseServer: ["large"] },
    };
    expect(errorsOf(d, live(withValue), objects)).toEqual([]);
    expect(
      errorsOf(
        d,
        live({ ...presetSpec, presets: { databaseServer: ["large"] } }),
        objects,
      ),
    ).toEqual(["Choose a Preset for New PostgreSQL server."]);
  });

  it("refuses a policy list of names that has more items than its schema allows", () => {
    const d = draft({
      storage: {
        "management-database": [
          { ...database, parameters: { preset: "large" } },
        ],
      },
    });
    const objects = { DatabaseServerPreset: [preset("large")] };
    const zone = (name: string) => ({ name, label: name, value: name });
    const withZones = (value: string[]) =>
      live({
        parameters: {
          "management-database-server": {
            preset: editable,
            zones: {
              claimEditable: true,
              allowed: [zone("a"), zone("b")],
              value,
            },
          },
        },
        presets: { databaseServer: ["large"] },
      });
    expect(errorsOf(d, withZones(["a"]), objects)).toEqual([]);
    expect(errorsOf(d, withZones(["a", "b"]), objects)).toEqual([
      "Fix the Zones of New PostgreSQL server.",
    ]);
  });

  it("refuses a preset that an allowed name of the Environment gives once the Environment no longer lists it", () => {
    const d = draft({
      storage: {
        "management-database": [
          { ...database, parameters: { preset: "small-option" } },
        ],
      },
    });
    const objects = {
      DatabaseServerPreset: [preset("small"), preset("large")],
    };
    const withPresets = (listed: string[]) =>
      live({
        parameters: {
          "management-database-server": {
            preset: {
              claimEditable: true,
              allowed: [
                { name: "small-option", label: "Small", value: "small" },
              ],
            },
          },
        },
        presets: { databaseServer: listed },
      });
    expect(errorsOf(d, withPresets(["small", "large"]), objects)).toEqual([]);
    expect(errorsOf(d, withPresets(["large"]), objects)).toEqual([
      'DatabaseServerPreset "small" is not in spec.presets.databaseServer of Environment "production"',
    ]);
  });

  it("refuses a policy value that its schema refuses", () => {
    const d = draft({
      storage: {
        "management-database": [
          { ...database, parameters: { preset: "large" } },
        ],
      },
    });
    const objects = { DatabaseServerPreset: [preset("large")] };
    const withVersion = (value: string) =>
      live({
        parameters: {
          "management-database-server": {
            preset: editable,
            version: { claimEditable: true, value },
          },
        },
        presets: { databaseServer: ["large"] },
      });
    expect(errorsOf(d, withVersion("17"), objects)).toEqual([]);
    expect(errorsOf(d, withVersion("not-a-version"), objects)).toEqual([
      "Fix the PostgreSQL version of New PostgreSQL server.",
    ]);
  });

  it("refuses an empty policy value that its schema refuses, also when the parameter is optional", () => {
    const d = draft({
      storage: {
        "management-database": [
          { ...database, parameters: { preset: "large" } },
        ],
      },
    });
    const c = live({
      parameters: {
        "management-database-server": {
          preset: editable,
          version: { claimEditable: true, value: "" },
        },
      },
      presets: { databaseServer: ["large"] },
    });
    expect(errorsOf(d, c, { DatabaseServerPreset: [preset("large")] })).toEqual(
      ["Fix the PostgreSQL version of New PostgreSQL server."],
    );
  });

  it("passes a database registration whose schema requires the databases that the controller sets", () => {
    const custom = builtin("custom-db", "Custom database", {
      databases: { type: "array", items: { type: "string" }, minItems: 1 },
    });
    const c: PlaneContext = {
      ...live({
        sections: {
          "management-database": { required: true, allowed: ["custom-db"] },
        },
      }),
      registrations: [
        {
          ...custom,
          spec: {
            ...custom.spec,
            controller: undefined,
            parameters: {
              type: "object",
              properties: {
                databases: {
                  type: "array",
                  items: { type: "string" },
                  minItems: 1,
                },
              },
              required: ["databases"],
            },
          },
        } as PluginRegistration,
      ],
    };
    const d = draft({
      storage: {
        "management-database": [
          { section: "management-database", registration: "custom-db" },
        ],
      },
    });
    expect(errorsOf(d, c, {})).toEqual([]);
  });

  it("refuses a DatabaseConfig that the policy gives an app that does not run once it is gone", () => {
    const d = draft({
      mode: "oidc",
      storage: { "management-database": [configs({ identity: "id-db" })] },
    });
    const c = live({
      parameters: {
        "existing-database-configs": {
          ...configSpec.parameters["existing-database-configs"],
          keycloak: { claimEditable: true, value: "kc-db" },
        },
      },
    });
    expect(
      errorsOf(d, c, { DatabaseConfig: [config("kc-db"), config("id-db")] }),
    ).toEqual([]);
    expect(errorsOf(d, c, { DatabaseConfig: [config("id-db")] })).toEqual([
      "The DatabaseConfig kc-db that the policy gives Keycloak is not in platform. Ask your admin to change the policy of the Environment.",
    ]);
  });

  it("refuses a locked policy value that the schema of its registration refuses", () => {
    const d = draft({
      storage: {
        "management-database": [
          { ...database, parameters: { preset: "large" } },
        ],
      },
    });
    const withVersion = (value: string) =>
      live({
        parameters: {
          "management-database-server": {
            preset: editable,
            version: { value },
          },
        },
        presets: { databaseServer: ["large"] },
      });
    const objects = { DatabaseServerPreset: [preset("large")] };
    expect(errorsOf(d, withVersion("17"), objects)).toEqual([]);
    expect(errorsOf(d, withVersion("not-a-version"), objects)).toEqual([
      expect.stringMatching(
        /^The policy gives New PostgreSQL server a value that its schema refuses: version .+\. Pick another option, or ask your admin to change the policy of the Environment\.$/,
      ),
    ]);
  });

  it("refuses a DatabaseConfig that an allowed name of the Environment gives once it is gone", () => {
    const d = draft({
      ...keycloakOnly,
      storage: {
        "management-database": [
          configs({ keycloak: "kc-db", identity: "gold" }),
        ],
      },
    });
    const c = live({
      parameters: {
        "existing-database-configs": {
          ...configSpec.parameters["existing-database-configs"],
          identity: {
            claimEditable: true,
            allowed: [{ name: "gold", label: "Gold", value: "id-db" }],
          },
        },
      },
    });
    expect(
      errorsOf(d, c, { DatabaseConfig: [config("kc-db"), config("id-db")] }),
    ).toEqual([]);
    expect(
      errorsOf(d, c, { DatabaseConfig: [config("kc-db"), config("other")] }),
    ).toEqual([
      "Pick another DatabaseConfig for Management Identity. id-db is not in platform.",
    ]);
  });
});

// The Go source is the source of the suffixes. A change on one side alone
// fails here.
it("DATABASE_SUFFIX matches NameSuffix of pkg/planedatabases", () => {
  const source = readFileSync(
    new URL(
      "../../../../pkg/planedatabases/planedatabases.go",
      import.meta.url,
    ),
    "utf8",
  );
  const components = new Map(
    [...source.matchAll(/(\w+)\s+Component\s*=\s*"([^"]*)"/g)].map((m) => [
      m[1],
      m[2],
    ]),
  );
  const suffixes = Object.fromEntries(
    [
      ...source.matchAll(/Component:\s*(\w+),[^}]*?NameSuffix:\s*"([^"]*)"/g),
    ].map((m) => [components.get(m[1]), m[2]]),
  );
  expect(DATABASE_SUFFIX).toEqual(suffixes);
});
