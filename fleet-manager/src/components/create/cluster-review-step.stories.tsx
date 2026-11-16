import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { GrantPreview } from "@/lib/claims/templates";
import type { Change, Preview } from "@/lib/write/types";
import {
  type ClusterReviewSummary,
  ClusterReviewStep,
} from "./cluster-review-step";

const namespace = {
  apiVersion: "v1",
  kind: "Namespace",
  metadata: {
    name: "team-new",
    labels: { "fleet.camunda.io/environment": "apps", team: "orders" },
  },
};

const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name: "orders", namespace: "team-new" },
  spec: {
    presetRef: "small",
    releaseRef: "camunda-8-9-0",
    plugins: [
      { section: "secondary-storage", registration: "elasticsearch" },
      { section: "backup-storage", registration: "s3" },
    ],
  },
};

const change: Change = {
  intent: "Create the cluster orders in team-new",
  apply: [namespace, claim],
  create: ["Namespace//team-new", "CamundaClusterClaim/team-new/orders"],
};

const summary: ClusterReviewSummary = [
  {
    label: "Environment and name",
    terms: [
      { label: "Environment", value: "Apps" },
      { label: "Name", value: "orders" },
      { label: "Namespace", value: "team-new", note: "New" },
      { label: "Preset", value: "small" },
    ],
  },
  {
    label: "Storage and access",
    terms: [
      {
        label: "Secondary storage",
        value: "New Elasticsearch cluster",
        note: "Preset: small-es",
      },
      { label: "Backup storage", value: "S3 bucket" },
      { label: "When the claim is deleted", value: "Delete the data" },
    ],
  },
  {
    label: "Cluster settings",
    terms: [
      { label: "Release", value: "camunda-8-9-0 (8.9.0)" },
      {
        label: "Platform config",
        value: "default-platform",
        note: "Default of the environment",
      },
      { label: "Changed settings", value: "None" },
    ],
  },
  {
    label: "Attachments",
    terms: [
      { label: "Optimize", value: "Off" },
      { label: "Backup schedule", value: "Off" },
    ],
  },
];

const effects = [
  "Creates the namespace team-new.",
  "Creates the claim orders.",
  "Requests Secondary storage: New Elasticsearch cluster as orders-secondary-storage.",
  "Requests Backup storage: S3 bucket as orders-backup-storage.",
  "Creates the CamundaCluster orders.",
];

const grant: GrantPreview = {
  section: "backup-storage",
  sectionLabel: "Backup storage",
  registration: "s3",
  optionLabel: "S3 bucket",
  template: "backup-access",
  description: "Lets the cluster read and write its backups.",
  result: {
    value: {
      Effect: "Allow",
      Action: ["s3:GetObject", "s3:PutObject"],
      Resource: "<bucket ARN from Backup storage>",
    },
  },
};

const clean = async (): Promise<Preview> => ({ objects: [], errors: [] });

const meta = {
  title: "create/ClusterReviewStep",
  component: ClusterReviewStep,
  args: {
    change,
    summary,
    effects,
    grants: [],
    mode: "direct",
    preview: clean,
    create: fn(async () => ({ ok: true as const, mode: "direct" as const })),
    onWritten: fn(),
    onCancel: fn(),
  },
} satisfies Meta<typeof ClusterReviewStep>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DirectModeWithNewNamespace: Story = {
  name: "Direct mode with a new namespace",
};

export const GitMode: Story = {
  name: "Git mode",
  args: { mode: "git" },
};

export const RefusedDryRun: Story = {
  name: "Refused dry run",
  args: {
    preview: async () => ({
      objects: [],
      errors: [
        {
          code: "Forbidden",
          message:
            "The fleet manager may not create Namespaces. Ask your admin to create team-new.",
        },
      ],
    }),
  },
};

export const GrantPreviewWithPlaceholder: Story = {
  name: "Grant preview with a placeholder",
  args: { grants: [grant] },
};

export const ListUnread: Story = {
  name: "A list could not be read",
  args: { grants: [grant], unread: ["PluginRegistration"] },
};
