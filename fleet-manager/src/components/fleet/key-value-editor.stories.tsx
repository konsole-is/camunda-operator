import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { KeyValueEditor } from "./key-value-editor";

const meta = {
  title: "fleet/KeyValueEditor",
  component: KeyValueEditor,
  args: { label: "Annotations", onChange: fn() },
} satisfies Meta<typeof KeyValueEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Labels: Story = {
  name: "Labels",
  args: {
    label: "Pod labels",
    format: "label",
    defaultValue: { team: "payments", tier: "gold" },
  },
};

export const Empty: Story = { name: "Empty" };
