import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import type { ResolvedPolicy } from "@/lib/policy";
import {
  type PresetUsageSources,
  presetResources,
  presetUsage,
  presetUsers,
} from "./presets";

function cluster(namespace: string, name: string, presetRef?: string) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { namespace, name },
    spec: { ...(presetRef && { presetRef }) },
  } as CamundaCluster;
}

describe("presetUsage", () => {
  it("lists the clusters of each preset by their presetRef, sorted", () => {
    expect(
      presetUsage([
        cluster("team-b", "billing", "small"),
        cluster("team-a", "orders", "small"),
        cluster("team-a", "search"),
      ]),
    ).toEqual(new Map([["small", ["team-a/orders", "team-b/billing"]]]));
  });

  it("gives no entry without clusters", () => {
    expect(presetUsage([])).toEqual(new Map());
  });
});

const named = <S>(namespace: string, name: string, spec: S) => ({
  metadata: { namespace, name },
  spec,
});
const selection = (registration: string, parameters: unknown) => ({
  section: "storage",
  registration,
  parameters,
});

const sources: PresetUsageSources = {
  clusters: [named("team-a", "orders", { presetRef: "medium" })],
  elasticsearchClusters: [
    named("team-a", "orders-es", { presetRef: "standard" }),
  ],
  databaseServers: [named("team-a", "orders-db", { presetRef: "standard" })],
  clusterClaims: [
    named("team-b", "billing", { presetRef: "large" }),
    named("team-a", "orders", {
      presetRef: "medium",
      plugins: [selection("database-server", { preset: "small" })],
    }),
    named("team-d", "twice", {
      presetRef: "medium",
      plugins: [
        selection("elasticsearch", { preset: "twice" }),
        selection("elasticsearch", { preset: "twice" }),
      ],
    }),
  ],
  planeClaims: [
    named("platform", "plane", {
      plugins: [selection("management-database-server", { preset: "small" })],
    }),
  ],
  requests: [
    named("team-c", "search-abc12", {
      registration: "elasticsearch",
      parameters: { preset: "kept" },
    }),
    named("team-c", "files-abc12", {
      registration: "rustfs",
      parameters: { preset: "rustfs-small" },
    }),
  ],
  environments: [],
  namespaces: [],
  registrations: [],
};

describe("presetResources", () => {
  it("lists the resources of the preset kind whose presetRef names the preset", () => {
    expect(
      presetResources("ElasticsearchClusterPreset", "standard", sources),
    ).toEqual([
      { kind: "ElasticsearchCluster", namespace: "team-a", name: "orders-es" },
    ]);
  });

  it("ignores a resource of another kind with the same presetRef", () => {
    expect(
      presetResources("DatabaseServerPreset", "standard", sources).map(
        (u) => u.kind,
      ),
    ).toEqual(["DatabaseServer"]);
  });

  it("matches nothing for an empty name", () => {
    expect(
      presetResources("CamundaClusterPreset", "", {
        ...sources,
        clusters: [named("team-a", "bare", { presetRef: "" })],
      }),
    ).toEqual([]);
  });
});

describe("presetUsers", () => {
  it("counts a cluster claim that names a cluster preset before its cluster exists", () => {
    expect(presetUsers("CamundaClusterPreset", "large", sources)).toEqual([
      { kind: "CamundaClusterClaim", namespace: "team-b", name: "billing" },
    ]);
  });

  it("lists the resources first, then the claims", () => {
    expect(presetUsers("CamundaClusterPreset", "medium", sources)).toEqual([
      { kind: "CamundaCluster", namespace: "team-a", name: "orders" },
      { kind: "CamundaClusterClaim", namespace: "team-a", name: "orders" },
      { kind: "CamundaClusterClaim", namespace: "team-d", name: "twice" },
    ]);
  });

  it("counts a built-in storage selection by its preset parameter, in both claim kinds", () => {
    expect(presetUsers("DatabaseServerPreset", "small", sources)).toEqual([
      { kind: "CamundaClusterClaim", namespace: "team-a", name: "orders" },
      {
        kind: "CamundaManagementClusterClaim",
        namespace: "platform",
        name: "plane",
      },
    ]);
  });

  it("counts a request of a built-in storage registration by its preset parameter", () => {
    expect(presetUsers("ElasticsearchClusterPreset", "kept", sources)).toEqual([
      { kind: "PluginRequest", namespace: "team-c", name: "search-abc12" },
    ]);
  });

  it("does not count a preset parameter of a registration that names no preset kind", () => {
    expect(
      presetUsers("ElasticsearchClusterPreset", "rustfs-small", sources),
    ).toEqual([]);
  });

  it("does not count a selection of a registration whose preset is of another kind", () => {
    expect(presetUsers("ElasticsearchClusterPreset", "small", sources)).toEqual(
      [],
    );
  });

  describe("with a preset from the policy of the claim", () => {
    const defaults = {
      metadata: { name: "database-server" },
      spec: {
        sections: ["storage"],
        mode: "Default",
        parameterValues: { preset: "policy-default" },
      },
    } as PluginRegistration;
    const claim = (resolvedPolicy?: ResolvedPolicy) => ({
      ...named("team-e", "plain", {
        plugins: [{ section: "storage", registration: "database-server" }],
      }),
      ...(resolvedPolicy && { status: { resolvedPolicy } }),
    });
    const recorded = (preset: string): ResolvedPolicy => ({
      deletionPolicy: "Delete",
      parameters: { "database-server": { preset: { value: preset } } },
    });
    const users = (
      name: string,
      planeClaims: PresetUsageSources["planeClaims"],
    ) =>
      presetUsers("DatabaseServerPreset", name, {
        ...sources,
        clusterClaims: [],
        planeClaims,
        namespaces: [{ metadata: { name: "team-e" } }],
        environments: [],
        registrations: [defaults],
      });
    const plain = {
      kind: "CamundaManagementClusterClaim",
      namespace: "team-e",
      name: "plain",
    };

    it("counts a claim whose recorded policy gives its request the preset", () => {
      expect(users("recorded", [claim(recorded("recorded"))])).toEqual([plain]);
    });

    it("counts a claim without a recorded policy whose registration gives the preset", () => {
      expect(users("policy-default", [claim()])).toEqual([plain]);
    });

    it("takes the recorded policy over the registration", () => {
      expect(users("policy-default", [claim(recorded("recorded"))])).toEqual(
        [],
      );
    });

    it("counts a claim whose selection gives a claim-editable preset over the recorded policy", () => {
      const resolvedPolicy: ResolvedPolicy = {
        deletionPolicy: "Delete",
        parameters: {
          "database-server": {
            preset: { value: "recorded", claimEditable: true },
          },
        },
      };
      const picked = {
        ...named("team-e", "plain", {
          plugins: [
            {
              section: "storage",
              registration: "database-server",
              parameters: { preset: "picked" },
            },
          ],
        }),
        status: { resolvedPolicy },
      };
      expect(users("picked", [picked])).toEqual([plain]);
    });

    it("counts a claim whose environment gives the preset over the registration", () => {
      const environment = {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        metadata: { name: "production" },
        spec: {
          parameters: {
            "database-server": { preset: { value: "from-environment" } },
          },
        },
      } as Environment;
      const inEnvironment = (name: string) =>
        presetUsers("DatabaseServerPreset", name, {
          ...sources,
          clusterClaims: [],
          planeClaims: [claim()],
          namespaces: [
            {
              metadata: {
                name: "team-e",
                labels: { [ENVIRONMENT_LABEL]: "production" },
              },
            },
          ],
          environments: [environment],
          registrations: [defaults],
        });
      expect(inEnvironment("from-environment")).toEqual([plain]);
      expect(inEnvironment("policy-default")).toEqual([]);
    });
  });

  describe("with a claim that does not resolve", () => {
    const recorded = (preset: string): ResolvedPolicy => ({
      deletionPolicy: "Delete",
      parameters: { elasticsearch: { preset: { value: preset } } },
    });
    const search = (parameters?: unknown) => ({
      section: "secondary-storage",
      registration: "elasticsearch",
      ...(parameters !== undefined && { parameters }),
    });
    const notAnObject = {
      section: "backup-storage",
      registration: "existing-object-storage",
      parameters: "x",
    };
    const claim = (
      plugins: {
        section: string;
        registration: string;
        parameters?: unknown;
      }[],
      resolvedPolicy?: ResolvedPolicy,
    ) => ({
      ...named("team-f", "broken", { plugins }),
      ...(resolvedPolicy && { status: { resolvedPolicy } }),
    });
    const registration = {
      metadata: { name: "elasticsearch" },
      spec: {
        sections: ["secondary-storage"],
        mode: "Default",
        parameterValues: { preset: "unlisted" },
      },
    } as PluginRegistration;
    const users = (
      name: string,
      clusterClaims: PresetUsageSources["clusterClaims"],
      more: Partial<PresetUsageSources> = {},
    ) =>
      presetUsers("ElasticsearchClusterPreset", name, {
        ...sources,
        clusterClaims,
        planeClaims: [],
        requests: [],
        namespaces: [{ metadata: { name: "team-f" } }],
        environments: [],
        registrations: [],
        ...more,
      });
    const broken = {
      kind: "CamundaClusterClaim",
      namespace: "team-f",
      name: "broken",
    };

    it("counts the preset that the recorded policy gives a selection while another selection is not a JSON object", () => {
      expect(
        users("unlisted", [
          claim([search(), notAnObject], recorded("unlisted")),
        ]),
      ).toEqual([broken]);
    });

    it("counts the preset that the recorded policy gives a selection whose parameters are not a JSON object", () => {
      expect(
        users("unlisted", [claim([search("x")], recorded("unlisted"))]),
      ).toEqual([broken]);
    });

    it("counts the preset that a selection names while another selection is not a JSON object", () => {
      expect(
        users("chosen", [
          claim([search({ preset: "chosen" }), notAnObject], recorded("other")),
        ]),
      ).toEqual([broken]);
    });

    it("does not count a preset that neither the selections nor the recorded policy name", () => {
      expect(
        users("unlisted", [
          claim([search("x"), notAnObject], recorded("other")),
        ]),
      ).toEqual([]);
    });

    it("counts a request that names the preset as well as the claim", () => {
      expect(
        users("unlisted", [claim([search("x")], recorded("unlisted"))], {
          requests: [
            named("team-f", "broken-search", {
              registration: "elasticsearch",
              parameters: { preset: "unlisted" },
            }),
          ],
        }),
      ).toEqual([
        broken,
        { kind: "PluginRequest", namespace: "team-f", name: "broken-search" },
      ]);
    });

    it("counts the preset that the registration gives a selection while the registration of another selection does not exist", () => {
      expect(
        users(
          "unlisted",
          [
            claim([
              search(),
              { section: "backup-storage", registration: "missing" },
            ]),
          ],
          { registrations: [registration] },
        ),
      ).toEqual([broken]);
    });

    it("counts the preset that the registration gives a selection whose parameters are not a JSON object", () => {
      expect(
        users("unlisted", [claim([search("x")])], {
          registrations: [registration],
        }),
      ).toEqual([broken]);
    });
  });

  it("lists a user once", () => {
    expect(presetUsers("ElasticsearchClusterPreset", "twice", sources)).toEqual(
      [{ kind: "CamundaClusterClaim", namespace: "team-d", name: "twice" }],
    );
  });

  it("matches nothing for an empty name", () => {
    expect(
      presetUsers("DatabaseServerPreset", "", {
        ...sources,
        planeClaims: [
          named("platform", "plane", {
            plugins: [selection("management-database-server", { preset: "" })],
          }),
        ],
      }),
    ).toEqual([]);
  });
});
