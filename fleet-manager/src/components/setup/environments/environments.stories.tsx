import type { Story } from "@/gallery/registry";
import { EnvironmentEditorStory } from "./editor-story";
import { EnvironmentList } from "./environment-list";
import { environmentFixtures, sectionFixtures } from "./fixtures";
import { ImpactLine } from "./impact-line";
import { environmentRows, unassignedNamespaces } from "./rows";
import {
  OptionSettingsStory,
  SectionsTableStory,
  TemplatePreviewStory,
} from "./settings-story";

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

const orders = {
  kind: "CamundaClusterClaim" as const,
  namespace: "team-a",
  name: "orders",
};

export const environmentStories: Story[] = [
  {
    component: "EnvironmentList",
    name: "Three environments and the banner",
    render: () => {
      const sources = environmentFixtures();
      return (
        <EnvironmentList
          rows={environmentRows(sources)}
          unassigned={unassignedNamespaces(sources)}
          errors={[]}
        />
      );
    },
  },
  {
    component: "EnvironmentList",
    name: "No environment",
    render: () => {
      const sources = { ...environmentFixtures(), environments: [] };
      return (
        <EnvironmentList
          rows={environmentRows(sources)}
          unassigned={unassignedNamespaces(sources)}
          errors={[]}
        />
      );
    },
  },
  {
    component: "EnvironmentEditor",
    name: "Production",
    render: () => (
      <EnvironmentEditorStory
        sources={environmentFixtures()}
        name="production"
        mode="direct"
      />
    ),
  },
  {
    component: "EnvironmentEditor",
    name: "New",
    render: () => (
      <EnvironmentEditorStory sources={environmentFixtures()} mode="direct" />
    ),
  },
  {
    component: "EnvironmentEditor",
    name: "Not ready",
    render: () => (
      <EnvironmentEditorStory
        sources={notReady()}
        name="production"
        mode="direct"
      />
    ),
  },
  {
    component: "EnvironmentEditor",
    name: "Git mode",
    render: () => (
      <EnvironmentEditorStory
        sources={environmentFixtures()}
        name="staging"
        mode="git"
      />
    ),
  },
  {
    component: "EnvironmentEditor",
    name: "Settings unread",
    render: () => (
      <EnvironmentEditorStory sources={environmentFixtures()} name="staging" />
    ),
  },
  {
    component: "EnvironmentEditor",
    name: "Sections and settings of the options",
    render: () => (
      <EnvironmentEditorStory
        sources={sectionFixtures()}
        name="production"
        mode="direct"
      />
    ),
  },
  {
    component: "EnvironmentEditor",
    name: "Names that do not exist",
    render: () => {
      const sources = sectionFixtures();
      const spec = sources.environments[0].spec;
      spec.sections = { ...spec.sections, "old-section": { allowed: ["x"] } };
      spec.parameters = { ...spec.parameters, "old-option": {} };
      return (
        <EnvironmentEditorStory
          sources={sources}
          name="production"
          mode="direct"
        />
      );
    },
  },
  {
    component: "SectionsTable",
    name: "Platform environment",
    render: () => {
      const sources = environmentFixtures();
      return (
        <SectionsTableStory
          sources={sources}
          spec={sources.environments[2].spec}
        />
      );
    },
  },
  {
    component: "SectionsTable",
    name: "Notes",
    render: () => {
      const sources = sectionFixtures();
      return (
        <SectionsTableStory
          sources={sources}
          spec={{
            sections: {
              ...sources.environments[0].spec.sections,
              "encryption-key": { allowed: ["aws-kms", "vault-transit"] },
            },
          }}
          sections={["encryption-key", "volume-encryption"]}
        />
      );
    },
  },
  {
    component: "OptionSettings",
    name: "Connectors role with allowed values",
    render: () => {
      const sources = sectionFixtures();
      return (
        <OptionSettingsStory
          sources={sources}
          environment={sources.environments[0]}
          registration="aws-connectors-role"
        />
      );
    },
  },
  {
    component: "TemplatePreview",
    name: "Inline policy",
    render: () => {
      const sources = sectionFixtures();
      return (
        <TemplatePreviewStory
          sources={sources}
          environment={sources.environments[0]}
          registration="aws-connectors-role"
          section="connectors-permissions"
          parameter="inlinePolicy"
        />
      );
    },
  },
  {
    component: "ImpactLine",
    name: "Warnings",
    render: () => (
      <ImpactLine
        impact={{
          clusters: 2,
          planes: 1,
          warnings: [
            {
              claim: orders,
              kind: "CamundaRelease",
              name: "8.9.22",
              removed: true,
            },
            {
              claim: { ...orders, name: "billing" },
              kind: "CamundaClusterPreset",
              name: "large",
              removed: true,
            },
            {
              claim: { ...orders, name: "search" },
              kind: "CamundaRelease",
              name: "8.9.23",
              followsDefault: true,
            },
            {
              claim: { ...orders, name: "search" },
              kind: "ElasticsearchClusterPreset",
              name: "es-small",
              removed: true,
            },
          ],
        }}
      />
    ),
  },
];
