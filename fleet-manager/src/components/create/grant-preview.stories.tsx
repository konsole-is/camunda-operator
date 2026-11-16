import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { GrantPreview } from "@/lib/claims/templates";
import { GrantPreviewList } from "./grant-preview";

const read: GrantPreview = {
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

const connectors: GrantPreview = {
  section: "connectors-permissions",
  sectionLabel: "Connector permissions",
  registration: "aws-connectors-role",
  optionLabel: "AWS role for connectors",
  template: "policies",
  result: {
    value: {
      Effect: "Allow",
      Action: "sts:AssumeRole",
      Policy: "<policy: SQS access>",
    },
  },
};

const meta = {
  title: "create/GrantPreviewList",
  component: GrantPreviewList,
} satisfies Meta<typeof GrantPreviewList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const PickedChoiceAndPlaceholder: Story = {
  name: "Picked choice and placeholder",
  args: { previews: [read, connectors] },
};

export const TemplateError: Story = {
  name: "Template error",
  args: {
    previews: [
      read,
      {
        ...connectors,
        result: {
          error:
            'parameter "managedPolicyArns" is a list: a template prints only a string, a number, or a boolean',
          field: "parameters.managedPolicyArns",
          path: "Policy",
        },
      },
    ],
  },
};

export const RegistrationsUnread: Story = {
  name: "Registrations could not be read",
  args: { previews: [read], unread: ["PluginRegistration"] },
};
