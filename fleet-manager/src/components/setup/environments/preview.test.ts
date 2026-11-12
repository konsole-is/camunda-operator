import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { environmentFixtures, sectionFixtures } from "./fixtures";
import { previewClaims, templateData, templateErrors } from "./preview";

const sources = sectionFixtures();
const production = sources.environments[0];
const registration = (name: string) =>
  sources.registrations.find((r) => r.metadata.name === name)!;
const section = (name: string) =>
  sources.sections.find((s) => s.metadata.name === name)!;

const orders = {
  kind: "CamundaClusterClaim" as const,
  namespace: "team-a",
  name: "orders",
  plugins: [],
};

describe("previewClaims", () => {
  it("returns the claims of the namespaces of the environment that a section of the registration applies to, by namespace and name", () => {
    expect(
      previewClaims(sources, "production", "aws-connectors-role")
        .map((c) => `${c.namespace}/${c.name}`),
    ).toEqual(["team-a/billing", "team-a/orders"]);
  });

  it("leaves out a claim of a kind that no section of the registration applies to", () => {
    const s = sectionFixtures();
    s.namespaces.find((n) => n.metadata?.name === "platform")!.metadata!.labels = {
      [ENVIRONMENT_LABEL]: "production",
    };
    expect(
      previewClaims(s, "production", "aws-connectors-role").map(
        (c) => c.name,
      ),
    ).toEqual(["billing", "orders"]);
    expect(
      previewClaims(s, "production", "gateway-public").map(
        (c) => c.name,
      ),
    ).toEqual(["plane-a", "billing", "orders"]);
  });

  it("offers the example claim in the first namespace without a claim", () => {
    expect(
      previewClaims(sources, "staging", "aws-connectors-role"),
    ).toEqual([
      {
        kind: "CamundaClusterClaim",
        namespace: "team-c",
        name: "example",
        plugins: [],
        example: true,
      },
    ]);
  });

  it("offers no claim without a namespace", () => {
    expect(previewClaims(sources, "lab", "aws-connectors-role")).toEqual([]);
    expect(previewClaims(sources, "", "aws-connectors-role")).toEqual([]);
  });
});

describe("templateData", () => {
  const input = {
    claim: orders,
    section: section("connectors-permissions"),
    registration: registration("aws-connectors-role"),
    environment: production,
    registrations: sources.registrations,
    requests: [] as PluginRequest[],
  };

  it("computes the request name and the account of a cluster claim without a request", () => {
    expect(templateData(input)).toMatchObject({
      namespace: "team-a",
      claim: "orders",
      request: "orders-connectors-permissions",
      serviceAccount: "orders-connectors",
      outputs: {},
    });
  });

  it("takes the request name and the account of the live request", () => {
    const request = {
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginRequest",
      metadata: { namespace: "team-a", name: "orders-perms-1" },
      spec: {
        claimRef: { kind: "CamundaClusterClaim", name: "orders" },
        section: "connectors-permissions",
        registration: "aws-connectors-role",
        deletionPolicy: "Delete",
        target: {
          kind: "CamundaCluster",
          name: "orders",
          serviceAccountName: "orders-custom",
        },
      },
    } as PluginRequest;
    const other = {
      ...request,
      metadata: { namespace: "team-b", name: "elsewhere" },
    } as PluginRequest;
    expect(
      templateData({ ...input, requests: [other, request] }),
    ).toMatchObject({ request: "orders-perms-1", serviceAccount: "orders-custom" });
  });

  it("gives a plane claim the account of the plane", () => {
    const env = environmentFixtures();
    expect(
      templateData({
        ...input,
        claim: {
          kind: "CamundaManagementClusterClaim",
          namespace: "platform",
          name: "plane-a",
          plugins: [],
        },
        section: section("access"),
        registration: registration("gateway-public"),
        environment: env.environments[2],
      }),
    ).toMatchObject({ request: "plane-a-access", serviceAccount: "plane-a-management" });
  });

  it("takes the parameters that the edited environment gives the request", () => {
    const edited: Environment = {
      ...production,
      spec: {
        ...production.spec,
        parameters: {
          "aws-connectors-role": { managedPolicyArns: { value: ["arn:a"] } },
        },
      },
    };
    expect(
      templateData({ ...input, environment: edited }).parameters,
    ).toEqual({ managedPolicyArns: ["arn:a"] });
  });

  it("takes the parameters of the selection of the claim", () => {
    expect(
      templateData({
        ...input,
        claim: {
          ...orders,
          plugins: [
            {
              section: "connectors-permissions",
              registration: "aws-connectors-role",
              parameters: { extraPolicyArns: ["sqs"] },
            },
          ],
        },
      }).parameters,
    ).toMatchObject({
      extraPolicyArns: ["arn:aws:iam::aws:policy/AmazonSQSFullAccess"],
    });
  });
});

describe("templateErrors", () => {
  it("names each string of an object that does not parse, by its path", () => {
    expect(
      templateErrors({
        Statement: [{ Resource: "{{ .claimName }}", Action: "{{ .outputs.x }}" }],
        Version: "2012-10-17",
      }),
    ).toEqual([
      {
        path: "Statement[0].Resource",
        message: "unknown placeholder .claimName",
      },
      {
        path: "Statement[0].Action",
        message:
          "the placeholder .outputs.x is not allowed: a template parameter cannot read the outputs",
      },
    ]);
  });

  it("names the error of a string value with an empty path", () => {
    expect(templateErrors("{{ .pods }}")).toEqual([
      {
        path: "",
        message: "the placeholder .pods is allowed only in a metric query",
      },
    ]);
  });

  it("has no error for a template that parses", () => {
    expect(templateErrors({ a: "{{ .namespace }}-{{ .parameters.x }}" })).toEqual([]);
  });
});
