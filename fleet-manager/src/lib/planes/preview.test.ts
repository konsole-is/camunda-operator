import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import { ENVIRONMENT_LABEL, MANAGEMENT_CLUSTER_ANNOTATION } from "@/lib/labels";
import { normalizeIssuer, servedPreview } from "./preview";

const ns = (name: string, env?: string): V1Namespace => ({
  metadata: { name, ...(env && { labels: { [ENVIRONMENT_LABEL]: env } }) },
});

const cluster = (
  namespace: string,
  name: string,
  {
    labels,
    holder,
    gateway = true,
    config = "basic",
  }: {
    labels?: Record<string, string>;
    holder?: string;
    gateway?: boolean;
    config?: string;
  } = {},
): CamundaCluster =>
  ({
    metadata: {
      namespace,
      name,
      ...(labels && { labels }),
      ...(holder && {
        annotations: { [MANAGEMENT_CLUSTER_ANNOTATION]: holder },
      }),
    },
    spec: config === "" ? {} : { platformConfigRef: config },
    ...(gateway && {
      status: {
        gateway: {
          grpcEndpoint: `${name}-gateway:26500`,
          restEndpoint: `http://${name}-gateway:8080`,
        },
      },
    }),
  }) as unknown as CamundaCluster;

const basic = (name: string): CamundaPlatformConfig =>
  ({ metadata: { name }, spec: {} }) as CamundaPlatformConfig;

const oidc = (name: string, issuerUrl?: string): CamundaPlatformConfig =>
  ({
    metadata: { name },
    spec: { auth: { method: "oidc", oidc: { issuerUrl } } },
  }) as unknown as CamundaPlatformConfig;

const ISSUER = "https://idp.example.com/realms/x";

type Input = Parameters<typeof servedPreview>[0];

const base: Input = {
  plane: { namespace: "platform", name: "plane-a" },
  environments: ["production"],
  clusterSelector: {},
  issuer: { kind: "known", url: ISSUER },
  clusters: [cluster("team-a", "c1")],
  namespaces: [
    ns("team-a", "production"),
    ns("team-b", "staging"),
    ns("loose"),
  ],
  platformConfigs: [
    basic("basic"),
    oidc("oidc-same", "HTTPS://IDP.example.com/realms/x/"),
    oidc("oidc-other", "https://other.example.com/realms/x"),
    oidc("oidc-none"),
  ],
};

const rows = (input: Partial<Input> = {}) =>
  servedPreview({ ...base, ...input });

const rowFor = (opts: Parameters<typeof cluster>[2]) =>
  rows({ clusters: [cluster("team-a", "c1", opts)] })[0];

describe("servedPreview", () => {
  it("lists only clusters in namespaces of the picked environments", () => {
    const out = rows({
      clusters: [
        cluster("team-a", "c1"),
        cluster("team-b", "c2"),
        cluster("loose", "c3"),
      ],
    });
    expect(out.map((r) => `${r.namespace}/${r.name}`)).toEqual(["team-a/c1"]);
  });

  it("serves the namespaces of each picked environment", () => {
    const out = rows({
      environments: ["production", "staging"],
      clusters: [cluster("team-b", "c2"), cluster("team-a", "c1")],
    });
    expect(out.map((r) => `${r.namespace}/${r.name}`)).toEqual([
      "team-a/c1",
      "team-b/c2",
    ]);
  });

  it("narrows by the cluster selector", () => {
    const out = rows({
      clusterSelector: { matchLabels: { tier: "gold" } },
      clusters: [
        cluster("team-a", "c1", { labels: { tier: "gold" } }),
        cluster("team-a", "c2"),
      ],
    });
    expect(out.map((r) => r.name)).toEqual(["c1"]);
  });

  it("an empty selector keeps every cluster of the environments", () => {
    const out = rows({
      clusters: [
        cluster("team-a", "c2"),
        cluster("team-a", "c1", { labels: { tier: "gold" } }),
      ],
    });
    expect(out.map((r) => r.name)).toEqual(["c1", "c2"]);
  });

  it("predicts Attach for a ready basic-auth cluster", () => {
    expect(rows()[0]).toEqual({
      namespace: "team-a",
      name: "c1",
      result: "Attach",
      label: "Will attach",
    });
  });

  it("predicts ClaimedElsewhere when another plane holds the cluster", () => {
    expect(rowFor({ holder: "other/plane-b" })).toMatchObject({
      result: "ClaimedElsewhere",
      label: "Served by other/plane-b",
      message: "Another plane serves this cluster.",
    });
  });

  it("does not count its own claim as elsewhere", () => {
    expect(rowFor({ holder: "platform/plane-a" }).result).toBe("Attach");
  });

  it("predicts NotReady while the cluster has no gateway status", () => {
    expect(rowFor({ gateway: false })).toMatchObject({
      result: "NotReady",
      label: "Attaches when it runs",
      message: "The cluster publishes no gateway endpoints yet.",
    });
  });

  it("checks the holder before the gateway, as the operator does", () => {
    expect(rowFor({ holder: "other/plane-b", gateway: false }).result).toBe(
      "ClaimedElsewhere",
    );
  });

  it("checks the gateway before the platform config, as the operator does", () => {
    expect(rowFor({ gateway: false, config: "" }).result).toBe("NotReady");
  });

  it("predicts InvalidReference for a cluster without a readable platform config", () => {
    expect(rowFor({ config: "" })).toMatchObject({
      result: "InvalidReference",
      label: "No platform config",
      message: "This cluster names no platform config.",
    });
    expect(rowFor({ config: "gone" })).toMatchObject({
      result: "InvalidReference",
      label: "No platform config",
      message: "Platform config gone of this cluster does not exist.",
    });
  });

  it("predicts InvalidReference for an oidc cluster without an issuer", () => {
    expect(rowFor({ config: "oidc-none" })).toMatchObject({
      result: "InvalidReference",
      label: "Different issuer",
      message: "Platform config oidc-none of this cluster sets no OIDC issuer.",
    });
  });

  it("predicts InvalidReference for an oidc cluster of another issuer", () => {
    expect(rowFor({ config: "oidc-other" })).toMatchObject({
      result: "InvalidReference",
      label: "Different issuer",
      message: `This cluster trusts https://other.example.com/realms/x. This plane hands out tokens of ${ISSUER}.`,
    });
  });

  it("attaches an oidc cluster of the same issuer in another case and with a trailing slash", () => {
    expect(rowFor({ config: "oidc-same" }).result).toBe("Attach");
  });

  it("refuses an oidc cluster whose issuer differs only in an explicit default port", () => {
    expect(
      rows({
        platformConfigs: [
          oidc("oidc-port", "https://idp.example.com:443/realms/x"),
        ],
        clusters: [cluster("team-a", "c1", { config: "oidc-port" })],
      })[0].result,
    ).toBe("InvalidReference");
  });

  it("in Keycloak mode refuses every oidc cluster with the new-issuer message, and never a basic one", () => {
    const out = rows({
      issuer: { kind: "new" },
      clusters: [
        cluster("team-a", "oidc", { config: "oidc-same" }),
        cluster("team-a", "basic", { config: "basic" }),
      ],
    });
    expect(out.find((r) => r.name === "oidc")).toMatchObject({
      result: "InvalidReference",
      label: "Different issuer",
      message:
        "This cluster trusts another issuer. A Keycloak that this plane runs is a new issuer.",
    });
    expect(out.find((r) => r.name === "basic")?.result).toBe("Attach");
  });
});

describe("normalizeIssuer", () => {
  it.each([
    ["HTTPS://IdP.Example.com/realms/x/", "https://idp.example.com/realms/x"],
    ["https://idp.example.com/Realms/X", "https://idp.example.com/Realms/X"],
    ["https://idp.example.com:443/x", "https://idp.example.com:443/x"],
    ["https://idp.example.com", "https://idp.example.com"],
    ["https://idp.example.com//", "https://idp.example.com"],
    ["https://idp.example.com/x/?a=B#F", "https://idp.example.com/x?a=B#F"],
    ["idp.example.com/x/", "idp.example.com/x"],
  ])("%s is %s", (input, want) => {
    expect(normalizeIssuer(input)).toBe(want);
  });

  // Each expected value is the output of the Go function for the input.
  it.each([
    ["https://idp.example.com/a%2Fb/", "https://idp.example.com/a/b"],
    ["https://idp.example.com/a%2Fb", "https://idp.example.com/a%2Fb"],
    ["https://idp.example.com/café", "https://idp.example.com/caf%C3%A9"],
    ["https://idp.example.com/caf%C3%A9", "https://idp.example.com/caf%C3%A9"],
    ["https://idp.example.com/caf%c3%a9/", "https://idp.example.com/caf%C3%A9"],
    ["https://idp.example.com/a%20b/", "https://idp.example.com/a%20b"],
    ["https://idp.example.com/a b", "https://idp.example.com/a%20b"],
    ["https://idp.example.com/(x)!*'", "https://idp.example.com/(x)!*'"],
    ["https://idp.example.com/a%zz", "https://idp.example.com/a%zz"],
    ["https://idp.example.com/%7Euser/", "https://idp.example.com/~user"],
    ["https://idp.example.com/a%3Bb", "https://idp.example.com/a%3Bb"],
    ["not a url/", "not%20a%20url"],
  ])("escapes the path of %s as Go does: %s", (input, want) => {
    expect(normalizeIssuer(input)).toBe(want);
  });

  it("matches an escaped issuer to its plain form, as Go does", () => {
    expect(normalizeIssuer("https://idp.example.com/a%2Fb/")).toBe(
      normalizeIssuer("https://idp.example.com/a/b"),
    );
    expect(normalizeIssuer("https://idp.example.com/café/")).toBe(
      normalizeIssuer("https://idp.example.com/caf%C3%A9"),
    );
  });
});
