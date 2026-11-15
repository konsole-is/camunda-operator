import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { platformFixtures } from "./fixtures";
import { usersOf } from "./model";
import { UsersSection } from "./users-section";

const meta = {
  title: "setup/platform/UsersSection",
  component: UsersSection,
  args: { users: usersOf("oidc-prod", platformFixtures()) },
} satisfies Meta<typeof UsersSection>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EveryKind: Story = { name: "Every kind of user" };

export const NoUser: Story = {
  name: "No user",
  args: { users: usersOf("basic-lab", platformFixtures()) },
};
