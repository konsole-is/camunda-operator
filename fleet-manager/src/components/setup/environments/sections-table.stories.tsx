import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { fn } from "storybook/test";
import type { Environment } from "@/generated/crds/environment";
import { environmentFixtures, sectionFixtures } from "./fixtures";
import {
  type SectionPolicyForm,
  sectionGroups,
  withSectionPolicy,
} from "./sections";
import { SectionsTable } from "./sections-table";
import type { EnvironmentSources } from "./sources";

// The harness keeps the edits of the story in spec, so a click changes the
// table. sections, when given, limits the rows to those sections.
function SectionsTableHarness({
  sources,
  spec,
  sections,
  onPolicy,
}: {
  sources: EnvironmentSources;
  spec: Environment["spec"];
  sections?: string[];
  onPolicy: (section: string, policy: SectionPolicyForm) => void;
}) {
  const [edited, setEdited] = useState(spec);
  const groups = sectionGroups(
    sources.sections,
    sources.registrations,
    edited,
  ).map((g) => ({
    ...g,
    rows: g.rows.filter((r) => !sections || sections.includes(r.name)),
  }));
  return (
    <SectionsTable
      groups={groups}
      errors={[]}
      onPolicy={(section, policy) => {
        onPolicy(section, policy);
        setEdited((s) => withSectionPolicy(s, section, policy));
      }}
    />
  );
}

const platform = environmentFixtures();
const withSections = sectionFixtures();

const meta = {
  title: "setup/environments/SectionsTable",
  component: SectionsTableHarness,
  args: {
    sources: platform,
    spec: platform.environments[2].spec,
    onPolicy: fn(),
  },
} satisfies Meta<typeof SectionsTableHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const PlatformEnvironment: Story = { name: "Platform environment" };

export const Notes: Story = {
  name: "Notes",
  args: {
    sources: withSections,
    spec: {
      sections: {
        ...withSections.environments[0].spec.sections,
        "encryption-key": { allowed: ["aws-kms", "vault-transit"] },
      },
    },
    sections: ["encryption-key", "volume-encryption"],
  },
};
