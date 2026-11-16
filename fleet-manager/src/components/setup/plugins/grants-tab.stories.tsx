import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { condition, pluginGrant, pluginRegistration } from "./fixtures";
import { GrantsTab } from "./grants-tab";

const role = pluginRegistration("aws-s3", {
  grantTemplates: [
    {
      name: "bucket-access",
      description: "Lets the cluster read and write its backup bucket.",
      format: "aws-iam-statement",
      template: {
        Effect: "Allow",
        Action: ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
        Resource: "arn:aws:s3:::{{ .outputs.bucket }}/*",
      },
      requiresInputs: ["principal"],
    },
  ],
});
const grant = (name: string, template: string, conditions: ConditionRow[]) =>
  pluginGrant(
    "team-orders",
    name,
    { template, cluster: { name: "orders" } },
    {
      "plugins.camunda.io/registration": "aws-s3",
      "plugins.camunda.io/request": "orders-backup-storage",
    },
    conditions,
  );
const healthy = [
  condition("Applied", "True", "Healthy"),
  condition("MatchesTemplate", "True", "Healthy"),
  condition("Ready", "True", "Healthy"),
];
const refused = (name: string, reason: string, message: string) =>
  grant(name, "bucket-access", [
    condition("Applied", "False", reason, message),
    condition("MatchesTemplate", "True", "Healthy"),
    condition("Ready", "False", reason, message),
  ]);

const meta = {
  title: "setup/plugins/GrantsTab",
  component: GrantsTab,
  args: {
    registration: role,
    grants: [
      grant("orders-bucket-access", "bucket-access", healthy),
      refused(
        "billing-bucket-access",
        "OverCeiling",
        "s3:DeleteObject is outside the ceiling of the identity",
      ),
      grant("payments-bucket-access", "bucket-access", []),
    ],
    unread: [],
    now: "2026-11-13T09:00:00Z",
  },
} satisfies Meta<typeof GrantsTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OneTemplate: Story = {
  name: "One template with a ready, a refused, and a waiting Grant",
};

export const EachReason: Story = {
  name: "Each refusal reason",
  args: {
    grants: [
      refused(
        "a-bucket-access",
        "OverCeiling",
        "s3:DeleteObject is outside the ceiling",
      ),
      refused(
        "b-bucket-access",
        "OverSizeLimit",
        "The policy would be 6144 bytes",
      ),
      refused(
        "c-bucket-access",
        "Rejected",
        "MalformedPolicyDocument: invalid ARN",
      ),
      grant("d-bucket-access", "bucket-access", [
        condition("Applied", "True", "Healthy"),
        condition(
          "MatchesTemplate",
          "False",
          "TemplateChanged",
          "The template changed",
        ),
        condition("Ready", "False", "TemplateChanged", "The template changed"),
      ]),
    ],
  },
};

export const NoTemplates: Story = {
  name: "No grant templates",
  args: { registration: pluginRegistration("rustfs", {}), grants: [] },
};

export const RemovedTemplate: Story = {
  name: "Grant of a removed template",
  args: {
    grants: [
      grant("orders-bucket-access", "bucket-access", healthy),
      grant("orders-kms-access", "kms-access", [
        condition("Ready", "False", "TemplateChanged", "The template is gone"),
      ]),
    ],
  },
};
