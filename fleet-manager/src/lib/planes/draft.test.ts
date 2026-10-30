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

const environment = (name: string, platformConfigRef?: string): Environment =>
  ({
    metadata: { name },
    spec: platformConfigRef ? { platformConfigRef } : {},
  }) as Environment;

const ctx: PlaneContext = {
  environments: [
    environment("production", "default-platform"),
    environment("staging", "default-platform"),
    environment("bare"),
    environment("dangling", "gone"),
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
    expect(basicsErrors({ ...ready(), environment: "bare" }, ctx)).toEqual([
      "Pick a platform config.",
    ]);
  });

  it("blocks step 1 when the platform config of the environment does not exist", () => {
    expect(basicsErrors({ ...ready(), environment: "dangling" }, ctx)).toEqual([
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
