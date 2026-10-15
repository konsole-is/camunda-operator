import type { Story } from "@/gallery/registry";
import type { SectionView } from "@/lib/policy/types";
import { SectionCards } from "./section-cards";

const story = (name: string, section: SectionView): Story => ({
  component: "SectionCards",
  name,
  render: () => <SectionCards section={section} />,
});

const backupStorage: SectionView = {
  name: "backup-storage",
  label: "Backup storage",
  description: "Where the backups of the cluster go.",
  selection: "One",
  emptyOptionLabel: "No backups",
  required: false,
  options: [
    {
      registration: "aws-s3",
      label: "New S3 bucket",
      description: "A bucket for this cluster, deleted with the claim.",
      isDefault: false,
      parameters: [
        {
          name: "bucketName",
          label: "Bucket name",
          claimEditable: false,
          value: "team-orders-orders-backups",
          note: "Named <namespace>-<claim>-backups",
        },
        {
          name: "retentionDays",
          label: "Retention in days",
          schema: { type: "integer", minimum: 1, maximum: 365 },
          claimEditable: true,
          value: 30,
        },
      ],
    },
    {
      registration: "existing-object-storage",
      label: "Existing bucket",
      description: "An ObjectStorageConfig of the namespace.",
      isDefault: true,
      parameters: [],
    },
  ],
};

const connectorsPermissions: SectionView = {
  name: "connectors-permissions",
  label: "Connectors permissions",
  selection: "One",
  emptyOptionLabel: "No cloud permissions",
  required: false,
  options: [
    {
      registration: "aws-connectors-role",
      label: "Role for each cluster",
      isDefault: true,
      parameters: [
        {
          name: "policies",
          label: "Policies",
          schema: { type: "array", items: { type: "string" } },
          claimEditable: true,
          allowed: [
            { name: "sqs", label: "SQS access" },
            { name: "s3", label: "S3 read" },
            { name: "sns", label: "SNS publish" },
          ],
          value: ["s3"],
        },
        {
          name: "boundaryPolicies",
          label: "Admin policies",
          claimEditable: false,
          value: ["a", "b", "c"],
        },
      ],
    },
  ],
};

export const sectionCardsStories: Story[] = [
  story("Default and empty option", backupStorage),
  story("Required section", { ...backupStorage, required: true }),
  story("Allowed checkboxes", connectorsPermissions),
  story("Allowed select", {
    ...connectorsPermissions,
    options: [
      {
        registration: "gcp-pubsub",
        label: "Pub/Sub topics",
        isDefault: true,
        parameters: [
          {
            name: "access",
            label: "Topic access",
            schema: { type: "string" },
            claimEditable: true,
            allowed: [
              { name: "publish", label: "Publish to the team topics" },
              { name: "subscribe", label: "Subscribe to the team topics" },
            ],
          },
        ],
      },
    ],
  }),
  story("Option that needs an input", {
    name: "volume-encryption",
    label: "Volume encryption",
    selection: "One",
    emptyOptionLabel: "Unencrypted volumes",
    required: false,
    options: [
      {
        registration: "aws-ebs",
        label: "Encrypted volumes",
        isDefault: false,
        disabledReason: "Needs an encryption key",
        parameters: [],
      },
    ],
  }),
  story("Preset cards", {
    name: "secondary-storage",
    label: "Secondary storage",
    selection: "One",
    required: true,
    options: [
      {
        registration: "elasticsearch",
        label: "New Elasticsearch",
        isDefault: true,
        parameters: [
          {
            name: "preset",
            label: "Preset",
            claimEditable: true,
            value: "small",
          },
        ],
        choices: {
          parameter: "preset",
          items: [
            {
              name: "small",
              specs: [
                { label: "Version", value: "8.19" },
                { label: "Nodes", value: "1" },
                { label: "Memory", value: "2 GiB" },
              ],
            },
            {
              name: "large",
              specs: [
                { label: "Version", value: "8.19" },
                { label: "Nodes", value: "3" },
                { label: "Memory", value: "8 GiB" },
              ],
            },
          ],
        },
      },
      {
        registration: "existing-secondary-storage",
        label: "Existing storage",
        isDefault: false,
        parameters: [],
      },
    ],
  }),
];
