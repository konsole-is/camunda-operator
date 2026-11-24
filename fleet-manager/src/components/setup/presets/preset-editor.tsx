"use client";

import { type ReactNode, useRef } from "react";
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
import { EditorSection, TextField } from "@/components/setup/form-parts";
import { type EnvSpec, namedSecretRefs } from "@/components/setup/releases/env";
import { EnvScopes } from "@/components/setup/releases/env-scopes";
import { COMPONENTS, type SchemaGroup, type Step } from "@/lib/fields";
import { without, withValue } from "@/lib/session";
import type { PresetKind } from "@/lib/usage";
import type { PresetDraft } from "./draft";
import { presetLabel } from "./kinds";
import { cannotSet, type PresetPart, presetParts } from "./merge-rules";
import { missingSecrets } from "./secrets";

/** BODY_STEPS are the steps of the settings before a field of the body of a preset. */
const BODY_STEPS = 2;

const ENV_LISTS = new Set(["extraEnv", "extraEnvFrom"]);

// A preset page has no YAML editor, as the presets guide says.
const OTHER_SOURCE_REASON =
  "Edit this variable with kubectl or through your Git repository";

const GROUPS: Readonly<Record<PresetKind, readonly SettingsGroup[]>> = {
  CamundaClusterPreset: [
    { label: "General", component: "cluster", embedded: false, fields: [] },
    ...COMPONENTS.map((component): SettingsGroup => ({
      label: PROCESS_LABEL[component as keyof typeof PROCESS_LABEL],
      component: component as SchemaGroup,
      embedded: false,
      fields: [],
    })),
  ],
  ElasticsearchClusterPreset: [
    {
      label: presetLabel("ElasticsearchClusterPreset"),
      component: "elasticsearch",
      embedded: false,
      fields: [],
    },
  ],
  DatabaseServerPreset: [
    {
      label: presetLabel("DatabaseServerPreset"),
      component: "databaseServer",
      embedded: false,
      fields: [],
    },
  ],
};

/**
 * PresetEditor edits draft, the body and the description of a preset of
 * kind. Every kind edits each field that it can set with the field editors
 * of the cluster page, and each group shows the merge rules of the fields
 * that the draft sets. A cluster preset edits the env lists of each scope
 * with EnvScopes. secrets are the Secret refs of each namespace whose
 * Secrets the body can name, and a Secret key that one of them lacks shows
 * as missing there. The editor names each list of secretsUnread: the lists
 * that secrets needs and that were not read. onDraft gets a function that
 * applies each valid edit to a draft. onErrors gets every error that the
 * editors show, and an empty list once they show none. While an editor
 * shows an error, the draft keeps the last valid value of its field. The
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
      <BodySettings
        kind={kind}
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

function BodySettings({
  kind,
  draft,
  live,
  secrets,
  missingIn,
  onDraft,
  report,
}: {
  kind: PresetKind;
  draft: PresetDraft;
  live?: Record<string, unknown>;
  secrets: SecretRefs;
  missingIn: (name: string, key: string) => readonly string[];
  onDraft: (update: (draft: PresetDraft) => PresetDraft) => void;
  report: (source: string, messages: string[]) => void;
}): ReactNode {
  const invalid = useRef<Record<string, string>>({});
  const cluster = kind === "CamundaClusterPreset";
  const refused = cannotSet(kind);
  const parts = presetParts(kind, draft.body);
  const setBody = (
    update: (body: Record<string, unknown>) => Record<string, unknown>,
  ) => onDraft((d) => ({ ...d, body: update(d.body) }));
  const editable = (setting: Setting): Setting | undefined => {
    const path = pathOf(setting.steps);
    if (path === undefined) return undefined;
    if (refused.some((r) => path === r || path.startsWith(`${r}.`)))
      return undefined;
    if (cluster && ENV_LISTS.has(path.split(".").at(-1) ?? "") && isScope(path))
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
        {GROUPS[kind].map((group) => {
          const settings = settingsOf({
            group,
            spec: draft.body,
            live,
          }).flatMap((s) => editable(s) ?? []);
          const rules = rulesFor(kind, group, parts);
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
                otherSourceReason={OTHER_SOURCE_REASON}
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
      {cluster && (
        <EditorSection title="Environment variables">
          <EnvScopes
            spec={draft.body as EnvSpec}
            secrets={secrets}
            missingIn={missingIn}
            otherSourceReason={OTHER_SOURCE_REASON}
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
      )}
    </>
  );
}

// pathOf returns the path of steps in the body of a preset, or
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

function rulesFor(kind: PresetKind, group: SettingsGroup, parts: PresetPart[]) {
  if (kind !== "CamundaClusterPreset") return parts.flatMap((p) => p.settings);
  const titles =
    group.component === "cluster"
      ? ["General", "Authentication", "Backup"]
      : [group.label];
  return parts
    .filter((p) => titles.includes(p.title))
    .flatMap((p) => p.settings);
}
