// Objects of a cluster claim with one selection in each core section, in
// the shape that the API server returns them. The tests and stories of the
// request detail read them through requestDetail.
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { type SectionClaim, sectionComponents } from "@/lib/claims/sections";
import type { Resource } from "@/lib/k8s/kinds";
import {
  BUILTIN_CONTROLLER,
  CLAIM_LABEL,
  REGISTRATION_LABEL,
  REQUEST_LABEL,
  SECTION_LABEL,
} from "@/lib/labels";

/** requestNow is the time of the fixtures. */
export const requestNow = "2026-11-16T12:00:00Z";

const SINCE = "2026-11-16T11:40:00Z";
const NAMESPACE = "team-a";
const CLAIM = "orders";
const CLAIM_UID = "uid-claim-orders";
const PLUGINS = "plugins.camunda.io/v1alpha1";

/** fixtureCondition returns a condition that changed 20 minutes before requestNow. */
export function fixtureCondition(
  type: string,
  status: "True" | "False" | "Unknown",
  reason: string,
  message = "",
): {
  type: string;
  status: "True" | "False" | "Unknown";
  reason: string;
  message: string;
  lastTransitionTime: string;
} {
  return { type, status, reason, message, lastTransitionTime: SINCE };
}

const ready = fixtureCondition("Ready", "True", "Healthy");
const outputsReady = fixtureCondition("OutputsReady", "True", "Healthy");

function section(
  name: string,
  label: string,
  order: number,
  slot?: Resource<"PluginSection">["spec"]["slot"],
): Resource<"PluginSection"> {
  return {
    apiVersion: PLUGINS,
    kind: "PluginSection",
    metadata: { name },
    spec: {
      label,
      order,
      selection: "One",
      appliesTo: ["CamundaClusterClaim"],
      ...(slot && { slot }),
    },
  } as Resource<"PluginSection">;
}

/** coreSections are the core sections of a cluster claim. */
export const coreSections: Resource<"PluginSection">[] = [
  section("cloud-identity", "Cloud identity", 10, "workloadIdentity"),
  section("encryption-key", "Encryption key", 20),
  section("volume-encryption", "Volume encryption", 30, "volumeEncryption"),
  section("backup-storage", "Backup storage", 40, "backupStorage"),
  section("secondary-storage", "Secondary storage", 50, "secondaryStorage"),
  section("access", "Access", 60, "externalAccess"),
  section(
    "connectors-permissions",
    "Connectors permissions",
    70,
    "connectorsIdentity",
  ),
];

function registration(
  name: string,
  label: string,
  sections: string[],
  over: Partial<Resource<"PluginRegistration">["spec"]> = {},
): Resource<"PluginRegistration"> {
  return {
    apiVersion: PLUGINS,
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      label,
      sections,
      mode: "Optional",
      deletionPolicies: ["Delete", "Orphan"],
      serviceAccount: { name, namespace: "camunda-plugins" },
      ...over,
    },
  } as Resource<"PluginRegistration">;
}

const keyInput = { name: "keyRef", from: "encryption-key/keyRef" };

// readObjects is the grant template of the Grants of fixtureGrant.
const readObjects = {
  name: "read-objects",
  format: "aws-iam-statement" as const,
  template: {
    Effect: "Allow",
    Action: ["s3:GetObject"],
    Resource: "arn:aws:s3:::{{ .Inputs.bucket }}/*",
  },
};

/** fixtureRegistrations are the registrations of the selections of the fixture claim. */
export const fixtureRegistrations: Resource<"PluginRegistration">[] = [
  registration("aws-iam-role", "AWS IAM role", ["cloud-identity"]),
  registration("aws-kms-key", "AWS KMS key", ["encryption-key"], {
    grantTemplates: [readObjects],
  }),
  registration("aws-ebs", "Amazon EBS", ["volume-encryption"], {
    inputs: [{ ...keyInput, required: true }],
  }),
  registration("aws-s3", "Amazon S3", ["backup-storage"], {
    inputs: [keyInput],
    grantTemplates: [readObjects],
  }),
  registration(
    "existing-secondary-storage",
    "Existing secondary storage",
    ["secondary-storage"],
    { controller: { name: BUILTIN_CONTROLLER } },
  ),
  registration("elasticsearch", "Elasticsearch", ["secondary-storage"], {
    controller: { name: BUILTIN_CONTROLLER },
  }),
  registration("gateway-access", "Gateway access", ["access"]),
  registration("aws-connectors-role", "AWS Connectors role", [
    "connectors-permissions",
  ]),
];

type Selection = {
  section: string;
  registration: string;
  parameters?: Record<string, unknown>;
};

/** fixtureSelections are the selections of the fixture claim. */
export const fixtureSelections: Selection[] = [
  { section: "cloud-identity", registration: "aws-iam-role" },
  { section: "encryption-key", registration: "aws-kms-key" },
  { section: "volume-encryption", registration: "aws-ebs" },
  { section: "backup-storage", registration: "aws-s3" },
  {
    section: "secondary-storage",
    registration: "existing-secondary-storage",
    parameters: { name: "orders-storage" },
  },
  { section: "access", registration: "gateway-access" },
  { section: "connectors-permissions", registration: "aws-connectors-role" },
];

/** requestNameOf is the name of the request of the fixture claim in section. */
export function requestNameOf(section: string): string {
  return `${CLAIM}-${section}`;
}

/** requestUidOf is the UID of the request of the fixture claim in section. */
export function requestUidOf(section: string): string {
  return `uid-${requestNameOf(section)}`;
}

/** fixtureClaim returns the fixture claim with selections, under the deletion policy of spec when it has one. */
export function fixtureClaim(
  selections: readonly Selection[] = fixtureSelections,
  deletionPolicy?: "Delete" | "Orphan",
): Resource<"CamundaClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: NAMESPACE, name: CLAIM, uid: CLAIM_UID },
    spec: {
      ...(deletionPolicy && { deletionPolicy }),
      plugins: selections,
    },
    status: {
      requests: selections.map((s) => ({
        section: s.section,
        registration: s.registration,
        name: requestNameOf(s.section),
      })),
    },
  } as unknown as Resource<"CamundaClusterClaim">;
}

type RequestStatus = NonNullable<Resource<"PluginRequest">["status"]>;

/**
 * fixtureRequest returns the request of the fixture claim in section, which
 * the claim controls, with status. A Ready condition of status replaces the
 * Healthy one.
 */
export function fixtureRequest(
  section: string,
  status: Partial<RequestStatus> = {},
  inputs?: Record<string, string>,
): Resource<"PluginRequest"> {
  const selection = fixtureSelections.find((s) => s.section === section);
  const registration = selection?.registration ?? "";
  return {
    apiVersion: PLUGINS,
    kind: "PluginRequest",
    metadata: {
      namespace: NAMESPACE,
      name: requestNameOf(section),
      uid: requestUidOf(section),
      labels: {
        [CLAIM_LABEL]: CLAIM,
        [SECTION_LABEL]: section,
        [REGISTRATION_LABEL]: registration,
      },
      ownerReferences: [
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "CamundaClusterClaim",
          name: CLAIM,
          uid: CLAIM_UID,
          controller: true,
        },
      ],
    },
    spec: {
      claimRef: { kind: "CamundaClusterClaim", name: CLAIM },
      deletionPolicy: "Delete",
      registration,
      section,
      target: {
        kind: "CamundaCluster",
        name: CLAIM,
        serviceAccountName: `${CLAIM}-camunda`,
      },
      ...(selection?.parameters && { parameters: selection.parameters }),
      ...(inputs && { inputs }),
    },
    status: {
      summaryTime: SINCE,
      ...status,
      conditions: status.conditions ?? [outputsReady, ready],
    },
  } as Resource<"PluginRequest">;
}

/**
 * fixtureRequests are the requests of the fixture claim, each Ready, with
 * the summary, outputs, and resources that its plugin writes.
 */
export const fixtureRequests: Resource<"PluginRequest">[] = [
  fixtureRequest("cloud-identity", {
    summary: [{ label: "Role", value: "orders-camunda", severity: "Info" }],
    outputs: {
      principal: "arn:aws:iam::1:role/orders-camunda",
      serviceAccountAnnotations:
        '{"eks.amazonaws.com/role-arn":"arn:aws:iam::1:role/orders-camunda"}',
    },
    resources: [
      {
        kind: "Role",
        name: "orders-camunda",
        id: "arn:aws:iam::1:role/orders-camunda",
        location: "aws-global",
        role: "Identity of the cluster",
        onOrphan: "Kept",
      },
    ],
  }),
  fixtureRequest("encryption-key", {
    summary: [
      { label: "Key", value: "alias/orders", severity: "Info" },
      { label: "State", value: "Enabled" },
      { label: "Rotation", value: "Off", severity: "Warning" },
    ],
    outputs: {
      keyRef: "arn:aws:kms:eu-west-1:1:key/1234",
      format: "aws-kms-arn",
    },
    resources: [
      {
        kind: "Key",
        name: "alias/orders",
        id: "arn:aws:kms:eu-west-1:1:key/1234",
        location: "eu-west-1",
        onOrphan: "Kept",
      },
    ],
  }),
  fixtureRequest(
    "volume-encryption",
    {
      summary: [{ label: "StorageClass", value: "orders-encrypted" }],
      outputs: { storageClassName: "orders-encrypted" },
      resources: [
        {
          kind: "StorageClass",
          name: "orders-encrypted",
          role: "Zeebe volumes",
          onOrphan: "Kept",
        },
      ],
    },
    { keyRef: "arn:aws:kms:eu-west-1:1:key/1234" },
  ),
  fixtureRequest(
    "backup-storage",
    {
      summary: [{ label: "Bucket", value: "orders-backups" }],
      outputs: { objectStorageConfig: "orders-backups" },
      resources: [
        {
          kind: "Bucket",
          name: "orders-backups",
          location: "eu-west-1",
          onOrphan: "Kept",
        },
        {
          kind: "ObjectStorageConfig",
          name: "orders-backups",
          onOrphan: "Deleted",
        },
      ],
    },
    { keyRef: "arn:aws:kms:eu-west-1:1:key/1234" },
  ),
  fixtureRequest("secondary-storage", {
    summary: [
      {
        label: "SecondaryStorageConfig",
        value: "orders-storage, Ready",
        severity: "Info",
      },
    ],
    outputs: { secondaryStorageConfig: "orders-storage" },
  }),
  fixtureRequest("access", {
    summary: [{ label: "Host", value: "orders.example.com" }],
    outputs: { externalUrl: "https://orders.example.com" },
    links: [{ name: "Operate", url: "https://orders.example.com/operate" }],
    resources: [{ kind: "HTTPRoute", name: "orders", onOrphan: "Deleted" }],
  }),
  fixtureRequest("connectors-permissions", {
    summary: [
      { label: "Role", value: "orders-connectors" },
      { label: "Ceiling", value: "s3:GetObject, sqs:SendMessage" },
    ],
    outputs: {
      principal: "arn:aws:iam::1:role/orders-connectors",
      serviceAccountAnnotations:
        '{"eks.amazonaws.com/role-arn":"arn:aws:iam::1:role/orders-connectors"}',
      attachedPolicies: "orders-connectors-s3",
    },
    resources: [{ kind: "Role", name: "orders-connectors", onOrphan: "Kept" }],
  }),
];

/**
 * fixtureGrant returns a Grant of the fixture cluster and of the template
 * read-objects that the request of section owns, with conditions as its
 * status. Without conditions it has no status.
 */
export function fixtureGrant(
  name: string,
  section: string,
  conditions?: ConditionRow[],
): Resource<"Grant"> {
  return {
    apiVersion: PLUGINS,
    kind: "Grant",
    metadata: {
      namespace: NAMESPACE,
      name,
      labels: {
        [REQUEST_LABEL]: requestNameOf(section),
        [REGISTRATION_LABEL]:
          fixtureSelections.find((s) => s.section === section)?.registration ??
          "",
      },
      ownerReferences: [
        {
          apiVersion: PLUGINS,
          kind: "PluginRequest",
          name: requestNameOf(section),
          uid: requestUidOf(section),
          controller: true,
        },
      ],
    },
    spec: {
      cluster: { name: CLAIM },
      template: "read-objects",
      format: "aws-iam-statement",
      fragment: {
        Effect: "Allow",
        Action: ["s3:GetObject"],
        Resource: "arn:aws:s3:::orders-backups/*",
      },
    },
    ...(conditions && { status: { conditions } }),
  } as Resource<"Grant">;
}

/** fixtureComponents returns the section components of the fixture claim over requests. */
export function fixtureComponents(
  claim: Resource<"CamundaClusterClaim"> = fixtureClaim(),
  requests: readonly Resource<"PluginRequest">[] = fixtureRequests,
) {
  return sectionComponents({
    claim: claim as SectionClaim,
    sections: coreSections,
    requests,
  });
}

/** fixtureContract returns a SecondaryStorageConfig of the fixture namespace with Ready. */
export function fixtureContract(
  name: string,
  readyStatus: "True" | "False" = "True",
): Resource<"SecondaryStorageConfig"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "SecondaryStorageConfig",
    metadata: { namespace: NAMESPACE, name },
    spec: { type: "elasticsearch" },
    status: {
      conditions: [
        fixtureCondition(
          "Ready",
          readyStatus,
          readyStatus === "True" ? "Healthy" : "Unreachable",
        ),
      ],
    },
  } as unknown as Resource<"SecondaryStorageConfig">;
}
