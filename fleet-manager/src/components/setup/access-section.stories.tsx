import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AccessSection } from "./access-section";

const meta = {
  title: "setup/AccessSection",
  component: AccessSection,
  args: { namespace: "camunda-system" },
} satisfies Meta<typeof AccessSection>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithTheServiceName: Story = {
  name: "With the Service name",
  args: { serviceName: "camunda-operator-fleet-manager" },
};

export const WithoutTheServiceName: Story = {
  name: "Without the Service name",
};
