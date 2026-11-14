import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { SecretRefPicker } from "./secret-ref-picker";

const meta = {
  title: "fleet/SecretRefPicker",
  component: SecretRefPicker,
  args: { label: "Client secret", secrets: [], onChange: fn() },
} satisfies Meta<typeof SecretRefPicker>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Picked: Story = {
  name: "Picked",
  args: {
    secrets: [
      { name: "orders-db", keys: ["password", "username"] },
      { name: "smtp", keys: ["token"] },
    ],
    defaultValue: { name: "smtp", key: "token" },
  },
};

export const NoSecrets: Story = { name: "No Secrets" };
