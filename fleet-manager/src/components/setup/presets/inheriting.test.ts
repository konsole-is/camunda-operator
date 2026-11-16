import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { presetFixtures } from "./fixtures";
import { inheritingRows } from "./inheriting";

const sources = presetFixtures();
const medium = sources.clusterPresets[0];

describe("inheritingRows", () => {
  it("shows each inheriting cluster with its environment, namespace, own fields, and Ready", () => {
    const [row] = inheritingRows(medium, presetFixtures());
    expect(row).toMatchObject({
      kind: "CamundaCluster",
      namespace: "team-orders",
      name: "orders",
      environment: "Production",
      ready: { status: "True" },
      href: "/fleet/clusters/team-orders/orders",
    });
    expect(row.ownFields).toContainEqual({ path: "zeebe.replicas" });
  });

  it("shows a field that a slot fills as Filled by its section", () => {
    expect(
      inheritingRows(medium, presetFixtures())[0].ownFields,
    ).toContainEqual({
      path: "zeebe.storageClassName",
      filledBy: "Volume encryption",
    });
  });

  it("shows a map that a slot fills as one field, without its keys", () => {
    const fixtures = presetFixtures();
    fixtures.clusters[0].spec.connectors = {
      serviceAccount: {
        annotations: { "eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/c" },
      },
    };
    fixtures.clusterClaims[0].spec.plugins?.push({
      section: "connectors-identity",
      registration: "aws-identity",
    });
    fixtures.sections.push({
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginSection",
      metadata: { name: "connectors-identity" },
      spec: {
        label: "Connectors identity",
        slot: "connectorsIdentity",
        appliesTo: ["CamundaClusterClaim"],
        selection: "One",
      },
    } as Resource<"PluginSection">);
    const own = inheritingRows(medium, fixtures)[0].ownFields;
    expect(own.filter((f) => f.path.startsWith("connectors."))).toEqual([
      {
        path: "connectors.serviceAccount.annotations",
        filledBy: "Connectors identity",
      },
    ]);
  });

  it("leaves out the instance-bound fields that every resource sets", () => {
    const paths = inheritingRows(medium, presetFixtures())[0].ownFields.map(
      (f) => f.path,
    );
    expect(paths).toEqual(["zeebe.replicas", "zeebe.storageClassName"]);
  });

  it("gives an Elasticsearch user the fields it sets over the preset, with no link", () => {
    const fixtures = presetFixtures();
    fixtures.elasticsearchClusters = [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "ElasticsearchCluster",
        metadata: { namespace: "team-orders", name: "search" },
        spec: {
          presetRef: "standard",
          version: "8.19.0",
          replicas: 1,
        },
      } as Resource<"ElasticsearchCluster">,
    ];
    expect(inheritingRows(fixtures.elasticsearchPresets[0], fixtures)).toEqual([
      {
        kind: "ElasticsearchCluster",
        namespace: "team-orders",
        name: "search",
        environment: "Production",
        ownFields: [{ path: "replicas" }],
      },
    ]);
  });

  it("has no environment for a namespace without one", () => {
    const fixtures = presetFixtures();
    fixtures.databaseServers = [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "DatabaseServer",
        metadata: { namespace: "platform", name: "db" },
        spec: { presetRef: "small", version: "17" },
      } as Resource<"DatabaseServer">,
    ];
    const [row] = inheritingRows(fixtures.databasePresets[0], fixtures);
    expect(row.environment).toBeUndefined();
    expect(row.ownFields).toEqual([]);
  });

  it("lists no resource of a preset that nothing names", () => {
    expect(inheritingRows(sources.clusterPresets[1], presetFixtures())).toEqual(
      [],
    );
  });
});
