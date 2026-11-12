import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import type { EnvironmentSources } from "./sources";

const SINCE = "2026-11-10T09:00:00Z";
const FLEET = "fleet.camunda.io/v1alpha1";
const PLUGINS = "plugins.camunda.io/v1alpha1";

function ready(status: "True" | "False", reason: string, message = "") {
  return {
    type: "Ready",
    status,
    reason,
    message,
    lastTransitionTime: SINCE,
  };
}

function environment(
  name: string,
  spec: Environment["spec"],
  status?: Environment["status"],
): Environment {
  return {
    apiVersion: FLEET,
    kind: "Environment",
    metadata: { name, resourceVersion: "12" },
    spec,
    ...(status && { status }),
  };
}

function namespace(name: string, env?: string): V1Namespace {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name,
      resourceVersion: "7",
      ...(env && { labels: { [ENVIRONMENT_LABEL]: env } }),
    },
  };
}

function section(
  name: string,
  label: string,
  appliesTo: PluginSection["spec"]["appliesTo"],
  order: number,
  extra: Partial<PluginSection["spec"]> = {},
): PluginSection {
  return {
    apiVersion: PLUGINS,
    kind: "PluginSection",
    metadata: { name },
    spec: { label, appliesTo, order, selection: "One", ...extra },
  };
}

const ACCEPTED = { ...ready("True", "Accepted"), type: "Accepted" };

function registration(
  name: string,
  label: string,
  sections: string[],
  extra: Partial<PluginRegistration["spec"]> = {},
  conditions: NonNullable<PluginRegistration["status"]>["conditions"] = [
    ACCEPTED,
    ready("True", "Ready"),
  ],
): PluginRegistration {
  return {
    apiVersion: PLUGINS,
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      label,
      sections,
      mode: "Optional",
      deletionPolicies: ["Delete", "Orphan"],
      serviceAccount: { name: "camunda-operator", namespace: "camunda-system" },
      ...extra,
    },
    status: { conditions },
  };
}

const POLICY_ARN = "^arn:aws[a-z-]*:iam::([0-9]{12}|aws):policy/.+$";

// The parameter schema of the chart charts/aws-connectors-role of the
// plugins repository.
const CONNECTORS_ROLE_PARAMETERS = {
  type: "object",
  properties: {
    managedPolicyArns: {
      type: "array",
      "x-kubernetes-list-type": "set",
      maxItems: 10,
      items: { type: "string", pattern: POLICY_ARN },
      description:
        "Managed policies that every cluster of the environment gets.",
    },
    extraPolicyArns: {
      type: "array",
      "x-kubernetes-list-type": "set",
      maxItems: 10,
      items: { type: "string", pattern: POLICY_ARN },
      description:
        "Managed policies that a claim can add from the allowed values of its environment.",
    },
    inlinePolicy: {
      type: "object",
      "x-kubernetes-preserve-unknown-fields": true,
      "x-fleet-template": true,
      description:
        "One IAM policy document. Its placeholders scope it to one cluster.",
    },
  },
};

function clusterClaim(
  ns: string,
  name: string,
  spec: Partial<CamundaClusterClaim["spec"]>,
): CamundaClusterClaim {
  return {
    apiVersion: FLEET,
    kind: "CamundaClusterClaim",
    metadata: { namespace: ns, name },
    spec: { presetRef: "small", ...spec },
    status: { resolvedPolicy: { deletionPolicy: "Delete" } },
  };
}

/**
 * environmentFixtures returns the sources of the Environments page in the
 * stories and the component tests: the environments production, staging
 * (no status yet), and platform (the plane sections access and
 * management-database), the namespaces legacy (a cluster without an
 * environment) and sandbox (empty, without an environment), the objects
 * they name, and the core sections with their registrations: aws-kms is
 * not ready, vault-transit is not accepted, and aws-ebs-encrypted needs an
 * encryption key.
 */
export function environmentFixtures(): EnvironmentSources {
  return {
    environments: [
      environment(
        "production",
        {
          displayName: "Production",
          color: "#C62828",
          platformConfigRef: "shared",
          presets: { camundaCluster: ["small", "large"] },
          releases: { allowed: ["8.9.22", "8.9.23"], default: "8.9.23" },
          deletionPolicy: "Delete",
        },
        { namespaces: 2, conditions: [ready("True", "Ready")] },
      ),
      environment("staging", {
        displayName: "Staging",
        color: "#F9A825",
        presets: { camundaCluster: ["small"] },
        releases: { allowed: ["8.9.23"], default: "8.9.23" },
        namespaceCreation: {
          allowed: true,
          labels: { team: "payments" },
          requiredLabelKeys: ["cost-center"],
        },
      }),
      environment(
        "platform",
        {
          displayName: "Platform",
          color: "#1565C0",
          platformConfigRef: "shared",
          sections: {
            access: {
              allowed: ["gateway-internal"],
              default: "gateway-internal",
            },
            "management-database": {
              required: true,
              allowed: ["management-database-server"],
              default: "management-database-server",
            },
          },
          deletionPolicy: "Orphan",
        },
        { namespaces: 1, conditions: [ready("True", "Ready")] },
      ),
    ],
    namespaces: [
      namespace("team-a", "production"),
      namespace("team-b", "production"),
      namespace("team-c", "staging"),
      namespace("platform", "platform"),
      namespace("legacy"),
      namespace("sandbox"),
    ],
    clusterClaims: [
      clusterClaim("team-a", "orders", { releaseRef: "8.9.22" }),
      clusterClaim("team-a", "billing", {
        presetRef: "large",
        releaseRef: "8.9.23",
      }),
    ],
    planeClaims: [
      {
        apiVersion: FLEET,
        kind: "CamundaManagementClusterClaim",
        metadata: { namespace: "platform", name: "plane-a" },
        spec: { environments: ["production", "staging"] },
        status: { resolvedPolicy: { deletionPolicy: "Orphan" } },
      } as unknown as CamundaManagementClusterClaim,
    ],
    clusters: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaCluster",
        metadata: { namespace: "legacy", name: "old-orders" },
        spec: {},
      } as CamundaCluster,
    ],
    planes: [],
    sections: [
      section("encryption-key", "Encryption key", ["CamundaClusterClaim"], 2, {
        emptyOptionLabel: "No key",
      }),
      section(
        "volume-encryption",
        "Volume encryption",
        ["CamundaClusterClaim"],
        3,
        { emptyOptionLabel: "Unencrypted volumes" },
      ),
      section(
        "access",
        "Access",
        ["CamundaClusterClaim", "CamundaManagementClusterClaim"],
        10,
        { emptyOptionLabel: "No access from outside" },
      ),
      section(
        "management-database",
        "Management database",
        ["CamundaManagementClusterClaim"],
        20,
      ),
      section("backup-storage", "Backup storage", ["CamundaClusterClaim"], 30, {
        emptyOptionLabel: "No backups",
      }),
      section(
        "connectors-permissions",
        "Connectors permissions",
        ["CamundaClusterClaim"],
        40,
        {
          emptyOptionLabel: "No cloud permissions",
          slot: "connectorsIdentity",
        },
      ),
    ],
    registrations: [
      registration("gateway-internal", "Internal gateway", ["access"]),
      registration("gateway-public", "Public gateway", ["access"], {
        parameters: {
          type: "object",
          properties: {
            hostnamePattern: {
              type: "string",
              title: "Hostname pattern",
              description: "The host of the cluster. {name} is the claim.",
            },
          },
        },
        parameterValues: { hostnamePattern: "{name}.{namespace}.example.com" },
      }),
      registration("management-database-server", "PostgreSQL in the cluster", [
        "management-database",
      ]),
      registration("rustfs", "RustFS", ["backup-storage"]),
      registration("aws-kms", "AWS KMS key", ["encryption-key"], {}, [
        ACCEPTED,
        ready("False", "AccessDenied", "AccessDenied on the key."),
      ]),
      registration(
        "vault-transit",
        "Vault transit key",
        ["encryption-key"],
        {},
        [{ ...ready("False", "Invalid"), type: "Accepted" }],
      ),
      registration(
        "aws-ebs-encrypted",
        "Encrypted EBS volumes",
        ["volume-encryption"],
        {
          inputs: [
            { name: "keyArn", from: "encryption-key/keyRef", required: true },
          ],
        },
      ),
      registration(
        "aws-connectors-role",
        "Role for each cluster",
        ["connectors-permissions"],
        {
          parameters: CONNECTORS_ROLE_PARAMETERS,
          limits: [
            {
              label: "Permissions boundary",
              value: "arn:aws:iam::123456789012:policy/camunda-boundary",
              description:
                "The managed policy that bounds the role of the Connectors of each cluster.",
            },
          ],
        },
      ),
    ],
    requests: [],
    clusterPresets: (["small", "large"] as const).map(
      (name): CamundaClusterPreset =>
        ({
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaClusterPreset",
          metadata: { name },
          spec: {},
        }) as CamundaClusterPreset,
    ),
    elasticsearchPresets: [],
    databasePresets: [],
    releases: ["8.9.22", "8.9.23"].map(
      (name): CamundaRelease =>
        ({
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaRelease",
          metadata: { name },
          spec: { version: name },
        }) as CamundaRelease,
    ),
    platformConfigs: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaPlatformConfig",
        metadata: { name: "shared" },
        spec: {},
      } as CamundaPlatformConfig,
    ],
  };
}

/**
 * sectionFixtures returns environmentFixtures with sections and parameters
 * in production: an encryption key without a default, encrypted volumes,
 * required backups, and the Connectors role with an extra policy that a
 * claim can pick and an inline policy template.
 */
export function sectionFixtures(): EnvironmentSources {
  const sources = environmentFixtures();
  sources.environments[0].spec.sections = {
    "encryption-key": { allowed: ["aws-kms"] },
    "volume-encryption": { allowed: ["aws-ebs-encrypted"] },
    "backup-storage": {
      required: true,
      allowed: ["rustfs"],
      default: "rustfs",
    },
    "connectors-permissions": {
      allowed: ["aws-connectors-role"],
      default: "aws-connectors-role",
    },
  };
  sources.environments[0].spec.parameters = {
    "aws-connectors-role": {
      extraPolicyArns: {
        claimEditable: true,
        allowed: [
          {
            name: "sqs",
            label: "SQS access",
            value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess",
          },
        ],
      },
      inlinePolicy: {
        value: {
          Version: "2012-10-17",
          Statement: [
            {
              Effect: "Allow",
              Action: "sqs:*",
              Resource: "arn:aws:sqs:*:*:{{ .namespace }}-*",
            },
          ],
        },
      },
    },
  };
  return sources;
}
