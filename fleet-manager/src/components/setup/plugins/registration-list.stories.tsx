import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { CatalogChart } from "@/startup/bundled-data";
import catalog from "../../../../catalog/official-plugins.json";
import { condition, pluginRegistration, registrationSources } from "./fixtures";
import { registrationGroups, registrationSummary } from "./groups";
import { RegistrationList } from "./registration-list";
import type { RegistrationSources } from "./sources";

const listOf = (sources: RegistrationSources) => ({
  groups: registrationGroups(sources),
  summary: registrationSummary(sources.registrations),
  sections: sources.sections,
});

const withRegistrations = (...extra: RegistrationSources["registrations"]) => {
  const sources = registrationSources();
  return listOf({
    ...sources,
    registrations: [...sources.registrations, ...extra],
  });
};

const meta = {
  title: "setup/plugins/RegistrationList",
  component: RegistrationList,
  args: {
    ...listOf(registrationSources()),
    charts: [],
    errors: [],
    now: "2026-11-13T09:00:00Z",
  },
} satisfies Meta<typeof RegistrationList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BuiltInRegistrations: Story = { name: "Built-in registrations" };

export const NotAcceptedAndNotReady: Story = {
  name: "A registration that is not accepted and one that is not ready",
  args: withRegistrations(
    pluginRegistration(
      "rustfs",
      {
        label: "RustFS bucket",
        sections: ["backup-storage"],
        mode: "Default",
        controller: { name: "rustfs", version: "0.1.0" },
        operatorVersions: ">=0.3.0 <1.0.0",
      },
      [
        condition(
          "Accepted",
          "False",
          "DefaultConflict",
          "In section backup-storage, these registrations also have mode Default: s3-bucket",
        ),
        condition("Ready", "True", "Healthy"),
      ],
    ),
    pluginRegistration(
      "aws-kms",
      {
        label: "AWS KMS key",
        sections: ["encryption-key"],
        controller: { name: "aws-kms", version: "0.1.0" },
        deletionPolicies: ["Delete"],
      },
      [
        condition("Accepted", "True", "Healthy"),
        condition(
          "Ready",
          "False",
          "CredentialsInvalid",
          "The role cannot be assumed",
        ),
      ],
    ),
  ),
};

export const InTwoSections: Story = {
  name: "A registration in two sections",
  args: withRegistrations(
    pluginRegistration(
      "aws-identity",
      {
        label: "AWS IAM role",
        sections: ["cloud-identity", "connectors-permissions"],
        controller: { name: "aws-identity", version: "0.1.0" },
      },
      [
        condition("Accepted", "True", "Healthy"),
        condition("Ready", "True", "Healthy"),
      ],
    ),
  ),
};

export const SectionGone: Story = {
  name: "A registration whose section is gone",
  args: withRegistrations(
    pluginRegistration(
      "vault-transit",
      {
        label: "Vault transit key",
        sections: ["secret-store"],
        controller: { name: "vault-transit", version: "0.2.0" },
      },
      [
        condition(
          "Accepted",
          "False",
          "SectionMissing",
          "These sections do not exist: secret-store",
        ),
      ],
    ),
  ),
};

export const WithTheCatalogPanel: Story = {
  name: "With the catalog panel",
  args: { charts: catalog.charts as CatalogChart[] },
};
