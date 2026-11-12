import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  environmentRows,
  labelledNamespaces,
  namespacesToAdd,
  unassignedNamespaces,
} from "./rows";
import type { EnvironmentSources } from "./sources";

function environment(
  name: string,
  status?: Environment["status"],
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: {},
    ...(status && { status }),
  };
}

function namespace(
  name: string,
  env?: string,
  extra: Partial<V1Namespace["metadata"]> = {},
): V1Namespace {
  return {
    metadata: {
      name,
      ...(env !== undefined && { labels: { [ENVIRONMENT_LABEL]: env } }),
      ...extra,
    },
  };
}

function clusterClaim(ns: string, name: string): CamundaClusterClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: ns, name },
    spec: { presetRef: "small" },
  };
}

function planeClaim(ns: string, name: string): CamundaManagementClusterClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: ns, name },
    spec: { environments: ["staging"] },
  } as unknown as CamundaManagementClusterClaim;
}

function cluster(ns: string, name: string): CamundaCluster {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { namespace: ns, name },
    spec: {},
  } as CamundaCluster;
}

function sources(s: Partial<EnvironmentSources>): EnvironmentSources {
  return {
    environments: [],
    namespaces: [],
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

describe("environmentRows", () => {
  it("counts the labelled namespaces while the status has no namespaces", () => {
    const [row] = environmentRows(
      sources({
        environments: [environment("staging")],
        namespaces: [
          namespace("team-a", "staging"),
          namespace("team-b", "staging"),
          namespace("team-c", "production"),
        ],
      }),
    );
    expect(row.namespaces).toBe(2);
  });

  it("takes status.namespaces when the controller wrote it, also 0", () => {
    const [row] = environmentRows(
      sources({
        environments: [environment("staging", { namespaces: 0 })],
        namespaces: [namespace("team-a", "staging")],
      }),
    );
    expect(row.namespaces).toBe(0);
  });

  it("counts cluster claims and plane claims of the labelled namespaces apart", () => {
    const [row] = environmentRows(
      sources({
        environments: [environment("staging")],
        namespaces: [namespace("team-a", "staging"), namespace("other")],
        clusterClaims: [
          clusterClaim("team-a", "orders"),
          clusterClaim("other", "billing"),
        ],
        planeClaims: [planeClaim("team-a", "plane")],
      }),
    );
    expect(row).toMatchObject({ clusters: 1, planes: 1 });
  });

  it("gives one row for each environment by name, with its Ready condition and its looks", () => {
    const ready = {
      type: "Ready",
      status: "False" as const,
      reason: "InvalidReference",
      message: "The spec names objects that do not exist: CamundaRelease 8.9.9",
      lastTransitionTime: "2026-11-10T10:00:00Z",
    };
    const staging = environment("staging", { conditions: [ready] });
    staging.spec = { displayName: "Staging", color: "#F9A825" };
    expect(
      environmentRows(
        sources({ environments: [staging, environment("production")] }),
      ).map((r) => [r.name, r.displayName, r.color, r.ready?.status]),
    ).toEqual([
      ["production", undefined, undefined, undefined],
      ["staging", "Staging", "#F9A825", "False"],
    ]);
  });
});

describe("unassignedNamespaces", () => {
  it("lists a namespace with a claim or a cluster and no label once an Environment exists", () => {
    expect(
      unassignedNamespaces(
        sources({
          environments: [environment("staging")],
          namespaces: [namespace("legacy"), namespace("sandbox")],
          clusters: [cluster("legacy", "orders")],
        }),
      ),
    ).toEqual([{ name: "legacy", reason: "NoEnvironment" }]);
  });

  it("lists a namespace whose label names no Environment", () => {
    expect(
      unassignedNamespaces(
        sources({
          environments: [environment("staging")],
          namespaces: [namespace("old-ns", "old")],
          clusterClaims: [clusterClaim("old-ns", "orders")],
        }),
      ),
    ).toEqual([{ name: "old-ns", reason: "UnknownEnvironment" }]);
  });

  it("lists the namespaces by name and skips a namespace of an environment", () => {
    expect(
      unassignedNamespaces(
        sources({
          environments: [environment("staging")],
          namespaces: [
            namespace("team-x"),
            namespace("legacy"),
            namespace("team-a", "staging"),
          ],
          planeClaims: [planeClaim("team-x", "plane")],
          clusterClaims: [
            clusterClaim("legacy", "a"),
            clusterClaim("team-a", "b"),
          ],
        }),
      ).map((n) => n.name),
    ).toEqual(["legacy", "team-x"]);
  });

  it("lists nothing while no Environment exists", () => {
    expect(
      unassignedNamespaces(
        sources({
          namespaces: [namespace("legacy"), namespace("old-ns", "old")],
          clusters: [cluster("legacy", "orders"), cluster("old-ns", "billing")],
        }),
      ),
    ).toEqual([]);
  });
});

describe("labelledNamespaces", () => {
  it("names the namespaces whose label names the environment, sorted", () => {
    expect(
      labelledNamespaces("staging", [
        namespace("team-b", "staging"),
        namespace("team-a", "staging"),
        namespace("team-c", "production"),
        namespace("team-d"),
      ]),
    ).toEqual(["team-a", "team-b"]);
  });

  it("names none for an empty environment name", () => {
    expect(labelledNamespaces("", [namespace("team-a", "")])).toEqual([]);
  });
});

describe("namespacesToAdd", () => {
  it("offers only namespaces without an environment label, and not one that is being deleted", () => {
    expect(
      namespacesToAdd([
        namespace("b", ""),
        namespace("a"),
        namespace("c", "prod"),
        namespace("d", undefined, {
          deletionTimestamp: new Date("2026-11-10T10:00:00Z"),
        }),
      ]),
    ).toEqual(["a", "b"]);
  });
});
