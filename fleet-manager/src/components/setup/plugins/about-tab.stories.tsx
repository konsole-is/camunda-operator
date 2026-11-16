import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AboutTab } from "./about-tab";
import { pluginRegistration, registrationSources } from "./fixtures";

const sources = registrationSources();
const kms = pluginRegistration("aws-kms", {
  label: "New KMS key",
  description:
    "Makes a customer managed KMS key for the volumes of the cluster.",
  docsUrl: "https://example.com/docs/aws-kms",
  controller: { name: "aws-kms", version: "0.4.1" },
  serviceAccount: { name: "aws-kms", namespace: "camunda-plugins" },
  limits: [
    {
      label: "Regions",
      value: "eu-west-1, eu-central-1",
      description: "A key can only be made in these regions.",
    },
    { label: "Permissions boundary", value: "cpo-plugin-boundary" },
  ],
});

const meta = {
  title: "setup/plugins/AboutTab",
  component: AboutTab,
  args: {
    registration: kms,
    registrations: [kms],
    unread: [],
  },
} satisfies Meta<typeof AboutTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithLimits: Story = { name: "With limits" };

export const WithoutLimits: Story = {
  name: "Without limits",
  args: {
    registration: sources.registrations.find(
      (r) => r.metadata.name === "elasticsearch",
    )!,
    registrations: sources.registrations,
  },
};
