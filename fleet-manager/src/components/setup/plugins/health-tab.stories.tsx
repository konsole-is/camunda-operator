import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { condition, pluginRegistration, registrationEvent } from "./fixtures";
import { HealthTab } from "./health-tab";

const meta = {
  title: "setup/plugins/HealthTab",
  component: HealthTab,
  args: {
    registration: pluginRegistration("aws-kms", {}, [
      condition("Accepted", "True", "Healthy", "All checks passed"),
      condition(
        "Ready",
        "True",
        "Healthy",
        "The controller can assume its role",
      ),
    ]),
    events: [
      registrationEvent(
        "aws-kms",
        "Normal",
        "Accepted",
        "The registration is accepted",
      ),
    ],
    unread: [],
    now: "2026-11-13T09:00:00Z",
  },
} satisfies Meta<typeof HealthTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Healthy: Story = { name: "Healthy" };

export const NotReady: Story = {
  name: "Not ready with a warning event",
  args: {
    registration: pluginRegistration("aws-kms", {}, [
      condition("Accepted", "True", "Healthy", "All checks passed"),
      condition(
        "Ready",
        "False",
        "CredentialsInvalid",
        "The controller cannot assume the role arn:aws:iam::123456789012:role/cpo-aws-kms",
      ),
    ]),
    events: [
      registrationEvent(
        "aws-kms",
        "Warning",
        "CredentialsInvalid",
        "The controller cannot assume its role",
      ),
    ],
  },
};

export const NoEvents: Story = { name: "No events", args: { events: [] } };
