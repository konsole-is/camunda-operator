import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import { deleteClaimSteps } from "@/lib/claims/deletion";
import type { Resource } from "@/lib/k8s/kinds";
import type { DeletionPolicy } from "@/lib/policy";
import type {
  Change,
  ForeignManager,
  ObjectOwnership,
  Preview,
} from "@/lib/write/types";
import {
  type DeleteClusterData,
  deleteClusterModel,
  deleteProgress,
} from "./delete-cluster-model";
import { DeleteClusterView } from "./delete-cluster-view";

type Kind =
  | "CamundaClusterClaim"
  | "CamundaCluster"
  | "CamundaOptimize"
  | "BackupSchedule"
  | "LogicalBackupElasticsearch"
  | "PluginSection"
  | "PluginRegistration"
  | "PluginRequest";

// The stories build their objects as plain JSON, as the cache gives them.
const object = <K extends Kind>(kind: K, value: object) =>
  value as unknown as Resource<K>;

const CLAIM_UID = "8c1f6c1e-1d1a-4c5e-9d43-0f3b1c2d4e5f";
// controlled is the owner reference of an object that the claim made.
const controlled = {
  ownerReferences: [{ controller: true, uid: CLAIM_UID }],
};

const selections = [
  { section: "access", registration: "gateway" },
  { section: "backup-storage", registration: "aws-s3" },
  { section: "secondary-storage", registration: "elasticsearch" },
];

const claim = (deletionPolicy?: DeletionPolicy, status: object = {}) =>
  object("CamundaClusterClaim", {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: {
      namespace: "team-orders",
      name: "orders",
      resourceVersion: "41",
      uid: CLAIM_UID,
    },
    spec: { releaseRef: "camunda-8-9-1", plugins: selections, deletionPolicy },
    status: {
      resolvedPolicy: { deletionPolicy: "Orphan" },
      requests: selections.map((s) => ({ ...s, name: `orders-${s.section}` })),
      cluster: { name: "orders" },
      ...status,
    },
  });

const section = (name: string, label: string, order: number, slot?: string) =>
  object("PluginSection", {
    metadata: { name },
    spec: {
      label,
      order,
      selection: "One",
      appliesTo: ["CamundaClusterClaim"],
      ...(slot && { slot }),
    },
  });

const registration = (
  name: string,
  label: string,
  sections: string[],
  deletionPolicies: DeletionPolicy[],
) =>
  object("PluginRegistration", {
    metadata: { name },
    spec: { label, sections, deletionPolicies },
  });

const request = (name: string, resources: object[]) =>
  object("PluginRequest", {
    metadata: { namespace: "team-orders", name, ...controlled },
    status: { resources },
  });

const of = (cluster: string) => ({ spec: { clusterRef: { name: cluster } } });

// reconciled is the status of a cluster whose generation 4 the operator
// applied.
const reconciled = (status = "True", reason = "Healthy") => ({
  observedGeneration: 4,
  conditions: [
    { type: "Ready", status, reason, observedGeneration: 4 },
    {
      type: "ZeebeReady",
      status: "True",
      reason: "Healthy",
      observedGeneration: 4,
    },
  ],
});
const retaining = (status: object) =>
  object("CamundaCluster", {
    metadata: {
      namespace: "team-orders",
      name: "orders",
      generation: 4,
      ...controlled,
    },
    spec: {
      zeebe: {
        persistentVolumeClaimRetentionPolicy: { whenDeleted: "Retain" },
      },
    },
    status,
  });

const data = (over: Partial<DeleteClusterData> = {}): DeleteClusterData => ({
  claim: claim(),
  cluster: retaining(reconciled()),
  environments: [],
  sections: [
    section("access", "Access", 10, "externalAccess"),
    section("backup-storage", "Backup storage", 20, "backupStorage"),
    section("secondary-storage", "Secondary storage", 30, "secondaryStorage"),
  ],
  registrations: [
    registration("gateway", "Gateway", ["access"], ["Delete", "Orphan"]),
    registration(
      "aws-s3",
      "Amazon S3",
      ["backup-storage"],
      ["Delete", "Orphan"],
    ),
    registration(
      "elasticsearch",
      "Elasticsearch",
      ["secondary-storage"],
      ["Delete", "Orphan"],
    ),
  ],
  requests: [
    request("orders-access", [
      { kind: "HTTPRoute", name: "orders", onOrphan: "Deleted" },
      { kind: "Certificate", name: "orders-tls", onOrphan: "Kept" },
    ]),
    request("orders-backup-storage", [
      {
        kind: "S3Bucket",
        name: "orders-backups-7f3a",
        id: "arn:aws:s3:::orders-backups-7f3a",
        onOrphan: "Kept",
      },
    ]),
    request("orders-secondary-storage", [
      { kind: "ElasticsearchCluster", name: "orders-es", onOrphan: "Kept" },
    ]),
  ],
  optimizes: [
    object("CamundaOptimize", {
      metadata: {
        namespace: "team-orders",
        name: "orders-optimize",
        ...controlled,
      },
      ...of("orders"),
    }),
  ],
  schedules: [
    object("BackupSchedule", {
      metadata: {
        namespace: "team-orders",
        name: "orders-backups",
        ...controlled,
      },
      ...of("orders"),
    }),
    object("BackupSchedule", {
      metadata: { namespace: "team-orders", name: "orders-hourly" },
      ...of("orders"),
    }),
  ],
  backups: {
    elasticsearch: ["orders-1", "orders-2", "orders-3"].map((name) =>
      object("LogicalBackupElasticsearch", {
        metadata: { namespace: "team-orders", name },
        ...of("orders"),
      }),
    ),
    rdbms: [],
  },
  presets: [],
  releases: [],
  errors: [],
  ...over,
});

const refused = data({
  claim: claim("Orphan"),
  registrations: [
    registration("gateway", "Gateway", ["access"], ["Delete"]),
    registration(
      "aws-s3",
      "Amazon S3",
      ["backup-storage"],
      ["Delete", "Orphan"],
    ),
    registration(
      "elasticsearch",
      "Elasticsearch",
      ["secondary-storage"],
      ["Delete", "Orphan"],
    ),
  ],
});

const keep = deleteClusterModel(data(), "Orphan");
const remove = deleteClaimSteps(
  data().claim,
  "Orphan",
  "Delete the cluster orders",
);

const meta = {
  title: "cluster/DeleteClusterView",
  component: DeleteClusterView,
  args: {
    model: keep,
    mode: "direct",
    stage: { kind: "choose" },
    errors: [],
    remove: remove.remove,
    policyStep: false,
    onPick: fn(),
    preview: fn(async () => ({ objects: [], errors: [] })),
    apply: fn(async () => ({ ok: true as const, mode: "direct" as const })),
    show: fn(),
    onWritten: fn(),
    onStopWaiting: fn(),
  },
  decorators: [
    (Story) => (
      <div className="max-w-3xl">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof DeleteClusterView>;
export default meta;
type Story = StoryObj<typeof meta>;

const progress = deleteProgress(keep.targets, {
  CamundaOptimize: [],
  BackupSchedule: [],
  CamundaCluster: [
    {
      metadata: { name: "orders", deletionTimestamp: "2026-11-15T10:00:00Z" },
    },
  ],
  PluginRequest: [
    { metadata: { name: "orders-access" } },
    { metadata: { name: "orders-backup-storage" } },
    { metadata: { name: "orders-secondary-storage" } },
  ],
  CamundaClusterClaim: [
    {
      metadata: { name: "orders", deletionTimestamp: "2026-11-15T10:00:00Z" },
    },
  ],
});

export const KeepTheResources: Story = {
  name: "Keep the resources",
};

export const DeleteTheResources: Story = {
  name: "Delete the resources, with the bucket warning",
  args: { model: deleteClusterModel(data(), "Delete"), policyStep: true },
};

export const KeepRefused: Story = {
  name: "A claim on Keep with an option that cannot keep",
  args: { model: deleteClusterModel(refused, "Delete") },
};

export const RequestsNotRead: Story = {
  name: "The requests could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["PluginRequest"]),
    errors: [
      {
        kind: "PluginRequest",
        message: "pluginrequests.plugins.camunda.io is forbidden",
      },
    ],
  },
};

export const PolicyWritten: Story = {
  name: "Git: the delete was refused after the policy commit",
  args: {
    mode: "git",
    stage: {
      kind: "choose",
      errors: [
        {
          code: "Stale",
          message: "The claim orders changed since you opened it.",
        },
      ],
      afterPolicyWrite: { held: "Delete", unapplied: "Orphan" },
    },
  },
};

export const KeepPickRefused: Story = {
  name: "Keep picked, and an option cannot keep",
  args: {
    model: deleteClusterModel(refused, "Orphan"),
    keepRefused:
      'PluginRegistration "gateway" of section "access" cannot keep the resources. Choose Delete the resources.',
  },
};

export const VolumesNotApplied: Story = {
  name: "The Ready condition of the cluster reports a problem",
  args: {
    model: deleteClusterModel(
      data({
        cluster: retaining(reconciled("False", "InvalidReference")),
      }),
      "Orphan",
    ),
  },
};

export const AttachmentsNotRead: Story = {
  name: "The Optimize objects could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["CamundaOptimize"]),
    errors: [
      {
        kind: "CamundaOptimize",
        message: "camundaoptimizes.core.camunda.io is forbidden",
      },
    ],
  },
};

const notMade = data({
  claim: claim(undefined, { cluster: undefined }),
  cluster: object("CamundaCluster", {
    metadata: { namespace: "team-orders", name: "orders" },
  }),
});

export const ClusterNotMade: Story = {
  name: "A cluster that the claim did not make",
  args: { model: deleteClusterModel(notMade, "Orphan") },
};

export const ClusterNotMadeDeleted: Story = {
  name: "The claim is deleted, and its cluster stays",
  args: {
    model: deleteClusterModel(notMade, "Orphan"),
    stage: { kind: "deleted" },
  },
};

export const DirectPolicyWait: Story = {
  name: "Waiting for the operator to read the deletion policy",
  args: { stage: { kind: "waiting" } },
};

export const DirectProgress: Story = {
  name: "The cluster and its requests going",
  args: { stage: { kind: "deleting", progress, gitWaiting: false } },
};

export const DeleteKeepRefused: Story = {
  name: "The operator refuses the delete, because an option cannot keep",
  args: {
    stage: {
      kind: "deleting",
      progress,
      gitWaiting: false,
      keepRefused:
        'The delete waits, because PluginRegistration "gateway" of section "access" cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.',
    },
  },
};

export const GitPolicyWait: Story = {
  name: "Git: waiting for the deletion policy",
  args: { mode: "git", stage: { kind: "waiting" } },
};

export const GitDeleteWait: Story = {
  name: "Git: waiting for GitOps to delete the claim",
  args: {
    mode: "git",
    stage: {
      kind: "deleting",
      progress: deleteProgress(keep.targets, {
        CamundaOptimize: [{ metadata: { name: "orders-optimize" } }],
        BackupSchedule: [{ metadata: { name: "orders-backups" } }],
        CamundaCluster: [{ metadata: { name: "orders" } }],
        PluginRequest: [
          { metadata: { name: "orders-access" } },
          { metadata: { name: "orders-backup-storage" } },
          { metadata: { name: "orders-secondary-storage" } },
        ],
        CamundaClusterClaim: [{ metadata: { name: "orders" } }],
      }),
      gitWaiting: true,
    },
  },
};

export const Deleted: Story = {
  name: "The cluster is deleted",
  args: { stage: { kind: "deleted" } },
};

export const Replaced: Story = {
  name: "The claim was made again",
  args: { stage: { kind: "replaced" } },
};

export const ChangesOff: Story = {
  name: "Changes are off",
  args: { mode: undefined },
};

// ownedPreview is the preview of a delete whose policy write changes
// spec.deletionPolicy, which manager set.
const ownedPreview = (manager: ForeignManager): Preview => {
  const ref = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    namespace: "team-orders",
    name: "orders",
  };
  return {
    objects: [],
    errors: [
      {
        code: "Conflict",
        message: `conflict with "${manager.name}"`,
        path: "spec.deletionPolicy",
        manager: manager.name,
        ref,
      },
    ],
    ownership: [
      {
        ref,
        changed: [
          {
            path: "spec.deletionPolicy",
            steps: [{ field: "spec" }, { field: "deletionPolicy" }],
            managers: [manager],
          },
        ],
        cleared: [],
      },
    ],
  };
};

export const PolicyOfAnotherTool: Story = {
  name: "Keep picked, and kubectl set the deletion policy",
  args: {
    model: deleteClusterModel(data({ claim: claim("Delete") }), "Orphan"),
    policyStep: true,
    policyOwned:
      "Another tool set the deletion policy of the claim, so the delete cannot change it without the take-over. Take over the field below, or choose Delete the resources to delete with the policy that the claim has.",
    // With the take-over, the dry run of the policy write has no conflict.
    preview: fn(async (_change: Change, taken?: readonly ObjectOwnership[]) =>
      taken?.length
        ? { objects: [], errors: [] }
        : ownedPreview({
            name: "kubectl-client-side-apply",
            operation: "Update",
            kind: "other",
            known: true,
          }),
    ),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const group = await canvas.findByRole("group", {
      name: "Fields that another tool set",
    });
    await expect(group).toHaveTextContent(
      "spec.deletionPolicy: kubectl-client-side-apply set it. The save changes it.",
    );
    await userEvent.type(
      canvas.getByLabelText("Type orders to confirm"),
      "orders",
    );
    const confirm = canvas.getByRole("button", { name: "Delete the cluster" });
    await expect(confirm).toBeDisabled();
    await userEvent.click(
      canvas.getByRole("checkbox", { name: "Take over these fields" }),
    );
    await expect(confirm).toBeEnabled();
  },
};

export const PolicyOfTheOperator: Story = {
  name: "Keep picked, and the operator sets the deletion policy",
  args: {
    model: deleteClusterModel(data({ claim: claim("Delete") }), "Orphan"),
    policyStep: true,
    policyOwned:
      "The operator sets the deletion policy of the claim, so the delete cannot change it. Choose Delete the resources to delete with the policy that the claim has.",
    preview: fn(async () =>
      ownedPreview({
        name: "camunda-operator/CamundaClusterClaim/8c1f6c1e",
        operation: "Apply",
        kind: "operator",
        known: true,
      }),
    ),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      await canvas.findByText(/of the operator sets it\./),
    ).toBeVisible();
    await expect(canvas.queryByRole("checkbox")).toBeNull();
    await userEvent.type(
      canvas.getByLabelText("Type orders to confirm"),
      "orders",
    );
    await expect(
      canvas.getByRole("button", { name: "Delete the cluster" }),
    ).toBeDisabled();
  },
};
