import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { EnvironmentEditor } from "./environment-editor";
import { environmentFixtures, sectionFixtures } from "./fixtures";
import type { EnvironmentSources } from "./sources";

const sourcesOf = (sources: EnvironmentSources) => ({
  ...sources,
  errors: [],
});

function notReady() {
  const sources = environmentFixtures();
  sources.environments[0].spec.releases = {
    allowed: ["8.9.23", "8.9.9"],
    default: "8.9.23",
  };
  sources.environments[0].status = {
    namespaces: 2,
    conditions: [
      {
        type: "Ready",
        status: "False",
        reason: "InvalidReference",
        message:
          "The spec names objects that do not exist: CamundaRelease 8.9.9",
        lastTransitionTime: "2026-11-10T10:00:00Z",
      },
    ],
  };
  return sources;
}

function unknownNames() {
  const sources = sectionFixtures();
  const spec = sources.environments[0].spec;
  spec.sections = { ...spec.sections, "old-section": { allowed: ["x"] } };
  spec.parameters = { ...spec.parameters, "old-option": {} };
  return sources;
}

const meta = {
  title: "setup/environments/EnvironmentEditor",
  component: EnvironmentEditor,
  args: {
    sources: sourcesOf(environmentFixtures()),
    mode: "direct",
    actions: {
      previewChange: fn(async () => ({ objects: [], errors: [] })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
      previewLabel: fn(async () => ({ objects: [], errors: [] })),
      applyLabel: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof EnvironmentEditor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Production: Story = {
  name: "Production",
  args: { name: "production" },
};

export const New: Story = { name: "New" };

export const NotReady: Story = {
  name: "Not ready",
  args: { sources: sourcesOf(notReady()), name: "production" },
};

export const GitMode: Story = {
  name: "Git mode",
  args: { name: "staging", mode: "git" },
};

export const SettingsUnread: Story = {
  name: "Settings unread",
  args: { name: "staging", mode: undefined },
};

export const SectionsAndSettingsOfTheOptions: Story = {
  name: "Sections and settings of the options",
  args: { sources: sourcesOf(sectionFixtures()), name: "production" },
};

export const NamesThatDoNotExist: Story = {
  name: "Names that do not exist",
  args: { sources: sourcesOf(unknownNames()), name: "production" },
};
