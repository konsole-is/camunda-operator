import { describe, expect, it } from "vitest";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { EMPTY_ADMIN } from "./components";
import { EMPTY_PLANE_DRAFT, type PlaneContext, type PlaneDraft } from "./draft";
import {
  contractNameErrors,
  contractNameOf,
  databaseConfigErrors,
  databaseSection,
  managementHost,
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

const gateway = {
  metadata: { name: "gateway" },
  spec: {
    sections: ["access"],
    mode: "Optional",
    label: "Gateway",
    deletionPolicies: ["Delete"],
    serviceAccount: { name: "gateway", namespace: "plugins" },
    parameterValues: {
      managementHostnamePattern: "{name}.{namespace}.example.com",
    },
  },
} as PluginRegistration;

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
  environments: [{ metadata: { name: "production" }, spec: {} } as Environment],
  platformConfigs: [],
  takenNames: [],
  namespaces: [{ name: "platform", environment: "production" }],
  releases: [],
  images: { schemaVersion: 1, minors: {} },
  sections,
  registrations: [gateway],
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

  it("ignores the name of an app that does not run", () => {
    expect(
      databaseConfigErrors(configs({ keycloak: "db", identity: "db" }), {
        ...all,
        keycloak: false,
      }),
    ).toEqual([]);
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
    expect(storageErrors(draft(), ctx, [])).toEqual([]);
  });

  it("asks for a database option", () => {
    expect(storageErrors(draft({ storage: {} }), ctx, [])).toEqual([
      "Pick a database option.",
    ]);
  });

  it("asks to fix the fields of the options that SectionCards reported", () => {
    expect(
      storageErrors(draft(), ctx, [
        { path: "existing-database-configs/identity", message: "required" },
      ]),
    ).toEqual(["Fix the fields of the databases and access options."]);
  });

  it("asks for the URLs without an access selection", () => {
    expect(storageErrors(draft({ urls: {} }), ctx, [])).toEqual([
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
        [],
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
      storageErrors(withAccess, { ...ctx, accessRequests: [cluster] }, []),
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
    expect(storageErrors(d, { ...ctx, authConfigs: ["ops"] }, [])).toEqual([
      "Pick a different DatabaseConfig for each app. Keycloak and Management Identity both use db.",
      "A contract ops exists already. Enter another name.",
    ]);
  });
});
