import { describe, expect, it } from "vitest";
import { presetFixtures } from "./fixtures";
import { changedLabel, presetFacts, presetRows, usedByLabel } from "./rows";

const rowOf = (id: string, sources = presetFixtures()) =>
  presetRows(sources, []).find((r) => r.id === id);

describe("presetRows", () => {
  it("gives each preset its kind, facts, Allowed in, users, description, and last change", () => {
    expect(rowOf("cluster/medium")).toMatchObject({
      kind: "CamundaClusterPreset",
      name: "medium",
      description:
        "For production workloads with three brokers and replication.",
      facts: [
        { label: "Brokers", value: "3" },
        { label: "Partitions", value: "3" },
        { label: "CPU", value: "1" },
        { label: "Memory", value: "2Gi" },
        { label: "Storage", value: "32Gi" },
      ],
      allowedIn: ["production"],
      users: [
        { kind: "CamundaCluster", namespace: "team-orders", name: "orders" },
        {
          kind: "CamundaClusterClaim",
          namespace: "team-orders",
          name: "orders",
        },
      ],
      lastChanged: "2026-11-14T09:30:00.000Z",
      deletable: false,
      blockedBy:
        "Delete is possible when no resource, claim, or request uses the preset.",
    });
  });

  it("offers delete for an unused preset that no environment allows", () => {
    expect(rowOf("cluster/spare")).toMatchObject({
      deletable: true,
      allowedIn: [],
      users: [],
    });
    expect(rowOf("cluster/spare")?.blockedBy).toBeUndefined();
  });

  it("counts a claim selection as a user in the delete guard", () => {
    expect(rowOf("postgresql/small")).toMatchObject({
      users: [
        {
          kind: "CamundaManagementClusterClaim",
          namespace: "platform",
          name: "plane",
        },
      ],
      deletable: false,
    });
  });

  it("refuses delete for an allowed preset without users", () => {
    const sources = presetFixtures();
    sources.environments[0].spec.presets = {
      elasticsearchCluster: ["standard"],
    };
    expect(rowOf("elasticsearch/standard", sources)).toMatchObject({
      allowedIn: ["production"],
      users: [],
      deletable: false,
      blockedBy: "Delete is possible when no environment allows the preset.",
    });
  });

  it("keeps two presets of one name and two kinds apart", () => {
    const rows = presetRows(presetFixtures(), []);
    expect(
      rows
        .filter((r) => r.name === "standard")
        .map((r) => [r.id, r.kind, r.preset.metadata.uid]),
    ).toEqual([
      ["elasticsearch/standard", "ElasticsearchClusterPreset", "uid-es"],
      ["postgresql/standard", "DatabaseServerPreset", "uid-pg"],
    ]);
  });

  it("offers no delete while a list of users cannot be read", () => {
    const rows = presetRows(presetFixtures(), [
      { kind: "ElasticsearchCluster", message: "forbidden" },
    ]);
    expect(rows.find((r) => r.id === "elasticsearch/standard")).toMatchObject({
      deletable: false,
      blockedBy:
        "Delete is possible when the fleet manager can read ElasticsearchCluster.",
    });
    expect(rows.find((r) => r.id === "cluster/spare")?.deletable).toBe(true);
  });

  it("names every list that cannot be read", () => {
    const rows = presetRows(presetFixtures(), [
      { kind: "Environment", message: "forbidden" },
      { kind: "PluginRequest", message: "forbidden" },
    ]);
    expect(rows.find((r) => r.id === "postgresql/standard")?.blockedBy).toBe(
      "Delete is possible when the fleet manager can read Environment, PluginRequest.",
    );
  });

  it("sorts by kind, then by name", () => {
    expect(presetRows(presetFixtures(), []).map((r) => r.id)).toEqual([
      "cluster/medium",
      "cluster/spare",
      "elasticsearch/standard",
      "postgresql/small",
      "postgresql/standard",
    ]);
  });

  it("has no last change for a preset without a timed managedFields entry", () => {
    expect(rowOf("cluster/spare")?.lastChanged).toBeUndefined();
  });
});

describe("presetFacts", () => {
  it("gives the facts of each kind in order, and only the set ones", () => {
    expect(
      presetFacts("ElasticsearchClusterPreset", {
        replicas: 3,
        storageSize: "64Gi",
      }),
    ).toEqual([
      { label: "Nodes", value: "3" },
      { label: "Storage", value: "64Gi" },
    ]);
    expect(
      presetFacts("DatabaseServerPreset", {
        instances: 2,
        resources: { requests: { cpu: "500m", memory: "1Gi" } },
      }),
    ).toEqual([
      { label: "Instances", value: "2" },
      { label: "CPU", value: "500m" },
      { label: "Memory", value: "1Gi" },
    ]);
    expect(presetFacts("CamundaClusterPreset", undefined)).toEqual([]);
  });
});

describe("usedByLabel", () => {
  it("says Unused, or the users by kind", () => {
    expect(usedByLabel([])).toBe("Unused");
    expect(
      usedByLabel([
        { kind: "CamundaCluster", namespace: "a", name: "x" },
        { kind: "CamundaCluster", namespace: "b", name: "y" },
        { kind: "CamundaManagementClusterClaim", namespace: "c", name: "z" },
      ]),
    ).toBe("2 clusters, 1 claim");
  });

  it("counts both claim kinds as claims, and names the other kinds", () => {
    expect(
      usedByLabel([
        { kind: "ElasticsearchCluster", namespace: "a", name: "x" },
        { kind: "DatabaseServer", namespace: "a", name: "x" },
        { kind: "DatabaseServer", namespace: "a", name: "y" },
        { kind: "CamundaClusterClaim", namespace: "a", name: "x" },
        { kind: "CamundaManagementClusterClaim", namespace: "a", name: "x" },
        { kind: "PluginRequest", namespace: "a", name: "x" },
      ]),
    ).toBe("1 Elasticsearch cluster, 2 database servers, 2 claims, 1 request");
  });
});

describe("changedLabel", () => {
  it("shows a change time in UTC to the minute", () => {
    expect(changedLabel("2026-11-14T09:30:59.000Z")).toBe(
      "2026-11-14 09:30 UTC",
    );
  });
});
