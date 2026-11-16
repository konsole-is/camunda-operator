import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  environmentFixtures,
  sectionFixtures,
} from "@/components/setup/environments/fixtures";
import { grantPreviews, templateData } from "./templates";

const sources = sectionFixtures();
const production = sources.environments[0];
const registrationNamed = (name: string) =>
  sources.registrations.find((r) => r.metadata.name === name)!;
const sectionNamed = (name: string) =>
  sources.sections.find((s) => s.metadata.name === name)!;

const orders = {
  kind: "CamundaClusterClaim" as const,
  namespace: "team-a",
  name: "orders",
  plugins: [],
};

describe("templateData", () => {
  const input = {
    claim: orders,
    section: sectionNamed("connectors-permissions"),
    registration: registrationNamed("aws-connectors-role"),
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
    ).toMatchObject({
      request: "orders-perms-1",
      serviceAccount: "orders-custom",
    });
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
        section: sectionNamed("access"),
        registration: registrationNamed("gateway-public"),
        environment: env.environments[2],
      }),
    ).toMatchObject({
      request: "plane-a-access",
      serviceAccount: "plane-a-management",
    });
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
    expect(templateData({ ...input, environment: edited }).parameters).toEqual({
      managedPolicyArns: ["arn:a"],
    });
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

function section(name: string, label: string): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: { appliesTo: ["CamundaClusterClaim"], label, selection: "One" },
  } as PluginSection;
}

function registration(
  name: string,
  spec: Partial<PluginRegistration["spec"]>,
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: [],
      mode: "Optional",
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...spec,
    },
  } as PluginRegistration;
}

function environmentWith(
  parameters: NonNullable<Environment["spec"]["parameters"]>,
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      sections: { "backup-storage": { allowed: ["s3"] } },
      parameters,
    },
  } as Environment;
}

describe("grantPreviews", () => {
  const backup = section("backup-storage", "Backup storage");
  const template = (fields: Record<string, string>) => ({
    name: "read",
    description: "Read the backups",
    format: "aws-iam-statement" as const,
    template: { Effect: "Allow", ...fields },
  });
  const s3 = registration("s3", {
    label: "S3 bucket",
    sections: ["backup-storage"],
    parameters: {
      type: "object",
      properties: {
        policy: { type: "string", title: "policy" },
        region: { type: "string" },
        tags: { type: "array" },
      },
    },
    outputs: [{ key: "bucketArn", description: "bucket ARN" }],
    grantTemplates: [
      template({
        Resource: "{{ .outputs.bucketArn }}",
        Policy: "{{ .parameters.policy }}",
        Region: "{{ .parameters.region }}",
      }),
    ],
  });
  const environment = environmentWith({
    s3: {
      policy: {
        claimEditable: true,
        allowed: [
          {
            name: "sqs",
            label: "SQS access",
            value: "arn:aws:iam::123456789012:policy/sqs",
          },
        ],
      },
      region: { value: "eu-west-1" },
    },
  });
  const claim = {
    kind: "CamundaClusterClaim" as const,
    namespace: "team-a",
    name: "orders",
    plugins: [
      {
        section: "backup-storage",
        registration: "s3",
        parameters: { policy: "sqs" },
      },
    ],
  };

  it("renders a picked choice by its label and an output as a placeholder", () => {
    const [preview] = grantPreviews({
      claim,
      sections: [backup],
      registrations: [s3],
      environment,
    });
    expect(preview).toMatchObject({
      section: "backup-storage",
      sectionLabel: "Backup storage",
      registration: "s3",
      optionLabel: "S3 bucket",
      template: "read",
      description: "Read the backups",
    });
    expect(preview.result).toEqual({
      value: {
        Effect: "Allow",
        Resource: "<bucket ARN from Backup storage>",
        Policy: "<policy: SQS access>",
        Region: "eu-west-1",
      },
    });
    expect(JSON.stringify(preview)).not.toContain("arn:");
  });

  it("renders the choice of the policy value by its label when the selection does not set it", () => {
    const fixed = environmentWith({
      s3: {
        policy: {
          value: "arn:aws:iam::123456789012:policy/sqs",
          allowed: [
            {
              name: "sqs",
              label: "SQS access",
              value: "arn:aws:iam::123456789012:policy/sqs",
            },
          ],
        },
        region: { value: "eu-west-1" },
      },
    });
    const [preview] = grantPreviews({
      claim: {
        ...claim,
        plugins: [{ section: "backup-storage", registration: "s3" }],
      },
      sections: [backup],
      registrations: [s3],
      environment: fixed,
    });
    expect(preview.result).toMatchObject({
      value: { Policy: "<policy: SQS access>" },
    });
  });

  it("shows an ARN that the admin set as set by the admin", () => {
    const locked = environmentWith({
      s3: {
        policy: { value: "arn:aws:iam::123456789012:policy/locked" },
        region: { value: "eu-west-1" },
      },
    });
    const [preview] = grantPreviews({
      claim: {
        ...claim,
        plugins: [{ section: "backup-storage", registration: "s3" }],
      },
      sections: [backup],
      registrations: [s3],
      environment: locked,
    });
    expect(preview.result).toMatchObject({
      value: { Policy: "<policy: Set by your admin>" },
    });
    expect(JSON.stringify(preview)).not.toContain("arn:");
  });

  it("shows an ARN that the selection sets as entered", () => {
    const open = environmentWith({
      s3: { policy: { claimEditable: true }, region: { value: "eu-west-1" } },
    });
    const [preview] = grantPreviews({
      claim: {
        ...claim,
        plugins: [
          {
            section: "backup-storage",
            registration: "s3",
            parameters: { policy: "arn:aws:iam::123456789012:policy/mine" },
          },
        ],
      },
      sections: [backup],
      registrations: [s3],
      environment: open,
    });
    expect(preview.result).toMatchObject({
      value: { Policy: "<policy: An ARN that you entered>" },
    });
  });

  it("returns the error of a template that cannot render, with its path", () => {
    const tagged = registration("s3", {
      ...s3.spec,
      grantTemplates: [template({ Tags: "{{ .parameters.tags }}" })],
    });
    const withTags = environmentWith({ s3: { tags: { value: ["a", "b"] } } });
    const [preview] = grantPreviews({
      claim,
      sections: [backup],
      registrations: [tagged],
      environment: withTags,
    });
    expect(preview.result).toMatchObject({
      path: "Tags",
      field: "parameters.tags",
    });
  });

  it("renders without an Environment", () => {
    const valued = registration("s3", {
      ...s3.spec,
      parameterValues: {
        policy: "arn:aws:iam::123456789012:policy/default",
        region: "eu-central-1",
      },
    });
    const [preview] = grantPreviews({
      claim: {
        ...claim,
        plugins: [{ section: "backup-storage", registration: "s3" }],
      },
      sections: [backup],
      registrations: [valued],
      environment: undefined,
    });
    expect(preview.result).toEqual({
      value: {
        Effect: "Allow",
        Resource: "<bucket ARN from Backup storage>",
        Policy: "<policy: Set by your admin>",
        Region: "eu-central-1",
      },
    });
  });

  it("gives one preview for each template of each selection, in order", () => {
    const two = registration("s3", {
      ...s3.spec,
      grantTemplates: [
        { ...template({ Region: "{{ .parameters.region }}" }), name: "a" },
        { ...template({ Region: "{{ .parameters.region }}" }), name: "b" },
      ],
    });
    expect(
      grantPreviews({
        claim,
        sections: [backup],
        registrations: [two],
        environment,
      }).map((p) => p.template),
    ).toEqual(["a", "b"]);
  });

  it("gives nothing for a selection whose registration or section is missing", () => {
    expect(
      grantPreviews({
        claim,
        sections: [backup],
        registrations: [],
        environment,
      }),
    ).toEqual([]);
    expect(
      grantPreviews({ claim, sections: [], registrations: [s3], environment }),
    ).toEqual([]);
  });
});
