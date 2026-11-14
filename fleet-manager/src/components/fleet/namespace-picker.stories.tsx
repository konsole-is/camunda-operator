import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { NamespacePicker } from "./namespace-picker";

const meta = {
  title: "fleet/NamespacePicker",
  component: NamespacePicker,
  args: {
    namespaces: [
      { name: "team-payments", environment: "prod" },
      { name: "team-orders", environment: "prod" },
    ],
    environment: {
      name: "prod",
      displayName: "Production",
      namespaceCreation: {
        allowed: true,
        labels: { "cost-center": "4711" },
        requiredLabelKeys: ["team"],
      },
    },
    onChange: fn(),
  },
} satisfies Meta<typeof NamespacePicker>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ExistingNamespace: Story = {
  name: "Existing namespace",
  args: { defaultValue: { kind: "existing", name: "team-orders" } },
};

export const NewNamespace: Story = {
  name: "New namespace",
  args: {
    defaultValue: {
      kind: "new",
      name: "team-search",
      labels: { team: "search" },
    },
  },
};

export const CreationNotAllowed: Story = {
  name: "Creation not allowed",
  args: { environment: { name: "prod", displayName: "Production" } },
};
