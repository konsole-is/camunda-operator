// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import {
  type Story,
  findStory,
  groupByComponent,
  slug,
  storyId,
} from "./registry";
import { stories } from "./stories";

const story = (component: string, name: string): Story => ({
  component,
  name,
  render: () => null,
});

describe("slug", () => {
  it.each([
    ["Not ready", "not-ready"],
    ["StatusChip", "statuschip"],
    ["  Setup / detail  ", "setup-detail"],
  ])("turns %j into %j", (text, expected) => {
    expect(slug(text)).toBe(expected);
  });
});

describe("the gallery stories", () => {
  it("has at least one story", () => {
    expect(stories.length).toBeGreaterThan(0);
  });

  it("gives every story a different id, so no screenshot overwrites another", () => {
    const ids = stories.map(storyId);
    expect(ids.filter((id, index) => ids.indexOf(id) !== index)).toEqual([]);
  });

  it("gives every story an id that is a safe path", () => {
    for (const s of stories) {
      expect(storyId(s)).toMatch(
        /^[a-z0-9]+(-[a-z0-9]+)*\/[a-z0-9]+(-[a-z0-9]+)*$/,
      );
    }
  });

  it("has a story for each shared component of the fleet pages", () => {
    const components = new Set(stories.map((s) => s.component));
    for (const name of [
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
    ]) {
      expect(components).toContain(name);
    }
  });

  it("has a story for each shell, status, and locked field component", () => {
    const components = new Set(stories.map((s) => s.component));
    for (const name of [
      "StatusChip",
      "StatusDot",
      "LockedField",
      "Logo",
      "TopNav",
      "SetupNav",
      "SetupBreadcrumb",
      "AppShell",
    ]) {
      expect(components).toContain(name);
    }
  });
});

describe("findStory and groupByComponent", () => {
  const list = [story("B", "One"), story("A", "One"), story("B", "Two")];

  it("finds a story by its id", () => {
    expect(findStory(list, "b/two")).toBe(list[2]);
    expect(findStory(list, "b/three")).toBeUndefined();
  });

  it("groups by component in first-seen order", () => {
    expect(
      groupByComponent(list).map(([component, entries]) => [
        component,
        entries.map((s) => s.name),
      ]),
    ).toEqual([
      ["B", ["One", "Two"]],
      ["A", ["One"]],
    ]);
  });
});
