import { describe, expect, it } from "vitest";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import { parseImagesTable } from "@/startup/bundled-data";
import { EMPTY_ADMIN, EMPTY_MAIL } from "./components";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
  basicsErrors,
  componentsErrors,
  planeClaimOf,
  planeCreateChange,
  platformConfigOf,
  servedErrors,
  servedOf,
} from "./draft";

const environment = (
  name: string,
  platformConfigRef?: string,
  extra: object = {},
): Environment =>
  ({
    metadata: { name },
    spec: platformConfigRef ? { platformConfigRef } : {},
    ...extra,
  }) as Environment;

const images = parseImagesTable({
  schemaVersion: 1,
  minors: {
    "8.9": {
      identity: { version: "8.9.10" },
      console: { version: "8.9.122" },
      "web-modeler-restapi": { version: "8.9.11" },
      "web-modeler-websockets": { version: "8.9.9" },
      keycloak: { tagPrefix: "quay-optimized-", version: "26.6.4" },
    },
  },
});

const ctx: PlaneContext = {
  environments: [
    environment("production", "default-platform", {
      spec: {
        platformConfigRef: "default-platform",
        releases: { default: "camunda-8-9-23" },
      },
    }),
    environment("staging", "default-platform"),
    environment("bare"),
    environment("dangling", "gone"),
    environment("open", "default-platform", {
      spec: {
        platformConfigRef: "default-platform",
        namespaceCreation: {
          allowed: true,
          labels: { tier: "gold" },
          requiredLabelKeys: ["tier", "team"],
        },
      },
    }),
    environment("broken", "default-platform", {
      status: {
        conditions: [
          {
            type: "Ready",
            status: "False",
            reason: "InvalidReference",
            message: "CamundaRelease 8.9 does not exist",
            lastTransitionTime: "2026-10-30T00:00:00Z",
          },
        ],
      },
    }),
  ],
  namespaces: [
    { name: "platform", environment: "production" },
    { name: "ops", environment: "staging" },
    { name: "team-a", environment: "open" },
    { name: "team-b", environment: "broken" },
    { name: "bare-ns", environment: "bare" },
    { name: "dangling-ns", environment: "dangling" },
  ],
  platformConfigs: [
    { metadata: { name: "default-platform" }, spec: {} },
    {
      metadata: { name: "oidc-platform" },
      spec: { auth: { method: "oidc", oidc: { issuerUrl: "https://idp" } } },
    },
  ] as unknown as CamundaPlatformConfig[],
  takenNames: ["taken"],
  releases: [
    {
      metadata: { name: "camunda-8-9-23" },
      spec: { version: "8.9.23" },
    } as CamundaRelease,
  ],
  images,
  sections: [],
  registrations: [],
  accessRequests: [],
  authConfigs: [],
  planeClaims: [],
};

const ready = (): PlaneDraft => ({
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "keycloak",
  admin: { ...EMPTY_ADMIN, username: "admin" },
});

describe("EMPTY_PLANE_DRAFT", () => {
  it("prefills the realm camunda-platform", () => {
    expect(EMPTY_PLANE_DRAFT.externalKeycloak.realm).toBe("camunda-platform");
  });
});

describe("platformConfigOf", () => {
  it("is the environment default without a pick", () => {
    expect(platformConfigOf(ready(), ctx)).toBe("default-platform");
  });

  it("is the pick of the admin", () => {
    expect(
      platformConfigOf({ ...ready(), platformConfig: "oidc-platform" }, ctx),
    ).toBe("oidc-platform");
  });

  it("is undefined when the environment names none and nothing is picked", () => {
    expect(platformConfigOf({ ...ready(), environment: "bare" }, ctx)).toBe(
      undefined,
    );
  });
});

describe("basicsErrors", () => {
  it("passes a complete draft", () => {
    expect(basicsErrors(ready(), ctx)).toEqual([]);
  });

  it("blocks step 1 without any Environment", () => {
    expect(basicsErrors(ready(), { ...ctx, environments: [] })).toEqual([
      "No Environment exists. A management plane serves environments, so create an Environment first.",
    ]);
  });

  it("asks for an environment, a namespace, and a mode in an empty draft", () => {
    expect(basicsErrors(EMPTY_PLANE_DRAFT, ctx)).toEqual([
      "Pick an environment.",
      "Enter a name",
      "Pick or create a namespace.",
      "Pick a platform config.",
      "Pick where users sign in.",
    ]);
  });

  it("refuses a name that is taken in the namespace", () => {
    expect(basicsErrors({ ...ready(), name: "taken" }, ctx)).toEqual([
      "The name taken is taken in this namespace.",
    ]);
  });

  it("blocks step 1 when the environment names no platform config and none is picked", () => {
    const d: PlaneDraft = {
      ...ready(),
      environment: "bare",
      namespace: { kind: "existing", name: "bare-ns" },
    };
    expect(basicsErrors(d, ctx)).toEqual(["Pick a platform config."]);
  });

  it("blocks step 1 when the platform config of the environment does not exist", () => {
    const d: PlaneDraft = {
      ...ready(),
      environment: "dangling",
      namespace: { kind: "existing", name: "dangling-ns" },
    };
    expect(basicsErrors(d, ctx)).toEqual([
      "Platform config gone of the environment does not exist. Pick another one.",
    ]);
  });

  it("blocks step 1 when the picked platform config does not exist", () => {
    expect(basicsErrors({ ...ready(), platformConfig: "gone" }, ctx)).toEqual([
      "Platform config gone does not exist. Pick another one.",
    ]);
  });

  it("blocks OIDC with a basic-auth platform config", () => {
    expect(basicsErrors({ ...ready(), mode: "oidc" }, ctx)).toEqual([
      "OIDC needs a platform config with OIDC authentication.",
    ]);
  });

  it("passes OIDC with an oidc platform config", () => {
    expect(
      basicsErrors(
        { ...ready(), mode: "oidc", platformConfig: "oidc-platform" },
        ctx,
      ),
    ).toEqual([]);
  });

  it("refuses a CA bundle Secret of an external Keycloak that is gone or lacks its key", () => {
    const d: PlaneDraft = {
      ...ready(),
      mode: "externalKeycloak",
      externalKeycloak: {
        url: "https://kc.example.com/auth",
        realm: "",
        adminSecret: "kc-admin",
        caBundle: { name: "kc-ca", key: "ca.crt" },
      },
    };
    expect(basicsErrors(d, ctx, [{ name: "kc-ca", keys: ["ca.crt"] }])).toEqual(
      [],
    );
    expect(
      basicsErrors(d, ctx, [{ name: "kc-ca", keys: ["tls.crt"] }]),
    ).toEqual([
      "Pick another CA bundle Secret. kc-ca is gone, or it lacks the key ca.crt.",
    ]);
  });

  it("checks the External Keycloak fields in that mode only", () => {
    const external = { url: "kc", realm: "", adminSecret: "" };
    expect(
      basicsErrors(
        { ...ready(), mode: "externalKeycloak", externalKeycloak: external },
        ctx,
      ),
    ).toEqual([
      "Use an http or https URL with a host.",
      "Name the Secret with the Keycloak administrator credentials.",
    ]);
    expect(
      basicsErrors({ ...ready(), externalKeycloak: external }, ctx),
    ).toEqual([]);
  });
});

describe("basicsErrors and the live data", () => {
  it("refuses an environment that turned Ready=False, with its message", () => {
    expect(
      basicsErrors(
        {
          ...ready(),
          environment: "broken",
          namespace: { kind: "existing", name: "team-b" },
        },
        ctx,
      ),
    ).toEqual([
      "The environment broken is not ready: CamundaRelease 8.9 does not exist",
    ]);
  });

  it("refuses an existing namespace that is gone", () => {
    expect(
      basicsErrors(
        { ...ready(), namespace: { kind: "existing", name: "gone" } },
        ctx,
      ),
    ).toEqual(["Pick or create a namespace."]);
  });

  it("refuses an existing namespace of another environment", () => {
    expect(
      basicsErrors(
        { ...ready(), namespace: { kind: "existing", name: "ops" } },
        ctx,
      ),
    ).toEqual(["Pick or create a namespace."]);
  });

  it("refuses a new namespace in an environment that allows none", () => {
    expect(
      basicsErrors(
        { ...ready(), namespace: { kind: "new", name: "fresh", labels: {} } },
        ctx,
      ),
    ).toEqual(["Pick or create a namespace."]);
  });

  it("refuses a new namespace whose name exists, and asks for each missing required label", () => {
    expect(
      basicsErrors(
        {
          ...ready(),
          environment: "open",
          namespace: { kind: "new", name: "ops", labels: {} },
        },
        ctx,
      ),
    ).toEqual(["The namespace ops already exists.", "Enter a value for team."]);
  });

  it("passes a new namespace with the required labels", () => {
    expect(
      basicsErrors(
        {
          ...ready(),
          environment: "open",
          namespace: { kind: "new", name: "fresh", labels: { team: "a" } },
        },
        ctx,
      ),
    ).toEqual([]);
  });
});

describe("servedOf", () => {
  it("serves the environment of step 1 until the admin picks", () => {
    expect(servedOf({ ...ready(), environment: "staging" })).toEqual([
      "staging",
    ]);
    expect(servedOf({ ...ready(), served: ["production", "staging"] })).toEqual(
      ["production", "staging"],
    );
  });

  it("serves nothing without an environment", () => {
    expect(servedOf(EMPTY_PLANE_DRAFT)).toEqual([]);
  });
});

describe("servedErrors", () => {
  it("passes the environment of step 1", () => {
    expect(servedErrors(ready(), ctx, [])).toEqual([]);
  });

  it("blocks step 2 with no environment picked", () => {
    expect(servedErrors({ ...ready(), served: [] }, ctx, [])).toEqual([
      "Pick at least one environment.",
    ]);
  });

  it("refuses an environment that is gone", () => {
    expect(
      servedErrors({ ...ready(), served: ["production", "old"] }, ctx, []),
    ).toEqual(["Environment old does not exist."]);
  });

  it("blocks while the cluster labels have errors", () => {
    expect(
      servedErrors(ready(), ctx, [{ path: "0.key", message: "Enter a key" }]),
    ).toEqual(["Fix the cluster labels."]);
  });
});

describe("planeClaimOf", () => {
  it("names the claim and its namespace", () => {
    expect(planeClaimOf(ready(), ctx)).toMatchObject({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaManagementClusterClaim",
      metadata: { name: "plane-a", namespace: "platform" },
    });
  });

  it("writes clusterSelector {} without cluster labels", () => {
    expect(planeClaimOf(ready(), ctx).spec.clusterSelector).toEqual({});
  });

  it("writes the cluster labels as matchLabels", () => {
    expect(
      planeClaimOf({ ...ready(), clusterLabels: { tier: "gold" } }, ctx).spec
        .clusterSelector,
    ).toEqual({ matchLabels: { tier: "gold" } });
  });

  it("leaves platformConfigRef out when the pick is the environment default", () => {
    expect(
      planeClaimOf({ ...ready(), platformConfig: "default-platform" }, ctx)
        .spec,
    ).not.toHaveProperty("platformConfigRef");
    expect(planeClaimOf(ready(), ctx).spec).not.toHaveProperty(
      "platformConfigRef",
    );
  });

  it("writes platformConfigRef when the admin picks another one", () => {
    expect(
      planeClaimOf({ ...ready(), platformConfig: "oidc-platform" }, ctx).spec
        .platformConfigRef,
    ).toBe("oidc-platform");
  });

  it("writes platformConfigRef when the environment names none", () => {
    const d = {
      ...ready(),
      environment: "bare",
      platformConfig: "default-platform",
    };
    expect(planeClaimOf(d, ctx).spec.platformConfigRef).toBe(
      "default-platform",
    );
  });

  it("writes the served environments by object name", () => {
    expect(
      planeClaimOf({ ...ready(), served: ["production", "staging"] }, ctx).spec
        .environments,
    ).toEqual(["production", "staging"]);
    expect(planeClaimOf(ready(), ctx).spec.environments).toEqual([
      "production",
    ]);
  });

  it.each([
    ["keycloak", { keycloak: {} }],
    ["oidc", { oidc: {} }],
  ] as const)("writes the %s identity provider", (mode, want) => {
    expect(
      planeClaimOf({ ...ready(), mode }, ctx).spec.management.identityProvider,
    ).toEqual(want);
  });

  it("writes an external Keycloak with the default credential keys", () => {
    const d: PlaneDraft = {
      ...ready(),
      mode: "externalKeycloak",
      externalKeycloak: {
        url: "https://kc/auth",
        realm: "",
        adminSecret: "kc-admin",
      },
    };
    expect(planeClaimOf(d, ctx).spec.management.identityProvider).toEqual({
      externalKeycloak: {
        url: "https://kc/auth",
        adminCredentialsSecretRef: { name: "kc-admin" },
      },
    });
  });

  it("writes the realm and the CA bundle of an external Keycloak when set", () => {
    const d: PlaneDraft = {
      ...ready(),
      mode: "externalKeycloak",
      externalKeycloak: {
        url: "https://kc/auth",
        realm: "ops",
        adminSecret: "kc-admin",
        caBundle: { name: "kc-ca", key: "ca.crt" },
      },
    };
    expect(planeClaimOf(d, ctx).spec.management.identityProvider).toEqual({
      externalKeycloak: {
        url: "https://kc/auth",
        realm: "ops",
        adminCredentialsSecretRef: { name: "kc-admin" },
        caBundleSecretRef: { name: "kc-ca", key: "ca.crt" },
      },
    });
  });
});

describe("componentsErrors", () => {
  it("passes a Keycloak draft with a username and the prefilled versions", () => {
    expect(componentsErrors(ready(), ctx)).toEqual([]);
  });

  it("checks no mail while Web Modeler is off", () => {
    expect(
      componentsErrors({ ...ready(), mail: { ...EMPTY_MAIL } }, ctx),
    ).toEqual([]);
  });

  it("gives the admin errors, then the mail errors, then the version errors", () => {
    const d: PlaneDraft = {
      ...ready(),
      admin: EMPTY_ADMIN,
      webModeler: true,
      versions: { keycloak: "27.0.0" },
    };
    expect(componentsErrors(d, ctx)).toEqual([
      "Enter the username of the first administrator.",
      "Web Modeler needs the email of the first administrator.",
      "Enter the SMTP host.",
      "Enter the address that Web Modeler sends from.",
      "Keycloak needs a version from 26.0.0 up to, but not including, 27.0.0.",
    ]);
  });

  it("refuses a password Secret that is gone or lacks its key, in the Keycloak modes only", () => {
    const d: PlaneDraft = {
      ...ready(),
      admin: {
        ...EMPTY_ADMIN,
        username: "admin",
        password: { name: "admin-pw", key: "password" },
      },
    };
    expect(
      componentsErrors(d, ctx, [{ name: "admin-pw", keys: ["password"] }]),
    ).toEqual([]);
    expect(componentsErrors(d, ctx, [])).toEqual([
      "Pick another Password Secret. admin-pw is gone, or it lacks the key password.",
    ]);
    expect(
      componentsErrors(
        {
          ...d,
          mode: "oidc",
          admin: { ...d.admin, claimName: "sub", claimValue: "x" },
        },
        ctx,
        [],
      ),
    ).toEqual([]);
  });

  it("refuses an SMTP Secret that is gone or lacks the username or password key", () => {
    const d: PlaneDraft = {
      ...ready(),
      admin: { ...EMPTY_ADMIN, username: "admin", email: "admin@example.com" },
      webModeler: true,
      mail: {
        ...EMPTY_MAIL,
        smtpHost: "smtp.example.com",
        fromAddress: "modeler@example.com",
        credentialsSecret: "smtp",
      },
    };
    const refused = [
      "Pick another SMTP credentials Secret. smtp is gone, or it lacks the key username or password.",
    ];
    expect(
      componentsErrors(d, ctx, [
        { name: "smtp", keys: ["username", "password"] },
      ]),
    ).toEqual([]);
    expect(componentsErrors(d, ctx, [])).toEqual(refused);
    expect(
      componentsErrors(d, ctx, [{ name: "smtp", keys: ["username"] }]),
    ).toEqual(refused);
  });

  it("asks for each version when the served environment has no default release", () => {
    expect(componentsErrors({ ...ready(), served: ["staging"] }, ctx)).toEqual([
      "Enter the Management Identity version.",
      "Enter the Keycloak version.",
    ]);
  });
});

describe("planeClaimOf and the components", () => {
  const admin = {
    username: "admin",
    email: "a@b.c",
    password: { name: "pw", key: "p" },
    claimName: "sub",
    claimValue: "42",
  };

  it("writes claimName and claimValue in OIDC mode, and no username, email, or password", () => {
    const d: PlaneDraft = { ...ready(), mode: "oidc", admin };
    expect(planeClaimOf(d, ctx).spec.management.identity).toEqual({
      admin: { claimName: "sub", claimValue: "42" },
    });
  });

  it("writes username, email, and the password Secret in Keycloak mode, and no claim", () => {
    expect(
      planeClaimOf({ ...ready(), admin }, ctx).spec.management.identity,
    ).toEqual({
      admin: {
        username: "admin",
        email: "a@b.c",
        passwordSecretRef: { name: "pw", key: "p" },
      },
    });
    expect(planeClaimOf(ready(), ctx).spec.management.identity).toEqual({
      admin: { username: "admin" },
    });
  });

  it("writes console {} and its version only when Console is on", () => {
    const on = planeClaimOf({ ...ready(), console: true }, ctx).spec;
    expect(on.management.console).toEqual({});
    expect(on.versions.console).toBe("8.9.122");
    const off = planeClaimOf(
      { ...ready(), versions: { console: "8.9.1" } },
      ctx,
    ).spec;
    expect(off.management).not.toHaveProperty("console");
    expect(off.versions).not.toHaveProperty("console");
  });

  it("writes webModeler with mail and its version only when Web Modeler is on", () => {
    const d: PlaneDraft = {
      ...ready(),
      webModeler: true,
      mail: {
        smtpHost: "smtp.example.com",
        smtpPort: 25,
        fromAddress: "modeler@example.com",
        fromName: "",
        tls: false,
        credentialsSecret: "smtp",
      },
    };
    const spec = planeClaimOf(d, ctx).spec;
    expect(spec.management.webModeler).toEqual({
      mail: {
        smtpHost: "smtp.example.com",
        smtpPort: 25,
        fromAddress: "modeler@example.com",
        tls: false,
        credentialsSecretRef: { name: "smtp" },
      },
    });
    expect(spec.versions.webModeler).toBe("8.9.11");
    const off = planeClaimOf({ ...d, webModeler: false }, ctx).spec;
    expect(off.management).not.toHaveProperty("webModeler");
    expect(off.versions).not.toHaveProperty("webModeler");
  });

  it("writes versions.keycloak only in Keycloak mode", () => {
    expect(planeClaimOf(ready(), ctx).spec.versions.keycloak).toBe("26.6.4");
    expect(
      planeClaimOf({ ...ready(), mode: "oidc" }, ctx).spec.versions,
    ).not.toHaveProperty("keycloak");
  });

  it("writes the prefilled versions of the default release minor, and a typed version over it", () => {
    expect(
      planeClaimOf({ ...ready(), console: true }, ctx).spec.versions,
    ).toEqual({ identity: "8.9.10", console: "8.9.122", keycloak: "26.6.4" });
    expect(
      planeClaimOf({ ...ready(), versions: { identity: "8.9.11" } }, ctx).spec
        .versions.identity,
    ).toBe("8.9.11");
  });
});

describe("planeClaimOf and the deletion policy", () => {
  const database = {
    section: "management-database",
    registration: "management-database-server",
  };
  const server = (deletionPolicies: ("Delete" | "Orphan")[]) =>
    ({
      metadata: { name: "management-database-server" },
      spec: {
        sections: ["management-database"],
        mode: "Optional",
        label: "New PostgreSQL server",
        deletionPolicies,
        serviceAccount: { name: "camunda", namespace: "camunda-system" },
      },
    }) as PlaneContext["registrations"][number];
  const withPolicy = (
    deletionPolicy: "Delete" | "Orphan" | undefined,
    deletionPolicies: ("Delete" | "Orphan")[],
  ): PlaneContext => ({
    ...ctx,
    environments: [
      environment("production", "default-platform", {
        spec: {
          platformConfigRef: "default-platform",
          ...(deletionPolicy && { deletionPolicy }),
        },
      }),
    ],
    registrations: [server(deletionPolicies)],
  });
  const picked = (deletionPolicy?: "Delete" | "Orphan"): PlaneDraft => ({
    ...ready(),
    storage: { [database.section]: [database] },
    ...(deletionPolicy && { deletionPolicy }),
  });

  it("leaves deletionPolicy out while the claim takes the default of the Environment", () => {
    expect(
      planeClaimOf(picked(), withPolicy("Orphan", ["Delete", "Orphan"])).spec,
    ).not.toHaveProperty("deletionPolicy");
    expect(
      planeClaimOf(picked("Delete"), withPolicy(undefined, ["Delete"])).spec,
    ).not.toHaveProperty("deletionPolicy");
  });

  it("writes the deletion pick that differs from the default", () => {
    expect(
      planeClaimOf(picked("Delete"), withPolicy("Orphan", ["Delete", "Orphan"]))
        .spec.deletionPolicy,
    ).toBe("Delete");
    expect(
      planeClaimOf(
        picked("Orphan"),
        withPolicy(undefined, ["Delete", "Orphan"]),
      ).spec.deletionPolicy,
    ).toBe("Orphan");
  });

  it("writes Delete when the default keeps and a pick supports only Delete", () => {
    expect(
      planeClaimOf(picked(), withPolicy("Orphan", ["Delete"])).spec
        .deletionPolicy,
    ).toBe("Delete");
  });
});

describe("planeClaimOf and the storage and access", () => {
  const database = {
    section: "management-database",
    registration: "management-database-server",
    parameters: { preset: "small" },
  };
  const access = { section: "access", registration: "gateway" };
  const urls = {
    keycloak: "https://m.example.com/auth",
    identity: "https://m.example.com/identity",
    console: "https://m.example.com/console",
  };

  it("writes the selections of the draft", () => {
    expect(
      planeClaimOf(
        {
          ...ready(),
          storage: { [database.section]: [database], access: [access] },
        },
        ctx,
      ).spec.plugins,
    ).toEqual([database, access]);
    expect(planeClaimOf(ready(), ctx).spec).not.toHaveProperty("plugins");
  });

  it("writes no external URL with an access selection", () => {
    const management = planeClaimOf(
      {
        ...ready(),
        console: true,
        storage: { db: [database], access: [access] },
        urls,
      },
      ctx,
    ).spec.management;
    expect(management.identityProvider).toEqual({ keycloak: {} });
    expect(management.identity).not.toHaveProperty("externalUrl");
    expect(management.console).toEqual({});
  });

  it("writes each typed URL without one, and keeps them across an access round trip", () => {
    const d: PlaneDraft = {
      ...ready(),
      console: true,
      storage: { db: [database] },
      urls: { ...urls, webModeler: "https://m.example.com/modeler" },
    };
    const management = planeClaimOf(d, ctx).spec.management;
    expect(management.identityProvider).toEqual({
      keycloak: { externalUrl: "https://m.example.com/auth" },
    });
    expect(management.identity.externalUrl).toBe(
      "https://m.example.com/identity",
    );
    expect(management.console).toEqual({
      externalUrl: "https://m.example.com/console",
    });
    expect(management).not.toHaveProperty("webModeler");
    const withAccess: PlaneDraft = {
      ...d,
      storage: { db: [database], access: [access] },
    };
    expect(planeClaimOf(withAccess, ctx).spec.management.console).toEqual({});
    const back: PlaneDraft = {
      ...withAccess,
      storage: { db: [database], access: [] },
    };
    expect(planeClaimOf(back, ctx).spec.management).toEqual(management);
  });

  it("leaves out the database config of an app that does not run", () => {
    const d: PlaneDraft = {
      ...ready(),
      mode: "oidc",
      storage: {
        "management-database": [
          {
            section: "management-database",
            registration: "existing-database-configs",
            parameters: {
              keycloak: "kc-db",
              identity: "id-db",
              webModeler: "wm",
            },
          },
        ],
      },
    };
    expect(planeClaimOf(d, ctx).spec.plugins).toEqual([
      {
        section: "management-database",
        registration: "existing-database-configs",
        parameters: { identity: "id-db" },
      },
    ]);
  });

  it("writes managementAuthConfigName only when it differs from the claim name", () => {
    expect(
      planeClaimOf({ ...ready(), contractName: "ops" }, ctx).spec
        .managementAuthConfigName,
    ).toBe("ops");
    expect(
      planeClaimOf({ ...ready(), contractName: "plane-a" }, ctx).spec,
    ).not.toHaveProperty("managementAuthConfigName");
    expect(planeClaimOf(ready(), ctx).spec).not.toHaveProperty(
      "managementAuthConfigName",
    );
  });
});

describe("planeCreateChange", () => {
  it("planeCreateChange puts a new namespace with its labels first and lists both in create", () => {
    const labels = { "fleet.camunda.io/environment": "open", tier: "gold" };
    const d: PlaneDraft = {
      ...ready(),
      environment: "open",
      namespace: { kind: "new", name: "team-new", labels },
    };
    const change = planeCreateChange(d, ctx);
    expect(change.intent).toBe(
      "Create the management plane plane-a in team-new",
    );
    expect(change.apply).toEqual([
      {
        apiVersion: "v1",
        kind: "Namespace",
        metadata: { name: "team-new", labels },
      },
      planeClaimOf(d, ctx),
    ]);
    expect(change.create).toEqual([
      "Namespace//team-new",
      "CamundaManagementClusterClaim/team-new/plane-a",
    ]);
  });

  it("planeCreateChange gives a new namespace the current fixed labels of the Environment", () => {
    const d: PlaneDraft = {
      ...ready(),
      environment: "open",
      namespace: {
        kind: "new",
        name: "team-new",
        labels: {
          "fleet.camunda.io/environment": "open",
          tier: "silver",
          team: "a",
        },
      },
    };
    expect(planeCreateChange(d, ctx).apply[0]).toEqual({
      apiVersion: "v1",
      kind: "Namespace",
      metadata: {
        name: "team-new",
        labels: {
          "fleet.camunda.io/environment": "open",
          tier: "gold",
          team: "a",
        },
      },
    });
  });

  it("planeCreateChange in an existing namespace holds only the claim", () => {
    const change = planeCreateChange(ready(), ctx);
    expect(change.apply).toEqual([planeClaimOf(ready(), ctx)]);
    expect(change.create).toHaveLength(1);
  });
});
