import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { detailData } from "./fixtures";
import { RegistrationDetail } from "./registration-detail";

const data = detailData("elasticsearch");

const meta = {
  title: "setup/plugins/RegistrationDetail",
  component: RegistrationDetail,
  args: {
    data,
    errors: [],
    stale: [],
    tab: "about",
    now: "2026-11-13T09:00:00Z",
  },
} satisfies Meta<typeof RegistrationDetail>;
export default meta;
type Story = StoryObj<typeof meta>;

export const AboutBuiltIn: Story = {
  name: "About tab of a built-in registration",
};

export const Gone: Story = {
  name: "Registration that no longer exists",
  args: { data: { ...data, sources: { ...data.sources, registrations: [] } } },
};
