"use client";

import type { ValidationError } from "@/lib/schema";
import type { EnvironmentDraft } from "./draft";
import { EditorSection, NameChecklist, SelectField } from "./form-parts";
import type { EnvironmentSources } from "./sources";

type Spec = EnvironmentDraft["spec"];

const namesOf = (objects: readonly { metadata?: { name?: string } }[]) =>
  objects.map((o) => o.metadata?.name ?? "");

/**
 * ReferencesSection edits the platform config, the three preset lists, the
 * allowed releases, and the default release. A name of the spec that no
 * object has shows "Not found". The default release offers the checked
 * releases, and keeps a default that is no longer checked so that its error
 * shows.
 */
export function ReferencesSection({
  spec,
  sources,
  errors,
  onSpec,
}: {
  spec: Spec;
  sources: EnvironmentSources;
  errors: readonly ValidationError[];
  onSpec: (spec: Spec) => void;
}) {
  const configs = namesOf(sources.platformConfigs);
  const config = spec.platformConfigRef ?? "";
  const allowed = spec.releases?.allowed ?? [];
  const defaultRelease = spec.releases?.default ?? "";
  const presets = (key: keyof NonNullable<Spec["presets"]>, list: string[]) =>
    onSpec({ ...spec, presets: { ...spec.presets, [key]: list } });
  const releases = (next: NonNullable<Spec["releases"]>) =>
    onSpec({ ...spec, releases: { ...spec.releases, ...next } });
  return (
    <>
      <EditorSection title="Platform config">
        <SelectField
          label="Platform config"
          value={config}
          options={[
            ["", "None"],
            ...[...new Set([...configs, ...(config ? [config] : [])])].map(
              (name) =>
                [
                  name,
                  configs.includes(name) ? name : `${name} (not found)`,
                ] as const,
            ),
          ]}
          onChange={(platformConfigRef) =>
            onSpec({ ...spec, platformConfigRef })
          }
        />
        <p className="max-w-2xl text-sm text-muted">
          A claim of the environment that names no platform config gets this
          one.
        </p>
      </EditorSection>
      <EditorSection title="Presets">
        <NameChecklist
          legend="Cluster presets"
          available={namesOf(sources.clusterPresets)}
          selected={spec.presets?.camundaCluster ?? []}
          empty="No cluster preset exists."
          onChange={(list) => presets("camundaCluster", list)}
        />
        <NameChecklist
          legend="Elasticsearch presets"
          available={namesOf(sources.elasticsearchPresets)}
          selected={spec.presets?.elasticsearchCluster ?? []}
          empty="No Elasticsearch preset exists."
          onChange={(list) => presets("elasticsearchCluster", list)}
        />
        <NameChecklist
          legend="PostgreSQL presets"
          available={namesOf(sources.databasePresets)}
          selected={spec.presets?.databaseServer ?? []}
          empty="No PostgreSQL preset exists."
          onChange={(list) => presets("databaseServer", list)}
        />
      </EditorSection>
      <EditorSection title="Releases">
        <NameChecklist
          legend="Allowed releases"
          available={namesOf(sources.releases)}
          selected={allowed}
          empty="No release exists."
          onChange={(list) => releases({ allowed: list })}
        />
        <SelectField
          label="Default release"
          value={defaultRelease}
          error={
            errors.find((e) => e.path === "spec.releases.default")?.message
          }
          options={[
            ["", "No default"],
            ...[
              ...new Set([
                ...allowed,
                ...(defaultRelease ? [defaultRelease] : []),
              ]),
            ].map((name) => [name, name] as const),
          ]}
          onChange={(value) => releases({ default: value })}
        />
        <p className="max-w-2xl text-sm text-muted">
          A claim that names no release gets the default release.
        </p>
      </EditorSection>
    </>
  );
}
