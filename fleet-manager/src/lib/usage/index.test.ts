import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { environmentCounts, presetUsage } from "./index";

function cluster(namespace: string, name: string, presetRef?: string) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { namespace, name },
    spec: { ...(presetRef && { presetRef }) },
  } as CamundaCluster;
}

function environment(name: string): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: {},
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

describe("environmentCounts", () => {
  it("counts the namespaces and the clusters of each environment", () => {
    expect(
      environmentCounts(
        [environment("prod"), environment("dev")],
        [
          namespace("team-a", "prod"),
          namespace("team-b", "prod"),
          namespace("sandbox", "dev"),
          namespace("kube-system"),
        ],
        [
          cluster("team-a", "orders"),
          cluster("team-a", "billing"),
          cluster("kube-system", "stray"),
        ],
      ),
    ).toEqual({
      prod: { namespaces: 2, clusters: 2 },
      dev: { namespaces: 1, clusters: 0 },
    });
  });

  it("counts nothing for a label that names no environment", () => {
    expect(
      environmentCounts(
        [environment("prod")],
        [namespace("team-x", "gone")],
        [cluster("team-x", "orders")],
      ),
    ).toEqual({ prod: { namespaces: 0, clusters: 0 } });
  });
});
