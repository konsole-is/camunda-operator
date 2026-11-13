// @vitest-environment jsdom
import path from "node:path";
import { describe, expect, it } from "vitest";

type StoriesModule = { default?: { title?: string } };

// The gallery stories files have no default export, so only the files in
// Component Story Format remain.
const csf = Object.entries(
  import.meta.glob<StoriesModule>("./components/**/*.stories.tsx", {
    eager: true,
  }),
).filter(([, module]) => module.default);

describe("the Storybook stories", () => {
  it("gives each stories file the title of its directory and a component name", () => {
    expect(csf.length).toBeGreaterThan(0);
    for (const [file, module] of csf) {
      const directory = path.dirname(file).replace(/^\.\/components\//, "");
      expect(module.default?.title ?? "", file).toMatch(
        new RegExp(`^${directory}/[^/]+$`),
      );
    }
  });

  it("has a story for each shared component of the fleet pages", () => {
    const components = new Set(
      csf.map(([, module]) => module.default?.title?.split("/").at(-1)),
    );
    for (const name of [
      "Badge",
      "Button",
      "Checkbox",
      "Dialog",
      "Input",
      "Popover",
      "Select",
      "Table",
      "Tabs",
      "Tooltip",
      "AccessSection",
      "GitLayoutPreview",
      "MetricsServerLine",
      "PermissionsTable",
      "FleetView",
      "AttentionPanel",
      "ComponentDots",
      "ClusterTable",
      "PlaneList",
      "PlanesTable",
      "NotAttachedChip",
      "PlaneCreateFlow",
      "EnvironmentList",
      "EnvironmentEditor",
      "SectionsTable",
      "OptionSettings",
      "TemplatePreview",
      "ImpactLine",
    ]) {
      expect(components).toContain(name);
    }
  });
});
