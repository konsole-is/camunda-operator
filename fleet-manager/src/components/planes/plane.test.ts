import { describe, expect, it } from "vitest";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  formatSelector,
  optimizeLabel,
  planesIn,
  planesOf,
  servesEnvironment,
} from "./plane";

function cluster(
  namespace: string,
  name: string,
  {
    labels,
    spec = {},
    status,
  }: {
    labels?: Record<string, string>;
    spec?: Record<string, unknown>;
    status?: Record<string, unknown>;
  } = {},
) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace, name, labels },
    spec: { identityProvider: { keycloak: {} }, clusterSelector: {}, ...spec },
    status,
  } as never;
}

function claim(
  namespace: string,
  name: string,
  environments: string[],
  management: Record<string, unknown> = { identityProvider: { keycloak: {} } },
) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace, name },
    spec: { environments, management },
  } as never;
}

function environment(name: string) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: {},
  } as never;
}

function ns(name: string, labels: Record<string, string>) {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name, labels },
  } as never;
}

describe("planesOf", () => {
  it("reads every column of a managed plane", () => {
    const [plane] = planesOf({
      clusters: [
        cluster("platform", "plane-a", {
          labels: { [CLAIM_LABEL]: "plane-a" },
          spec: {
            identityProvider: { keycloak: {} },
            console: {},
            webModeler: {},
          },
          status: {
            conditions: [
              {
                type: "Ready",
                status: "False",
                reason: "UnsupportedVersion",
                message: "Keycloak 27.0.0 is not supported",
              },
            ],
            clusters: [
              { namespace: "team-a", name: "orders", attached: true },
              {
                namespace: "team-b",
                name: "billing",
                attached: false,
                reason: "ClaimedElsewhere",
                message: "plane-b holds it",
              },
            ],
            optimize: [
              {
                namespace: "team-a",
                name: "opt",
                externalUrl: "https://opt.example.com",
              },
            ],
            managementAuthConfig: "plane-a",
          },
        }),
      ],
      claims: [claim("platform", "plane-a", ["production", "staging"])],
      namespaces: [],
      environments: [],
    });
    expect(plane).toEqual({
      id: "cluster/platform/plane-a",
      namespace: "platform",
      name: "plane-a",
      managed: true,
      mode: "Keycloak",
      components: ["Identity", "Console", "Web Modeler"],
      serves: { environments: ["production", "staging"] },
      servedEnvironments: ["production", "staging"],
      servesNoEnvironment: false,
      attached: 1,
      notAttached: [
        {
          namespace: "team-b",
          name: "billing",
          reason: "ClaimedElsewhere",
          message: "plane-b holds it",
        },
      ],
      optimize: 1,
      contract: "plane-a",
      ready: expect.objectContaining({
        status: "False",
        reason: "UnsupportedVersion",
      }),
    });
  });

  it.each([
    [{ keycloak: {} }, "Keycloak"],
    [
      {
        externalKeycloak: {
          url: "https://kc",
          realm: "r",
          adminCredentialsSecretRef: { name: "s" },
        },
      },
      "External Keycloak",
    ],
    [{ oidc: {} }, "OIDC"],
  ])("names the identity mode %j as %s", (identityProvider, mode) => {
    const [plane] = planesOf({
      clusters: [
        cluster("platform", "plane-a", { spec: { identityProvider } }),
      ],
      claims: [],
      namespaces: [],
      environments: [],
    });
    expect(plane.mode).toBe(mode);
  });

  it("has no mode when no identity provider is set", () => {
    const [plane] = planesOf({
      clusters: [
        cluster("platform", "plane-a", { spec: { identityProvider: {} } }),
      ],
      claims: [],
      namespaces: [],
      environments: [],
    });
    expect(plane.mode).toBeUndefined();
  });

  it("lists Identity alone when Console and Web Modeler are unset", () => {
    const [plane] = planesOf({
      clusters: [cluster("platform", "plane-a")],
      claims: [],
      namespaces: [],
      environments: [],
    });
    expect(plane.components).toEqual(["Identity"]);
  });

  it("shows a claim without its management cluster as one row with no status", () => {
    const planes = planesOf({
      clusters: [],
      claims: [
        claim("platform", "plane-a", ["production"], {
          identityProvider: { oidc: {} },
          console: {},
        }),
      ],
      namespaces: [],
      environments: [],
    });
    expect(planes).toEqual([
      expect.objectContaining({
        namespace: "platform",
        name: "plane-a",
        managed: true,
        mode: "OIDC",
        components: ["Identity", "Console"],
        serves: { environments: ["production"] },
        servedEnvironments: ["production"],
        attached: 0,
        notAttached: [],
        optimize: 0,
        ready: undefined,
      }),
    ]);
  });

  it("joins a claim to the management cluster of its own namespace only", () => {
    const planes = planesOf({
      clusters: [
        cluster("platform", "plane-a", {
          labels: { [CLAIM_LABEL]: "plane-a" },
        }),
        cluster("sandbox", "plane-a", {
          labels: { [CLAIM_LABEL]: "plane-a" },
        }),
      ],
      claims: [claim("platform", "plane-a", ["production"])],
      namespaces: [],
      environments: [],
    });
    expect(planes.map((p) => [p.namespace, p.name, p.serves])).toEqual([
      ["platform", "plane-a", { environments: ["production"] }],
      ["sandbox", "plane-a", { selector: "Every namespace" }],
    ]);
  });

  it("gives an unmanaged cluster and a claim of the same name two ids", () => {
    const planes = planesOf({
      clusters: [cluster("platform", "plane-a")],
      claims: [claim("platform", "plane-a", ["production"])],
      namespaces: [],
      environments: [],
    });
    expect(planes).toHaveLength(2);
    expect(new Set(planes.map((p) => p.id)).size).toBe(2);
  });

  it("keeps a labelled cluster managed when its claim is missing", () => {
    const [plane] = planesOf({
      clusters: [
        cluster("platform", "plane-a", {
          labels: { [CLAIM_LABEL]: "plane-a" },
        }),
      ],
      claims: [],
      namespaces: [],
      environments: [],
    });
    expect(plane.managed).toBe(true);
    expect(plane.serves).toEqual({ selector: "Every namespace" });
  });

  it("shows an unmanaged plane with its raw selector and the environments of the namespaces it selects", () => {
    const [plane] = planesOf({
      clusters: [
        cluster("platform", "legacy", {
          spec: {
            identityProvider: { keycloak: {} },
            namespaceSelector: {
              matchExpressions: [
                {
                  key: ENVIRONMENT_LABEL,
                  operator: "In",
                  values: ["production"],
                },
              ],
            },
          },
        }),
      ],
      claims: [],
      namespaces: [
        ns("team-a", { [ENVIRONMENT_LABEL]: "production" }),
        ns("team-b", { [ENVIRONMENT_LABEL]: "staging" }),
        ns("tools", {}),
      ],
      environments: [environment("production"), environment("staging")],
    });
    expect(plane.managed).toBe(false);
    expect(plane.serves).toEqual({
      selector: "fleet.camunda.io/environment in (production)",
    });
    expect(plane.servedEnvironments).toEqual(["production"]);
  });

  it("serves every environment that has a namespace when the selector is unset", () => {
    const [plane] = planesOf({
      clusters: [cluster("platform", "legacy")],
      claims: [],
      namespaces: [
        ns("team-b", { [ENVIRONMENT_LABEL]: "staging" }),
        ns("team-a", { [ENVIRONMENT_LABEL]: "production" }),
        ns("team-c", { [ENVIRONMENT_LABEL]: "production" }),
        ns("tools", {}),
      ],
      environments: [environment("production"), environment("staging")],
    });
    expect(plane.serves).toEqual({ selector: "Every namespace" });
    expect(plane.servedEnvironments).toEqual(["production", "staging"]);
  });

  it("sorts the planes by namespace, then name", () => {
    const planes = planesOf({
      clusters: [cluster("b", "x"), cluster("a", "z")],
      claims: [claim("a", "y", ["production"])],
      namespaces: [],
      environments: [],
    });
    expect(planes.map((p) => `${p.namespace}/${p.name}`)).toEqual([
      "a/y",
      "a/z",
      "b/x",
    ]);
  });
});

describe("formatSelector", () => {
  it.each([
    [undefined, "Every namespace"],
    [{}, "Every namespace"],
    [{ matchLabels: {}, matchExpressions: [] }, "Every namespace"],
    [{ matchLabels: { b: "2", a: "1" } }, "a=1, b=2"],
    [
      {
        matchLabels: { team: "x" },
        matchExpressions: [
          { key: "env", operator: "In", values: ["prod", "dev"] },
          { key: "tier", operator: "NotIn", values: ["db"] },
          { key: "owned", operator: "Exists" },
          { key: "legacy", operator: "DoesNotExist" },
        ],
      },
      "team=x, env in (prod,dev), tier notin (db), owned, !legacy",
    ],
  ])("formats %j as %j", (selector, text) => {
    expect(formatSelector(selector)).toBe(text);
  });
});

describe("servesEnvironment and optimizeLabel", () => {
  it("is true only for an environment in servedEnvironments", () => {
    const [plane] = planesOf({
      clusters: [],
      claims: [claim("platform", "plane-a", ["production", "staging"])],
      namespaces: [],
      environments: [],
    });
    expect(servesEnvironment(plane, "production")).toBe(true);
    expect(servesEnvironment(plane, "staging")).toBe(true);
    expect(servesEnvironment(plane, "sandbox")).toBe(false);
  });

  it("reads 0 (OIDC) in OIDC mode and the count otherwise", () => {
    const optimize = [
      { namespace: "a", name: "o1", externalUrl: "https://o1" },
      { namespace: "a", name: "o2", externalUrl: "https://o2" },
    ];
    const [oidc, keycloak] = planesOf({
      clusters: [
        cluster("a", "oidc", {
          spec: { identityProvider: { oidc: {} } },
          status: { optimize: [] },
        }),
        cluster("b", "keycloak", { status: { optimize } }),
      ],
      claims: [],
      namespaces: [],
      environments: [],
    });
    expect(optimizeLabel(oidc)).toBe("0 (OIDC)");
    expect(optimizeLabel(keycloak)).toBe("2");
  });
});

describe("planesIn", () => {
  const planes = planesOf({
    clusters: [
      cluster("platform", "legacy", {
        spec: {
          namespaceSelector: {
            matchExpressions: [
              { key: ENVIRONMENT_LABEL, operator: "DoesNotExist" },
            ],
          },
        },
      }),
    ],
    claims: [
      claim("platform", "plane-a", ["production"]),
      claim("platform", "plane-b", ["staging"]),
    ],
    namespaces: [
      ns("team-a", { [ENVIRONMENT_LABEL]: "production" }),
      ns("team-x", {}),
    ],
    environments: [environment("production"), environment("staging")],
  });
  const names = (list: { name: string }[]) => list.map((p) => p.name);

  it("gives every plane without an environment", () => {
    expect(names(planesIn(planes))).toEqual(["legacy", "plane-a", "plane-b"]);
  });

  it("gives the planes that serve no environment for null", () => {
    expect(names(planesIn(planes, null))).toEqual(["legacy"]);
  });

  it("gives the planes that serve the environment", () => {
    expect(names(planesIn(planes, "staging"))).toEqual(["plane-b"]);
  });

  it("gives a plane that serves an environment and a namespace without one on both tabs", () => {
    const [every] = planesOf({
      clusters: [cluster("platform", "every")],
      claims: [],
      namespaces: [
        ns("team-a", { [ENVIRONMENT_LABEL]: "production" }),
        ns("team-x", {}),
      ],
      environments: [environment("production"), environment("staging")],
    });
    expect(names(planesIn([every], "production"))).toEqual(["every"]);
    expect(names(planesIn([every], null))).toEqual(["every"]);
  });

  it("puts a namespace whose label names no Environment with the namespaces without an environment", () => {
    const [plane] = planesOf({
      clusters: [cluster("platform", "every")],
      claims: [],
      namespaces: [
        ns("team-a", { [ENVIRONMENT_LABEL]: "production" }),
        ns("team-g", { [ENVIRONMENT_LABEL]: "ghost" }),
        ns("team-e", { [ENVIRONMENT_LABEL]: "" }),
      ],
      environments: [environment("production")],
    });
    expect(plane.servedEnvironments).toEqual(["production"]);
    expect(names(planesIn([plane], null))).toEqual(["every"]);
  });

  it("serves no namespace while the cluster selector is unset", () => {
    const [plane] = planesOf({
      clusters: [
        cluster("platform", "idle", {
          spec: {
            identityProvider: { keycloak: {} },
            clusterSelector: undefined,
          },
        }),
      ],
      claims: [],
      namespaces: [
        ns("team-a", { [ENVIRONMENT_LABEL]: "production" }),
        ns("team-x", {}),
      ],
      environments: [environment("production")],
    });
    expect(plane.servedEnvironments).toEqual([]);
    expect(planesIn([plane], null)).toEqual([]);
    expect(planesIn([plane], "production")).toEqual([]);
  });

  it("leaves out a plane that selects no namespace for null", () => {
    const [none] = planesOf({
      clusters: [
        cluster("platform", "none", {
          spec: { namespaceSelector: { matchLabels: { team: "nobody" } } },
        }),
      ],
      claims: [],
      namespaces: [ns("team-x", {})],
      environments: [],
    });
    expect(planesIn([none], null)).toEqual([]);
  });
});
