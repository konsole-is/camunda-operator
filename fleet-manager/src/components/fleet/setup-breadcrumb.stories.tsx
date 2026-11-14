import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SetupBreadcrumb } from "./setup-breadcrumb";

const meta = {
  title: "fleet/SetupBreadcrumb",
  component: SetupBreadcrumb,
} satisfies Meta<typeof SetupBreadcrumb>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DetailPage: Story = {
  name: "Detail page",
  args: { pathname: "/setup/presets/gold" },
};

export const LongName: Story = {
  name: "Long name",
  args: {
    pathname:
      "/setup/releases/camunda-8-9-long-term-support-with-extended-security-patches-eu-north-1",
  },
};
