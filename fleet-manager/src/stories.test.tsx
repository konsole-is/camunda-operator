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
      "DataTable",
      "EnvEditor",
      "KeyValueEditor",
      "ResourcesEditor",
      "SchedulingEditor",
      "SecretRefPicker",
      "YamlEditor",
      "SectionCards",
      "WizardStepper",
      "NamespacePicker",
      "DiffView",
      "ReviewPanel",
      "TypedConfirm",
      "InlineConfirm",
      "Toast",
      "FieldHelp",
      "ConditionsTable",
      "PodTable",
      "UsageBars",
      "MetricChart",
      "LogViewer",
      "EventList",
      "ProgressRows",
      "StatusChip",
      "StatusDot",
      "LockedField",
      "Logo",
      "TopNav",
      "ThemeSwitch",
      "SetupNav",
      "SetupBreadcrumb",
      "AppShell",
      "AccessSection",
      "GitLayoutPreview",
      "MetricsServerLine",
      "PermissionsTable",
    ]) {
      expect(components).toContain(name);
    }
  });
});
