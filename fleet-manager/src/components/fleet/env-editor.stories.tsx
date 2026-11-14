import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { EnvEditor } from "./env-editor";

const meta = {
  title: "fleet/EnvEditor",
  component: EnvEditor,
  args: {
    label: "Environment variables",
    secrets: [
      { name: "orders-db", keys: ["password", "username"] },
      { name: "smtp", keys: ["token"] },
    ],
    onChange: fn(),
  },
} satisfies Meta<typeof EnvEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ValuesAndSecrets: Story = {
  name: "Values and Secrets",
  args: {
    defaultValue: [
      { name: "JAVA_TOOL_OPTIONS", value: "-Xmx2g" },
      {
        name: "DB_PASSWORD",
        valueFrom: { secretKeyRef: { name: "orders-db", key: "password" } },
      },
      {
        name: "MODE",
        valueFrom: { configMapKeyRef: { name: "settings", key: "mode" } },
      },
    ],
  },
};

export const Empty: Story = { name: "Empty" };
