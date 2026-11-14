import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { fn } from "storybook/test";
import type { Environment } from "@/generated/crds/environment";
import type { ValidationError } from "@/lib/schema";
import { sectionFixtures } from "./fixtures";
import { OptionSettingsSection } from "./option-settings";
import { optionSettings } from "./settings";
import type { EnvironmentSources } from "./sources";

// The harness shows the settings of the option registration and keeps the
// edits of the story in the spec of environment.
function OptionSettingsHarness({
  sources,
  environment,
  registration,
  onSpec,
  onTextErrors,
}: {
  sources: EnvironmentSources;
  environment: Environment;
  registration: string;
  onSpec: (spec: Environment["spec"]) => void;
  onTextErrors: (errors: ValidationError[]) => void;
}) {
  const [spec, setSpec] = useState(environment.spec);
  return (
    <OptionSettingsSection
      settings={optionSettings(sources.registrations, spec).filter(
        (s) => s.registration === registration,
      )}
      sources={sources}
      environment={{ ...environment, spec }}
      errors={[]}
      onSpec={(next) => {
        onSpec(next);
        setSpec(next);
      }}
      onTextErrors={onTextErrors}
    />
  );
}

const sources = sectionFixtures();

const meta = {
  title: "setup/environments/OptionSettings",
  component: OptionSettingsHarness,
  args: {
    sources,
    environment: sources.environments[0],
    onSpec: fn(),
    onTextErrors: fn(),
  },
  render: (args) => (
    <OptionSettingsHarness key={JSON.stringify(args.environment)} {...args} />
  ),
} satisfies Meta<typeof OptionSettingsHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ConnectorsRoleWithAllowedValues: Story = {
  name: "Connectors role with allowed values",
  args: { registration: "aws-connectors-role" },
};
