import type { Story } from "@/gallery/registry";
import { SettingsEditorStory } from "./settings-editor-story";

export const settingsEditorStories: Story[] = [
  {
    component: "SettingsEditor",
    name: "Brokers from the preset",
    render: () => <SettingsEditorStory name="brokers" />,
  },
  {
    component: "SettingsEditor",
    name: "Embedded Tasklist",
    render: () => <SettingsEditorStory name="embedded" />,
  },
  {
    component: "SettingsEditor",
    name: "Locked fields",
    render: () => <SettingsEditorStory name="locked" />,
  },
  {
    component: "SettingsEditor",
    name: "Optimize",
    render: () => <SettingsEditorStory name="optimize" />,
  },
  {
    component: "SelectionSettingsEditor",
    name: "Built-in Elasticsearch overrides",
    render: () => <SettingsEditorStory name="selection" />,
  },
];
