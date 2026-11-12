import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it } from "vitest";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import type { EnvironmentDraft } from "./draft";
import { environmentImpact, namespaceImpact, withoutKept } from "./impact";
import type { EnvironmentSources } from "./sources";

type Recorded = NonNullable<
  NonNullable<CamundaClusterClaim["status"]>["resolvedPolicy"]
>;

const DELETE: Recorded = { deletionPolicy: "Delete" };

function environment(spec: Environment["spec"]): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "staging", resourceVersion: "3" },
    spec,
  };
}

function namespace(name: string, env?: string): V1Namespace {
  return {
    metadata: {
      name,
      ...(env && { labels: { [ENVIRONMENT_LABEL]: env } }),
    },
  };
}

function clusterClaim(
  name: string,
  spec: Partial<CamundaClusterClaim["spec"]> = {},
  resolvedPolicy?: Recorded,
  ns = "team-a",
): CamundaClusterClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: ns, name },
    spec: { presetRef: "small", ...spec },
    ...(resolvedPolicy && { status: { resolvedPolicy } }),
  };
}

function planeClaim(
  name: string,
  spec: Partial<CamundaManagementClusterClaim["spec"]> = {},
  resolvedPolicy?: Recorded,
): CamundaManagementClusterClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: "team-a", name },
    spec: { environments: ["staging"], ...spec },
    ...(resolvedPolicy && { status: { resolvedPolicy } }),
  } as CamundaManagementClusterClaim;
}

function sources(s: Partial<EnvironmentSources>): EnvironmentSources {
  return {
    environments: [],
    namespaces: [namespace("team-a", "staging"), namespace("team-b", "other")],
    clusterClaims: [],
    planeClaims: [],
    clusters: [],
    planes: [],
    sections: [],
    registrations: [],
    requests: [],
    clusterPresets: [],
    elasticsearchPresets: [],
    databasePresets: [],
    releases: [],
    platformConfigs: [],
    ...s,
  };
}

function edit(spec: Environment["spec"]): EnvironmentDraft {
  return { name: "staging", spec };
}

describe("environmentImpact", () => {
  it("counts the claims whose recorded policy differs from the edit", () => {
    const live = environment({ deletionPolicy: "Delete" });
    expect(
      environmentImpact(
        live,
        edit({ deletionPolicy: "Orphan" }),
        sources({
          clusterClaims: [
            clusterClaim("orders", { releaseRef: "8.9.1" }, DELETE),
            clusterClaim("billing", { releaseRef: "8.9.1" }, DELETE),
            clusterClaim("other", { releaseRef: "8.9.1" }, DELETE, "team-b"),
          ],
        }),
      ),
    ).toEqual({ clusters: 2, planes: 0, warnings: [] });
  });

  it("counts a plane claim as a plane", () => {
    expect(
      environmentImpact(
        environment({}),
        edit({ deletionPolicy: "Orphan" }),
        sources({ planeClaims: [planeClaim("plane", {}, DELETE)] }),
      ),
    ).toMatchObject({ clusters: 0, planes: 1 });
  });

  it("does not count a claim whose recorded policy equals the edit, nor a claim without a recorded policy", () => {
    expect(
      environmentImpact(
        environment({ deletionPolicy: "Delete", displayName: "Staging" }),
        edit({ deletionPolicy: "Delete", displayName: "Stage" }),
        sources({
          clusterClaims: [
            clusterClaim("orders", { releaseRef: "8.9.1" }, DELETE),
            clusterClaim("fresh", { releaseRef: "8.9.1" }),
          ],
        }),
      ),
    ).toEqual({ clusters: 0, planes: 0, warnings: [] });
  });

  it("lists a claim whose release the edit removes, with removed true", () => {
    expect(
      environmentImpact(
        environment({ releases: { allowed: ["8.9.1", "8.9.2"] } }),
        edit({ releases: { allowed: ["8.9.2"] } }),
        sources({
          clusterClaims: [clusterClaim("orders", { releaseRef: "8.9.1" })],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaRelease",
        name: "8.9.1",
        removed: true,
      },
    ]);
  });

  it("lists a claim whose CamundaClusterPreset the edit removes", () => {
    expect(
      environmentImpact(
        environment({
          presets: { camundaCluster: ["small", "large"] },
          releases: { allowed: ["8.9.1"] },
        }),
        edit({
          presets: { camundaCluster: ["large"] },
          releases: { allowed: ["8.9.1"] },
        }),
        sources({
          clusterClaims: [
            clusterClaim("orders", { presetRef: "small", releaseRef: "8.9.1" }),
          ],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaClusterPreset",
        name: "small",
        removed: true,
      },
    ]);
  });

  it("does not list a claim whose release the live environment already did not allow", () => {
    expect(
      environmentImpact(
        environment({ releases: { allowed: ["8.9.2"] } }),
        edit({ releases: { allowed: ["8.9.3"] } }),
        sources({
          clusterClaims: [clusterClaim("orders", { releaseRef: "8.9.1" })],
        }),
      ).warnings,
    ).toEqual([]);
  });

  it("lists a claim without a releaseRef when the default release changes", () => {
    expect(
      environmentImpact(
        environment({
          releases: { allowed: ["8.9.1", "8.9.2"], default: "8.9.1" },
        }),
        edit({ releases: { allowed: ["8.9.1", "8.9.2"], default: "8.9.2" } }),
        sources({ clusterClaims: [clusterClaim("orders")] }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaRelease",
        name: "8.9.2",
        followsDefault: true,
      },
    ]);
  });

  it.each([
    ["8.9.0", "VersionDowngradeRefused"],
    ["8.11.0", "VersionSkipRefused"],
  ])(
    "marks a new default release %s that the cluster refuses as %s, by the old default without a reported version",
    (version, refused) => {
      const live = environment({
        releases: { allowed: ["old", "new"], default: "old" },
      });
      const releases = [
        { metadata: { name: "old" }, spec: { version: "8.9.1" } },
        { metadata: { name: "new" }, spec: { version } },
      ] as unknown as EnvironmentSources["releases"];
      expect(
        environmentImpact(
          live,
          edit({ releases: { allowed: ["old", "new"], default: "new" } }),
          sources({ clusterClaims: [clusterClaim("orders")], releases }),
        ).warnings,
      ).toEqual([
        {
          claim: {
            kind: "CamundaClusterClaim",
            namespace: "team-a",
            name: "orders",
          },
          kind: "CamundaRelease",
          name: "new",
          followsDefault: true,
          refused,
        },
      ]);
    },
  );

  it("marks no refusal for a downgrade that the claim or the cluster approves", () => {
    const live = environment({
      releases: { allowed: ["old", "new"], default: "old" },
    });
    const releases = [
      { metadata: { name: "old" }, spec: { version: "8.9.1" } },
      { metadata: { name: "new" }, spec: { version: "8.9.0" } },
    ] as unknown as EnvironmentSources["releases"];
    const cluster = (name: string, annotations?: Record<string, string>) => ({
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: {
        namespace: "team-a",
        name,
        labels: { "fleet.camunda.io/claim": name },
        ...(annotations && { annotations }),
      },
      spec: {},
      status: { management: { version: "8.9.1" } },
    });
    const clusters = [
      cluster("orders"),
      cluster("billing", { "camunda.io/allow-version-downgrade": "8.9.0" }),
      cluster("search"),
    ] as unknown as EnvironmentSources["clusters"];
    const warnings = environmentImpact(
      live,
      edit({ releases: { allowed: ["old", "new"], default: "new" } }),
      sources({
        clusterClaims: [
          clusterClaim("orders", { allowVersionDowngrade: "8.9.0" }),
          clusterClaim("billing"),
          clusterClaim("search"),
        ],
        releases,
        clusters,
      }),
    ).warnings;
    expect(
      warnings.map((w) => [
        w.claim.name,
        "refused" in w ? w.refused : undefined,
      ]),
    ).toEqual([
      ["orders", undefined],
      ["billing", undefined],
      ["search", "VersionDowngradeRefused"],
    ]);
  });

  it("checks a new default release against the version that the cluster reports", () => {
    const live = environment({
      releases: { allowed: ["old", "new"], default: "old" },
    });
    const releases = [
      { metadata: { name: "old" }, spec: { version: "8.10.0" } },
      { metadata: { name: "new" }, spec: { version: "8.11.0" } },
    ] as unknown as EnvironmentSources["releases"];
    const clusters = [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaCluster",
        metadata: {
          namespace: "team-a",
          name: "orders",
          labels: { "fleet.camunda.io/claim": "orders" },
        },
        spec: {},
        status: { management: { version: "8.9.0" } },
      },
    ] as unknown as EnvironmentSources["clusters"];
    expect(
      environmentImpact(
        live,
        edit({ releases: { allowed: ["old", "new"], default: "new" } }),
        sources({
          clusterClaims: [clusterClaim("orders")],
          releases,
          clusters,
        }),
      ).warnings,
    ).toMatchObject([{ name: "new", refused: "VersionSkipRefused" }]);
  });

  it("names the namespaces whose claims a first environment leaves without one", () => {
    expect(
      environmentImpact(
        undefined,
        edit({}),
        sources({
          environments: [],
          namespaces: [namespace("team-a", "staging"), namespace("legacy")],
          clusterClaims: [
            clusterClaim("orders"),
            clusterClaim("old", {}, undefined, "legacy"),
          ],
        }),
      ).unassigned,
    ).toEqual(["legacy"]);
  });

  it("leaves out a namespace whose label names another environment", () => {
    expect(
      environmentImpact(
        undefined,
        edit({}),
        sources({
          environments: [],
          namespaces: [namespace("future-ns", "future"), namespace("legacy")],
          clusterClaims: [
            clusterClaim("next", {}, undefined, "future-ns"),
            clusterClaim("old", {}, undefined, "legacy"),
          ],
        }),
      ).unassigned,
    ).toEqual(["legacy"]);
  });

  it("names no namespace when environments exist already", () => {
    expect(
      environmentImpact(
        undefined,
        edit({}),
        sources({
          environments: [environment({})],
          namespaces: [namespace("legacy")],
          clusterClaims: [clusterClaim("old", {}, undefined, "legacy")],
        }),
      ).unassigned,
    ).toBeUndefined();
  });

  it("lists a claim without a platformConfigRef when the default platform config changes", () => {
    expect(
      environmentImpact(
        environment({ platformConfigRef: "shared" }),
        edit({ platformConfigRef: "strict" }),
        sources({
          clusterClaims: [
            clusterClaim("orders"),
            clusterClaim("pinned", { platformConfigRef: "own" }),
          ],
          planeClaims: [planeClaim("plane")],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaPlatformConfig",
        name: "strict",
        followsDefault: true,
      },
      {
        claim: {
          kind: "CamundaManagementClusterClaim",
          namespace: "team-a",
          name: "plane",
        },
        kind: "CamundaPlatformConfig",
        name: "strict",
        followsDefault: true,
      },
    ]);
  });

  it("names the claims of the namespaces that already carry the label of a new environment", () => {
    expect(
      environmentImpact(
        undefined,
        edit({ releases: { allowed: ["8.9.2"], default: "8.9.2" } }),
        sources({
          clusterClaims: [
            clusterClaim("orders"),
            clusterClaim("pinned", { releaseRef: "8.9.1" }),
          ],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaRelease",
        name: "8.9.2",
        followsDefault: true,
      },
    ]);
  });

  it("has no claims and no warnings for a new environment beside others", () => {
    expect(
      environmentImpact(
        undefined,
        edit({ releases: { allowed: ["8.9.2"], default: "8.9.2" } }),
        sources({
          environments: [environment({})],
          namespaces: [],
          clusterClaims: [clusterClaim("orders", {}, DELETE)],
        }),
      ),
    ).toEqual({ clusters: 0, planes: 0, warnings: [] });
  });

  it("lists a claim without a releaseRef when the edit clears the default release", () => {
    expect(
      environmentImpact(
        environment({ releases: { allowed: ["8.9.1"], default: "8.9.1" } }),
        edit({ releases: { allowed: ["8.9.1"] } }),
        sources({ clusterClaims: [clusterClaim("orders")] }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaRelease",
        cleared: true,
      },
    ]);
  });

  it("lists each claim without a platformConfigRef when the edit clears the default platform config", () => {
    expect(
      environmentImpact(
        environment({ platformConfigRef: "shared" }),
        edit({}),
        sources({
          clusterClaims: [clusterClaim("orders")],
          planeClaims: [planeClaim("plane")],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaPlatformConfig",
        cleared: true,
      },
      {
        claim: {
          kind: "CamundaManagementClusterClaim",
          namespace: "team-a",
          name: "plane",
        },
        kind: "CamundaPlatformConfig",
        cleared: true,
      },
    ]);
  });
});

describe("namespaceImpact", () => {
  const target = environment({
    releases: { allowed: ["8.9.2"], default: "8.9.2" },
    platformConfigRef: "shared",
  });

  it("names each claim of the namespace that takes a default of the environment", () => {
    expect(
      namespaceImpact(
        "team-b",
        target,
        sources({
          clusterClaims: [
            clusterClaim("orders", {}, undefined, "team-b"),
            clusterClaim(
              "pinned",
              { releaseRef: "8.9.1", platformConfigRef: "own" },
              undefined,
              "team-b",
            ),
            clusterClaim("elsewhere", {}, undefined, "team-a"),
          ],
        }),
      ),
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-b",
          name: "orders",
        },
        kind: "CamundaRelease",
        name: "8.9.2",
        followsDefault: true,
      },
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-b",
          name: "orders",
        },
        kind: "CamundaPlatformConfig",
        name: "shared",
        followsDefault: true,
      },
    ]);
  });

  it("names nothing for an environment without defaults", () => {
    expect(
      namespaceImpact(
        "team-b",
        environment({}),
        sources({
          clusterClaims: [clusterClaim("orders", {}, undefined, "team-b")],
        }),
      ),
    ).toEqual([]);
  });
});

describe("environmentImpact for the presets of the built-in storage", () => {
  const STORAGE = {
    section: "secondary-storage",
    registration: "elasticsearch",
  };
  const DATABASE = {
    section: "management-database",
    registration: "management-database-server",
  };
  const registration = (name: string) =>
    ({
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginRegistration",
      metadata: { name },
      spec: {
        label: name,
        sections: [],
        mode: "Optional",
        deletionPolicies: ["Delete"],
        serviceAccount: { name: "sa", namespace: "ns" },
        controller: { name: "camunda-operator" },
        claimEditable: ["preset"],
      },
    }) as unknown as EnvironmentSources["registrations"][number];
  const registrations = [
    registration("elasticsearch"),
    registration("management-database-server"),
  ];
  const editable = (registrationName: string, preset?: string): Recorded => ({
    deletionPolicy: "Delete",
    parameters: {
      [registrationName]: {
        preset: { claimEditable: true, ...(preset && { value: preset }) },
      },
    },
  });

  it("lists a claim whose Elasticsearch preset the edit removes", () => {
    expect(
      environmentImpact(
        environment({
          presets: { elasticsearchCluster: ["es-small", "es-large"] },
        }),
        edit({ presets: { elasticsearchCluster: ["es-large"] } }),
        sources({
          registrations,
          clusterClaims: [
            clusterClaim(
              "orders",
              {
                presetRef: undefined,
                plugins: [{ ...STORAGE, parameters: { preset: "es-small" } }],
              },
              editable("elasticsearch"),
            ),
          ],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "ElasticsearchClusterPreset",
        name: "es-small",
        removed: true,
      },
    ]);
  });

  it("lists a plane claim whose PostgreSQL preset the edit removes", () => {
    expect(
      environmentImpact(
        environment({ presets: { databaseServer: ["pg-small"] } }),
        edit({}),
        sources({
          registrations,
          planeClaims: [
            planeClaim(
              "plane-a",
              {
                plugins: [{ ...DATABASE, parameters: { preset: "pg-small" } }],
              },
              editable("management-database-server"),
            ),
          ],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaManagementClusterClaim",
          namespace: "team-a",
          name: "plane-a",
        },
        kind: "DatabaseServerPreset",
        name: "pg-small",
        removed: true,
      },
    ]);
  });

  it("reads the preset of the recorded policy when the claim sets none", () => {
    expect(
      environmentImpact(
        environment({ presets: { elasticsearchCluster: ["es-small"] } }),
        edit({}),
        sources({
          registrations,
          clusterClaims: [
            clusterClaim(
              "orders",
              { presetRef: undefined, plugins: [STORAGE] },
              editable("elasticsearch", "es-small"),
            ),
          ],
        }),
      ).warnings.map((w) => ("removed" in w ? w.name : undefined)),
    ).toEqual(["es-small"]);
  });

  it("does not list a preset that the live environment already did not allow", () => {
    expect(
      environmentImpact(
        environment({ presets: { elasticsearchCluster: ["es-large"] } }),
        edit({}),
        sources({
          registrations,
          clusterClaims: [
            clusterClaim(
              "orders",
              {
                presetRef: undefined,
                plugins: [{ ...STORAGE, parameters: { preset: "es-small" } }],
              },
              editable("elasticsearch"),
            ),
          ],
        }),
      ).warnings,
    ).toEqual([]);
  });
});

describe("withoutKept", () => {
  const orders = {
    kind: "CamundaClusterClaim" as const,
    namespace: "team-a",
    name: "orders",
  };
  const impact = {
    clusters: 1,
    planes: 0,
    warnings: [
      {
        claim: orders,
        kind: "CamundaRelease" as const,
        name: "8.9.1",
        removed: true as const,
      },
      {
        claim: orders,
        kind: "CamundaClusterPreset" as const,
        name: "small",
        removed: true as const,
      },
    ],
  };

  it("drops a removed warning whose name the saved spec still allows", () => {
    expect(
      withoutKept(impact, {
        releases: { allowed: ["8.9.1", "8.9.2"] },
        presets: { camundaCluster: ["large"] },
      }).warnings,
    ).toEqual([impact.warnings[1]]);
  });

  it("drops a cleared warning whose default the saved spec still has", () => {
    const cleared = {
      clusters: 0,
      planes: 0,
      warnings: [
        {
          claim: orders,
          kind: "CamundaRelease" as const,
          cleared: true as const,
        },
        {
          claim: orders,
          kind: "CamundaPlatformConfig" as const,
          cleared: true as const,
        },
      ],
    };
    expect(
      withoutKept(cleared, { releases: { allowed: ["r"], default: "r" } })
        .warnings,
    ).toEqual([cleared.warnings[1]]);
  });

  it("keeps every warning without a saved spec", () => {
    expect(withoutKept(impact, undefined)).toEqual(impact);
  });
});
