"use client";

import { useState } from "react";
import type { Environment } from "@/generated/crds/environment";
import { renderJSON } from "@/lib/gotemplate";
import { OptionSettingsSection } from "./option-settings";
import { previewClaims, templateData } from "./preview";
import { sectionGroups, withSectionPolicy } from "./sections";
import { SectionsTable } from "./sections-table";
import { optionSettings } from "./settings";
import type { EnvironmentSources } from "./sources";
import { TemplatePreview } from "./template-preview";

/**
 * SectionsTableStory shows SectionsTable in the gallery over spec, which
 * the edits of the story change. sections, when given, limits the rows to
 * those sections.
 */
export function SectionsTableStory({
  sources,
  spec,
  sections,
}: {
  sources: EnvironmentSources;
  spec: Environment["spec"];
  sections?: string[];
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
      onPolicy={(section, policy) =>
        setEdited((s) => withSectionPolicy(s, section, policy))
      }
    />
  );
}

/** OptionSettingsStory shows the settings of the option registration of environment in the gallery, with edits that stay in the story. */
export function OptionSettingsStory({
  sources,
  environment,
  registration,
}: {
  sources: EnvironmentSources;
  environment: Environment;
  registration: string;
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
      onSpec={setSpec}
      onTextErrors={() => {}}
    />
  );
}

/** TemplatePreviewStory shows the preview of the value of parameter of registration in section, under environment, in the gallery. */
export function TemplatePreviewStory({
  sources,
  environment,
  registration,
  section,
  parameter,
}: {
  sources: EnvironmentSources;
  environment: Environment;
  registration: string;
  section: string;
  parameter: string;
}) {
  const found = sources.registrations.find(
    (r) => r.metadata.name === registration,
  );
  const sectionObject = sources.sections.find(
    (s) => s.metadata.name === section,
  );
  const value = environment.spec.parameters?.[registration]?.[parameter]?.value;
  if (!found || !sectionObject) return null;
  return (
    <TemplatePreview
      claims={previewClaims(
        sources,
        environment.metadata.name ?? "",
        registration,
      )}
      render={(claim) =>
        renderJSON(
          value,
          templateData({
            claim,
            section: sectionObject,
            registration: found,
            environment,
            registrations: sources.registrations,
            requests: sources.requests,
          }),
          "parameter",
        )
      }
    />
  );
}
