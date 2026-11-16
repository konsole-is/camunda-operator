"use client";

import { type ReactNode, useRef, useState } from "react";
import {
  type Setting,
  settingsOf,
} from "@/components/cluster/editable-settings";
import type { SettingsGroup } from "@/components/cluster/settings";
import { SettingsEditor } from "@/components/cluster/settings-editor";
import { PROCESS_LABEL } from "@/components/cluster/topology";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import {
  secretRefsUnion,
  withSecretRefs,
} from "@/components/fleet/secret-refs-union";
import { YamlEditor } from "@/components/fleet/yaml-editor";
import { EditorSection, TextField } from "@/components/setup/form-parts";
import { type EnvSpec, namedSecretRefs } from "@/components/setup/releases/env";
import { EnvScopes } from "@/components/setup/releases/env-scopes";
import { COMPONENTS, type SchemaGroup, type Step } from "@/lib/fields";
import { without, withValue } from "@/lib/session";
import type { PresetKind } from "@/lib/usage";
import type { PresetDraft } from "./draft";
import { cannotSet, type PresetPart, presetParts } from "./merge-rules";
import { missingSecrets } from "./secrets";

/** BODY_STEPS are the steps of the settings before a field of the body of a cluster preset. */
const BODY_STEPS = 2;

const ENV_LISTS = new Set(["extraEnv", "extraEnvFrom"]);

const GROUPS: readonly SettingsGroup[] = [
  { label: "General", component: "cluster", embedded: false, fields: [] },
  ...COMPONENTS.map((component): SettingsGroup => ({
    label: PROCESS_LABEL[component as keyof typeof PROCESS_LABEL],
    component: component as SchemaGroup,
    embedded: false,
    fields: [],
  })),
];

/**
 * PresetEditor edits draft, the body and the description of a preset of
 * kind. A cluster preset edits each field that it can set with the field
 * editors of the cluster page, and the env lists of each scope with
 * EnvScopes. Each group shows the merge rules of the fields that the draft
 * sets. An Elasticsearch or PostgreSQL preset edits its body as YAML, with
 * no schema: the dry run of the review checks it. secrets are the Secret
 * refs of each namespace whose Secrets the body can name, and a Secret key
 * that one of them lacks shows as missing there. The editor names each
 * list of secretsUnread: the lists that secrets needs and that were not
 * read. onDraft gets a function
 * that applies each valid edit to a draft. onErrors gets every error that
 * the editors show, and an empty list once they show none. While the YAML
 * of the body has an error, the draft keeps its last valid body. The
 * editors read draft once, so a caller gives the component a new key when
 * it replaces the draft.
 */
export function PresetEditor({
  kind,
  draft,
  live,
  secrets,
  secretsUnread,
  onDraft,
  onErrors,
}: {
  kind: PresetKind;
  draft: PresetDraft;
  /** live is the body of the preset before the pending changes. */
  live?: Record<string, unknown>;
  secrets: Readonly<Record<string, SecretRefs>>;
  secretsUnread: readonly string[];
  onDraft: (update: (draft: PresetDraft) => PresetDraft) => void;
  onErrors: (errors: string[]) => void;
}): ReactNode {
  const errors = useRef<Record<string, string[]>>({});
  const [bodyError, setBodyError] = useState<string>();
  const report = (source: string, messages: string[]) => {
    errors.current[source] = messages;
    onErrors(Object.values(errors.current).flat());
  };
  const union = secretRefsUnion(secrets);
  const missing = missingSecrets(draft.body, union.missingIn).filter(
    // EnvScopes shows a missing key at its env entry.
    (m) => kind !== "CamundaClusterPreset" || !m.env,
  );

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <EditorSection title="Description">
        <TextField
          label="Description"
          value={draft.description}
          hint="What the preset is for. The lists of presets show it."
          onChange={(description) => onDraft((d) => ({ ...d, description }))}
        />
      </EditorSection>
      {kind === "CamundaClusterPreset" ? (
        <ClusterBody
          draft={draft}
          live={live}
          secrets={withSecretRefs(
            union.refs,
            namedSecretRefs(draft.body as EnvSpec),
          )}
          missingIn={union.missingIn}
          onDraft={onDraft}
          report={report}
        />
      ) : (
        <EditorSection title="Settings">
          <YamlEditor
            label={
              kind === "DatabaseServerPreset" ? "spec.server" : "spec.cluster"
            }
            rows={14}
            defaultValue={draft.body}
            error={bodyError}
            onChange={(value, yamlErrors) => {
              if (yamlErrors.length > 0) {
                report(
                  "yaml",
                  yamlErrors.map((e) =>
                    e.path ? `${e.path}: ${e.message}` : e.message,
                  ),
                );
                return;
              }
              if (value !== undefined && !isBody(value)) {
                setBodyError(NOT_A_MAP);
                report("yaml", [NOT_A_MAP]);
                return;
              }
              setBodyError(undefined);
              report("yaml", []);
              onDraft((d) => ({ ...d, body: value ?? {} }));
            }}
          />
          <p className="text-sm text-muted">
            The review checks each field against the CRD before the save.
          </p>
        </EditorSection>
      )}
      {secretsUnread.length > 0 && (
        <p role="alert" className="text-sm text-bad">
          The Secrets of the users are not known while the fleet manager cannot
          read {secretsUnread.join(", ")}. Reload the page when it can read
          them.
        </p>
      )}
      {missing.length > 0 && (
        <ul
          aria-label="Missing Secret keys"
          className="flex flex-col gap-1 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
        >
          {missing.map((m) => (
            <li key={`${m.name}/${m.key}`}>
              {m.name}/{m.key} is missing in {m.namespaces.join(", ")}.
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function ClusterBody({
  draft,
  live,
  secrets,
  missingIn,
  onDraft,
  report,
}: {
  draft: PresetDraft;
  live?: Record<string, unknown>;
  secrets: SecretRefs;
  missingIn: (name: string, key: string) => readonly string[];
  onDraft: (update: (draft: PresetDraft) => PresetDraft) => void;
  report: (source: string, messages: string[]) => void;
}): ReactNode {
  const invalid = useRef<Record<string, string>>({});
  const refused = cannotSet("CamundaClusterPreset");
  const parts = presetParts("CamundaClusterPreset", draft.body);
  const setBody = (
    update: (body: Record<string, unknown>) => Record<string, unknown>,
  ) => onDraft((d) => ({ ...d, body: update(d.body) }));
  const editable = (setting: Setting): Setting | undefined => {
    const path = pathOf(setting.steps);
    if (path === undefined) return undefined;
    if (refused.some((r) => path === r || path.startsWith(`${r}.`)))
      return undefined;
    if (ENV_LISTS.has(path.split(".").at(-1) ?? "") && isScope(path))
      return undefined;
    const set = setting.value !== undefined;
    return {
      ...setting,
      ...(set && { state: { kind: "preset" } }),
      claimSet: set,
      resetLabel: "Remove from the preset",
    };
  };

  return (
    <>
      <EditorSection title="Settings">
        {GROUPS.map((group) => {
          const settings = settingsOf({
            group,
            spec: draft.body,
            live,
          }).flatMap((s) => editable(s) ?? []);
          const rules = rulesFor(group, parts);
          return (
            <div key={group.component} className="flex min-w-0 flex-col gap-2">
              <SettingsEditor
                group={group}
                settings={settings}
                secrets={secrets}
                set={(steps, value) =>
                  setBody((body) =>
                    withValue(body, steps.slice(BODY_STEPS), value),
                  )
                }
                reset={(steps) =>
                  setBody((body) => without(body, steps.slice(BODY_STEPS)))
                }
                invalid={(steps, message) => {
                  const key = pathOf(steps) ?? "";
                  if (message === undefined) delete invalid.current[key];
                  else invalid.current[key] = message;
                  report("settings", Object.values(invalid.current));
                }}
              />
              {rules.length > 0 && (
                <div className="rounded-sm bg-panel-2 px-3 py-2">
                  <p className="mb-1 text-xs font-medium text-muted">
                    How the fields of {group.label} merge
                  </p>
                  <ul
                    aria-label={`Merge rules of ${group.label}`}
                    className="flex flex-col gap-0.5 text-xs"
                  >
                    {rules.map((r) => (
                      <li
                        key={r.path}
                        className="flex flex-wrap justify-between gap-x-4"
                      >
                        <span className="font-mono break-all text-text">
                          {r.path}
                        </span>
                        <span className="text-muted">{r.rule}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          );
        })}
      </EditorSection>
      <EditorSection title="Environment variables">
        <EnvScopes
          spec={draft.body as EnvSpec}
          secrets={secrets}
          missingIn={missingIn}
          onChange={(update, envErrors) => {
            setBody(
              (body) => update(body as EnvSpec) as Record<string, unknown>,
            );
            report(
              "env",
              envErrors.map((e) => `${e.path}: ${e.message}`),
            );
          }}
        />
      </EditorSection>
    </>
  );
}

// pathOf returns the path of steps in the body of a cluster preset, or
// undefined for steps that do not lead to a field of the body.
function pathOf(steps: readonly Step[] | undefined): string | undefined {
  if (!steps || steps.length <= BODY_STEPS) return undefined;
  const names = steps
    .slice(BODY_STEPS)
    .map((step) => ("field" in step ? step.field : undefined));
  return names.every((n) => n !== undefined) ? names.join(".") : undefined;
}

// isScope says whether the env list at path is one that EnvScopes edits:
// the top level or a process block.
function isScope(path: string): boolean {
  const names = path.split(".");
  return (
    names.length === 1 ||
    (names.length === 2 && (COMPONENTS as readonly string[]).includes(names[0]))
  );
}

function rulesFor(group: SettingsGroup, parts: PresetPart[]) {
  const titles =
    group.component === "cluster"
      ? ["General", "Authentication", "Backup"]
      : [group.label];
  return parts
    .filter((p) => titles.includes(p.title))
    .flatMap((p) => p.settings);
}

const NOT_A_MAP = "The body must be a map of fields.";

function isBody(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
