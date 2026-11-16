import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { Environment } from "@/generated/crds/environment";
import {
  type PlatformUsageSources,
  platformUserCount,
  platformUsers,
} from "./platform";

function object(kind: string, namespace: string, name: string, spec: object) {
  return { kind, metadata: { namespace, name }, spec } as unknown;
}

const cluster = (namespace: string, name: string, ref?: string) =>
  object("CamundaCluster", namespace, name, {
    ...(ref && { platformConfigRef: ref }),
  }) as CamundaCluster;

const plane = (namespace: string, name: string, ref: string) =>
  object("CamundaManagementCluster", namespace, name, {
    platformConfigRef: ref,
  }) as CamundaManagementCluster;

const server = (namespace: string, name: string, spec: object) =>
  object("DatabaseServer", namespace, name, spec) as DatabaseServer;

const preset = (name: string, ref: string) =>
  ({
    kind: "DatabaseServerPreset",
    metadata: { name },
    spec: { server: { platformConfigRef: ref } },
  }) as unknown as DatabaseServerPreset;

const clusterClaim = (namespace: string, name: string, ref?: string) =>
  object("CamundaClusterClaim", namespace, name, {
    ...(ref && { platformConfigRef: ref }),
  }) as CamundaClusterClaim;

const planeClaim = (namespace: string, name: string, ref?: string) =>
  object("CamundaManagementClusterClaim", namespace, name, {
    ...(ref && { platformConfigRef: ref }),
  }) as CamundaManagementClusterClaim;

const environment = (name: string, ref?: string) =>
  ({
    kind: "Environment",
    metadata: { name },
    spec: { ...(ref && { platformConfigRef: ref }) },
  }) as unknown as Environment;

function sources(
  overrides: Partial<PlatformUsageSources> = {},
): PlatformUsageSources {
  return {
    clusters: [],
    planes: [],
    databaseServers: [],
    databasePresets: [],
    clusterClaims: [],
    planeClaims: [],
    environments: [],
    ...overrides,
  };
}

const names = (
  items: readonly { metadata: { namespace?: string; name?: string } }[],
) => items.map((i) => `${i.metadata.namespace}/${i.metadata.name}`);

describe("platformUsers", () => {
  it("counts clusters and management planes whose platformConfigRef names the config", () => {
    const users = platformUsers(
      "prod",
      sources({
        clusters: [
          cluster("team-a", "orders", "prod"),
          cluster("team-a", "lab", "lab"),
        ],
        planes: [
          plane("platform", "main", "prod"),
          plane("platform", "test", "lab"),
        ],
      }),
    );

    expect(names(users.clusters)).toEqual(["team-a/orders"]);
    expect(names(users.planes)).toEqual(["platform/main"]);
  });

  it("counts a database server whose own spec names the config", () => {
    const own = server("team-a", "orders-db", { platformConfigRef: "prod" });

    const users = platformUsers("prod", sources({ databaseServers: [own] }));

    expect(users.databaseServers).toEqual([{ server: own }]);
  });

  it("counts a database server whose preset names the config and names that preset", () => {
    const fromPreset = server("team-a", "orders-db", {
      presetRef: "pg-standard",
    });

    const users = platformUsers(
      "prod",
      sources({
        databaseServers: [fromPreset],
        databasePresets: [preset("pg-standard", "prod")],
      }),
    );

    expect(users.databaseServers).toEqual([
      { server: fromPreset, preset: "pg-standard" },
    ]);
  });

  it("does not count a database server whose own reference replaces the preset's", () => {
    const own = server("team-a", "orders-db", {
      presetRef: "pg-standard",
      platformConfigRef: "lab",
    });
    const all = sources({
      databaseServers: [own],
      databasePresets: [preset("pg-standard", "prod")],
    });

    expect(platformUsers("prod", all).databaseServers).toEqual([]);
    expect(platformUsers("lab", all).databaseServers).toEqual([
      { server: own },
    ]);
  });

  it("counts a claim whose cluster does not exist yet", () => {
    const users = platformUsers(
      "prod",
      sources({ clusterClaims: [clusterClaim("team-b", "billing", "prod")] }),
    );

    expect(names(users.clusterClaims)).toEqual(["team-b/billing"]);
  });

  it("counts a claim once through its cluster when the cluster names the config", () => {
    const users = platformUsers(
      "prod",
      sources({
        clusters: [cluster("team-a", "orders", "prod")],
        clusterClaims: [clusterClaim("team-a", "orders", "prod")],
      }),
    );

    expect(names(users.clusters)).toEqual(["team-a/orders"]);
    expect(users.clusterClaims).toEqual([]);
  });

  it("counts a claim whose cluster still names another config", () => {
    const users = platformUsers(
      "prod",
      sources({
        clusters: [cluster("team-a", "orders", "lab")],
        clusterClaims: [clusterClaim("team-a", "orders", "prod")],
      }),
    );

    expect(users.clusters).toEqual([]);
    expect(names(users.clusterClaims)).toEqual(["team-a/orders"]);
  });

  it("does not count a claim of a cluster with the same name in another namespace as its own", () => {
    const users = platformUsers(
      "prod",
      sources({
        clusters: [cluster("team-b", "orders", "prod")],
        clusterClaims: [clusterClaim("team-a", "orders", "prod")],
      }),
    );

    expect(names(users.clusterClaims)).toEqual(["team-a/orders"]);
  });

  it("counts a management plane claim by the same rule", () => {
    const users = platformUsers(
      "prod",
      sources({
        planes: [plane("platform", "main", "prod")],
        planeClaims: [
          planeClaim("platform", "main", "prod"),
          planeClaim("platform", "next", "prod"),
          planeClaim("platform", "other"),
        ],
      }),
    );

    expect(names(users.planes)).toEqual(["platform/main"]);
    expect(names(users.planeClaims)).toEqual(["platform/next"]);
  });

  it("counts the environments that default to the config", () => {
    const users = platformUsers(
      "prod",
      sources({
        environments: [
          environment("staging", "prod"),
          environment("production", "prod"),
          environment("lab", "lab"),
          environment("none"),
        ],
      }),
    );

    expect(users.environments).toEqual(["production", "staging"]);
  });

  it("matches nothing for an empty name", () => {
    const users = platformUsers(
      "",
      sources({
        clusters: [cluster("team-a", "orders")],
        clusterClaims: [clusterClaim("team-a", "billing")],
        planeClaims: [planeClaim("platform", "main")],
        databaseServers: [server("team-a", "db", {})],
        environments: [environment("none")],
      }),
    );

    expect(platformUserCount(users)).toBe(0);
  });

  it("sorts each list by namespace and name", () => {
    const users = platformUsers(
      "prod",
      sources({
        clusters: [
          cluster("team-b", "a", "prod"),
          cluster("team-a", "z", "prod"),
          cluster("team-a", "b", "prod"),
        ],
        databaseServers: [
          server("team-b", "db", { platformConfigRef: "prod" }),
          server("team-a", "db", { platformConfigRef: "prod" }),
        ],
        clusterClaims: [
          clusterClaim("team-d", "x", "prod"),
          clusterClaim("team-c", "x", "prod"),
        ],
      }),
    );

    expect(names(users.clusters)).toEqual(["team-a/b", "team-a/z", "team-b/a"]);
    expect(names(users.databaseServers.map((s) => s.server))).toEqual([
      "team-a/db",
      "team-b/db",
    ]);
    expect(names(users.clusterClaims)).toEqual(["team-c/x", "team-d/x"]);
  });
});

describe("platformUserCount", () => {
  it("adds every user", () => {
    const users = platformUsers(
      "prod",
      sources({
        clusters: [cluster("team-a", "orders", "prod")],
        planes: [plane("platform", "main", "prod")],
        databaseServers: [
          server("team-a", "db", { platformConfigRef: "prod" }),
        ],
        clusterClaims: [clusterClaim("team-b", "billing", "prod")],
        planeClaims: [planeClaim("platform", "next", "prod")],
        environments: [environment("production", "prod")],
      }),
    );

    expect(platformUserCount(users)).toBe(6);
  });
});
