import type { Story } from "@/gallery/registry";
import { renderJSON } from "@/lib/gotemplate";
import { EnvironmentEditorStory } from "./editor-story";
import { EnvironmentList } from "./environment-list";
import { environmentFixtures, sectionFixtures } from "./fixtures";
import { ImpactLine } from "./impact-line";
import { OptionSettingsSection } from "./option-settings";
import { previewClaims, templateData } from "./preview";
import { environmentRows, unassignedNamespaces } from "./rows";
import { sectionGroups } from "./sections";
import { SectionsTable } from "./sections-table";
import { optionSettings } from "./settings";
import { TemplatePreview } from "./template-preview";

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

// production of sectionFixtures, with the Connectors role only.
function connectorsRole() {
  const sources = sectionFixtures();
  const production = sources.environments[0];
  return {
    sources,
    production,
    settings: optionSettings(sources.registrations, production.spec).filter(
      (s) => s.registration === "aws-connectors-role",
    ),
  };
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
    component: "SectionsTable",
    name: "Platform environment",
    render: () => {
      const sources = environmentFixtures();
      return (
        <SectionsTable
          groups={sectionGroups(
            sources.sections,
            sources.registrations,
            sources.environments[2].spec,
          )}
          errors={[]}
          onPolicy={() => {}}
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
        <SectionsTable
          groups={sectionGroups(sources.sections, sources.registrations, {
            sections: {
              ...sources.environments[0].spec.sections,
              "encryption-key": { allowed: ["aws-kms", "vault-transit"] },
            },
          }).map((g) => ({
            ...g,
            rows: g.rows.filter((r) =>
              ["encryption-key", "volume-encryption"].includes(r.name),
            ),
          }))}
          errors={[]}
          onPolicy={() => {}}
        />
      );
    },
  },
  {
    component: "OptionSettings",
    name: "Connectors role with allowed values",
    render: () => {
      const { sources, production, settings } = connectorsRole();
      return (
        <OptionSettingsSection
          settings={settings}
          sources={sources}
          environment={production}
          errors={[]}
          onSpec={() => {}}
          onTextErrors={() => {}}
        />
      );
    },
  },
  {
    component: "TemplatePreview",
    name: "Inline policy",
    render: () => {
      const { sources, production } = connectorsRole();
      const registration = sources.registrations.find(
        (r) => r.metadata.name === "aws-connectors-role",
      )!;
      const section = sources.sections.find(
        (s) => s.metadata.name === "connectors-permissions",
      )!;
      const value =
        production.spec.parameters?.["aws-connectors-role"]?.inlinePolicy
          ?.value;
      return (
        <TemplatePreview
          claims={previewClaims(sources, "production", "aws-connectors-role")}
          render={(claim) =>
            renderJSON(
              value,
              templateData({
                claim,
                section,
                registration,
                environment: production,
                registrations: sources.registrations,
                requests: sources.requests,
              }),
              "parameter",
            )
          }
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
