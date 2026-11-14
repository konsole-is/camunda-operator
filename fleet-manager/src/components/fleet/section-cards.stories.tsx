import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { SectionView } from "@/lib/policy/types";
import { SectionCards } from "./section-cards";

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
          summary: "team-orders-orders-backups",
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
          summary: "3 policies",
        },
      ],
    },
  ],
};

const meta = {
  title: "fleet/SectionCards",
  component: SectionCards,
  args: { section: backupStorage, onChange: fn() },
} satisfies Meta<typeof SectionCards>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DefaultAndEmptyOption: Story = {
  name: "Default and empty option",
};

export const RequiredSection: Story = {
  name: "Required section",
  args: { section: { ...backupStorage, required: true } },
};

export const AllowedCheckboxes: Story = {
  name: "Allowed checkboxes",
  args: { section: connectorsPermissions },
};

export const AllowedSelect: Story = {
  name: "Allowed select",
  args: {
    section: {
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
    },
  },
};

export const MapParameter: Story = {
  name: "Map parameter",
  args: {
    section: {
      ...backupStorage,
      options: [
        {
          registration: "aws-s3",
          label: "New S3 bucket",
          description: "A bucket for this cluster, deleted with the claim.",
          isDefault: true,
          parameters: [
            {
              name: "tags",
              label: "Bucket tags",
              schema: {
                type: "object",
                additionalProperties: { type: "string" },
              },
              claimEditable: true,
              value: { team: "orders", "cost-center": "4711" },
            },
          ],
        },
      ],
    },
  },
};

export const OptionThatNeedsAnInput: Story = {
  name: "Option that needs an input",
  args: {
    section: {
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
    },
  },
};

export const PresetCards: Story = {
  name: "Preset cards",
  args: {
    section: {
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
    },
  },
};

export const ParameterRules: Story = {
  name: "Parameter rules",
  args: {
    section: {
      name: "team-storage",
      label: "Team storage",
      selection: "One",
      required: true,
      options: [
        {
          registration: "team-bucket",
          label: "Team bucket",
          description: "A bucket that the team shares.",
          isDefault: true,
          schema: {
            type: "object",
            "x-kubernetes-validations": [
              {
                rule: "self.minSizeGi <= self.maxSizeGi",
                message: "The minimum size must not be above the maximum size",
              },
            ],
          },
          parameters: [
            {
              name: "name",
              label: "Bucket name",
              schema: {
                type: "string",
                "x-kubernetes-validations": [
                  {
                    rule: "self.startsWith('team-')",
                    message: "The name must start with team-",
                  },
                ],
              },
              claimEditable: true,
              value: "team-orders",
            },
            {
              name: "maxSizeGi",
              label: "Maximum size in GiB",
              schema: {
                type: "integer",
                "x-kubernetes-validations": [{ rule: "self % 10 == 0" }],
              },
              claimEditable: true,
              value: 50,
            },
            {
              name: "minSizeGi",
              label: "Minimum size in GiB",
              schema: { type: "integer" },
              claimEditable: true,
              value: 10,
            },
          ],
        },
      ],
    },
  },
};
