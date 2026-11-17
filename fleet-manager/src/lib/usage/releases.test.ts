import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  type ReleaseUsageSources,
  type ReleaseUser,
  releaseSecretNamespaces,
  releaseUsage,
} from "./releases";

function user(namespace: string, name: string, releaseRef?: string) {
  return {
    metadata: { namespace, name },
    spec: { ...(releaseRef && { releaseRef }) },
  } satisfies ReleaseUser;
}

function sources(
  partial: Partial<ReleaseUsageSources> = {},
): ReleaseUsageSources {
  return {
    clusters: [],
    elasticsearchClusters: [],
    databaseServers: [],
    claims: [],
    ...partial,
  };
}

function environment(name: string, allowed: string[] = []): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: { releases: { allowed } },
  };
}

function namespace(name: string, environment?: string): V1Namespace {
  return {
    metadata: {
      name,
      ...(environment && { labels: { [ENVIRONMENT_LABEL]: environment } }),
    },
  };
}

const empty = {
  clusters: [],
  elasticsearchClusters: [],
  databaseServers: [],
  claims: [],
};

describe("releaseUsage", () => {
  it("lists the users of a release by kind, as <namespace>/<name>, sorted", () => {
    expect(
      releaseUsage(
        "r1",
        sources({
          clusters: [
            user("team-b", "billing", "r1"),
            user("team-a", "orders", "r1"),
          ],
          elasticsearchClusters: [user("team-a", "search", "r1")],
          databaseServers: [user("team-a", "db", "r2")],
          claims: [user("team-a", "orders", "r1")],
        }),
      ),
    ).toEqual({
      clusters: ["team-a/orders", "team-b/billing"],
      elasticsearchClusters: ["team-a/search"],
      databaseServers: [],
      claims: ["team-a/orders"],
    });
  });

  it("ignores a resource without a releaseRef and an empty release name", () => {
    const all = sources({
      clusters: [user("team-a", "orders")],
      claims: [user("team-a", "orders")],
    });
    expect(releaseUsage("", all)).toEqual(empty);
    expect(releaseUsage("r1", all)).toEqual(empty);
  });
});

describe("releaseSecretNamespaces", () => {
  it("gives the namespaces of the users as the Secret namespaces", () => {
    expect(
      releaseSecretNamespaces("r1", {
        ...sources({
          clusters: [user("team-b", "billing", "r1")],
          claims: [user("team-a", "orders", "r1")],
          elasticsearchClusters: [user("team-a", "search", "r1")],
        }),
        environments: [environment("production", ["r1"])],
        namespaces: [namespace("team-orders", "production")],
      }),
    ).toEqual(["team-a", "team-b"]);
  });

  it("gives the namespaces of the environments that allow an unused release", () => {
    expect(
      releaseSecretNamespaces("r1", {
        ...sources(),
        environments: [
          environment("production", ["r1"]),
          environment("staging", ["r2"]),
        ],
        namespaces: [
          namespace("team-x", "staging"),
          namespace("team-orders", "production"),
          namespace("loose"),
        ],
      }),
    ).toEqual(["team-orders"]);
  });

  it("gives no Secret namespace for an unused release that no environment allows", () => {
    expect(
      releaseSecretNamespaces("r1", {
        ...sources(),
        environments: [environment("staging", ["r2"])],
        namespaces: [namespace("team-x", "staging")],
      }),
    ).toEqual([]);
  });
});
