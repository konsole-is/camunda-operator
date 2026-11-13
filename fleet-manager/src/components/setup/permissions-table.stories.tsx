import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { PermissionsTable } from "./permissions-table";

const rules = [
  {
    apiGroups: [""],
    resources: ["configmaps"],
    resourceNames: ["fleet-manager-settings"],
    verbs: ["get", "update", "patch"],
  },
  {
    apiGroups: [""],
    resources: ["secrets"],
    resourceNames: [],
    verbs: ["get", "list", "watch"],
  },
  {
    apiGroups: ["fleet.camunda.io"],
    resources: ["camundaclusterclaims", "environments"],
    resourceNames: [],
    verbs: ["create", "delete", "get", "list", "patch", "update", "watch"],
  },
];

const meta = {
  title: "setup/PermissionsTable",
  component: PermissionsTable,
  args: { permissions: { rules, incomplete: false } },
} satisfies Meta<typeof PermissionsTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Rules: Story = { name: "Rules" };

export const Incomplete: Story = {
  name: "Incomplete",
  args: { permissions: { rules, incomplete: true } },
};

export const ReadFailed: Story = {
  name: "Read failed",
  args: {
    permissions: {
      error:
        'selfsubjectrulesreviews.authorization.k8s.io is forbidden: User "system:serviceaccount:fleet:fleet-manager" cannot create resource',
    },
  },
};
