import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { platformFixtures } from "./fixtures";
import { platformRows } from "./model";
import { PlatformList } from "./platform-list";

const meta = {
  title: "setup/platform/PlatformList",
  component: PlatformList,
  args: { rows: platformRows(platformFixtures()), errors: [] },
} satisfies Meta<typeof PlatformList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Configs: Story = {
  name: "Configs with users, without users, and a missing Secret",
};

export const NoConfig: Story = { name: "No config", args: { rows: [] } };

const unread = {
  ...platformFixtures(),
  errors: [{ kind: "DatabaseServer", message: "forbidden" }],
  stale: ["Environment"],
};

export const UsersNotKnown: Story = {
  name: "A list that cannot be read",
  args: {
    rows: platformRows(unread),
    errors: unread.errors,
    stale: unread.stale,
  },
};
