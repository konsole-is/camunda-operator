import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { allowedIn, defaultIn, environmentCounts } from "./environments";

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

const env = (name: string, spec: Environment["spec"]): Environment => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name },
  spec,
});
const environments = [
  env("staging", {
    presets: { camundaCluster: ["small"] },
    releases: { allowed: ["8.9.1", "8.9.2"], default: "8.9.2" },
    platformConfigRef: "shared",
  }),
  env("production", {
    presets: {
      camundaCluster: ["small", "large"],
      elasticsearchCluster: ["es-small"],
    },
    releases: { allowed: ["8.9.1"] },
    platformConfigRef: "shared",
  }),
  env("lab", {}),
];

describe("allowedIn", () => {
  it("names each environment that lists the preset, sorted", () =>
    expect(allowedIn(environments, "CamundaClusterPreset", "small")).toEqual([
      "production",
      "staging",
    ]));

  it("reads the list of the preset kind", () =>
    expect(
      allowedIn(environments, "ElasticsearchClusterPreset", "small"),
    ).toEqual([]));

  it("reads releases.allowed for a release", () =>
    expect(allowedIn(environments, "CamundaRelease", "8.9.1")).toEqual([
      "production",
      "staging",
    ]));

  it("names none for an environment without spec.releases", () =>
    expect(allowedIn([env("lab", {})], "CamundaRelease", "8.9.1")).toEqual([]));

  it("names none for an empty name", () =>
    expect(
      allowedIn(
        [env("odd", { presets: { databaseServer: [""] } })],
        "DatabaseServerPreset",
        "",
      ),
    ).toEqual([]));
});

describe("defaultIn", () => {
  it("names the environments whose default release is the release", () =>
    expect(defaultIn(environments, "CamundaRelease", "8.9.2")).toEqual([
      "staging",
    ]));

  it("names the environments whose platform config is the config, sorted", () =>
    expect(defaultIn(environments, "CamundaPlatformConfig", "shared")).toEqual([
      "production",
      "staging",
    ]));

  it("names none for an empty default", () =>
    expect(defaultIn(environments, "CamundaRelease", "")).toEqual([]));
});
