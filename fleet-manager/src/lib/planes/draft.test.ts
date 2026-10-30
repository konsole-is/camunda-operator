import { describe, expect, it } from "vitest";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { Environment } from "@/generated/crds/environment";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
  basicsErrors,
  planeClaimOf,
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

const ctx: PlaneContext = {
  environments: [
    environment("production", "default-platform"),
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
};

const ready = (): PlaneDraft => ({
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "keycloak",
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
