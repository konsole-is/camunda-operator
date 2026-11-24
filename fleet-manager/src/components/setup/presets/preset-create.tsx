"use client";

import { type ReactNode, useState } from "react";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { ChangeBar } from "@/components/fleet/change-bar";
import { ChangeReview } from "@/components/fleet/change-review";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import {
  EditorSection,
  SelectField,
  TextField,
} from "@/components/setup/form-parts";
import { useSetupEditor } from "@/components/setup/use-setup-editor";
import { Radio } from "@/components/ui/radio";
import type { ReadError } from "@/lib/k8s/unread";
import type { PresetKind } from "@/lib/usage";
import type { WriteMode } from "@/lib/write/types";
import { CannotSetSection } from "./cannot-set-section";
import {
  CREATE_EFFECT,
  createChange,
  nameErrors,
  type PresetCreateDraft,
  presetChangeCount,
  presetDraftOf,
} from "./draft";
import {
  PRESET_KINDS,
  type PresetObject,
  PRESETS_HREF,
  presetHref,
  presetsOf,
} from "./kinds";
import { PresetEditor } from "./preset-editor";
import {
  type PresetActions,
  type PresetSnapshot,
  type PresetSources,
  usePresetSources,
} from "./sources";

/**
 * PresetCreate is the create page of a preset on sources. The admin picks
 * the kind, then starts empty or as a copy of the body of a preset of that
 * kind. A new kind starts empty again. The create shows a review with a dry
 * run before the write, and the page moves to the new preset once its list
 * shows it. Without mode the form is disabled.
 */
export function PresetCreate({
  sources,
  mode,
  actions,
}: {
  sources: PresetSources & { errors: ReadError[] };
  mode?: WriteMode;
  actions: PresetActions;
}): ReactNode {
  const [kind, setKind] = useState<PresetKind>("CamundaClusterPreset");
  const [copy, setCopy] = useState("");
  const [editorErrors, setEditorErrors] = useState<string[]>([]);
  const list = presetsOf(kind, sources);
  const names = list.map((p) => p.metadata.name ?? "");
  const bodyOf = (source: string) =>
    presetDraftOf(list.find((p) => p.metadata.name === source)).body;
  const editor = useSetupEditor<PresetObject, PresetCreateDraft>({
    live: undefined,
    isNew: true,
    listed: names,
    draftOf: () => ({
      name: "",
      preset: { body: bodyOf(copy), description: "" },
    }),
    listHref: PRESETS_HREF,
    hrefOf: (n) => presetHref(kind, n),
    onReset: () => setEditorErrors([]),
  });
  const { draft, reviewing, locked } = editor;
  const { toast, show, dismiss } = useWriteToast();
  const nameError = nameErrors(draft.name, names)[0]?.message;
  const count =
    presetChangeCount(undefined, draft.preset) + (draft.name !== "" ? 1 : 0);
  const setBody = (body: Record<string, unknown>) =>
    editor.setDraft((d) => ({ ...d, preset: { ...d.preset, body } }));

  return (
    <div className="flex flex-col gap-6">
      <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
        New preset
      </h1>
      {!mode && (
        <p role="alert" className="text-sm text-bad">
          {CHANGES_OFF}
        </p>
      )}
      {sources.errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      <fieldset
        disabled={locked || !mode}
        className="flex min-w-0 flex-col gap-4"
      >
        <EditorSection title="Basics">
          <fieldset className="flex flex-col gap-2">
            <legend className="mb-1 text-sm font-medium text-text">Kind</legend>
            {PRESET_KINDS.map((k) => (
              <label
                key={k.kind}
                className="flex min-h-6 items-center gap-2 text-sm"
              >
                <Radio
                  name="preset-kind"
                  checked={kind === k.kind}
                  onChange={() => {
                    setKind(k.kind);
                    setCopy("");
                    setEditorErrors([]);
                    setBody({});
                  }}
                />
                {k.label}
              </label>
            ))}
          </fieldset>
          <SelectField
            label="Copy from"
            value={copy}
            options={[
              ["", "Nothing, start empty"],
              ...names.map((n) => [n, n] as const),
            ]}
            onChange={(source) => {
              setCopy(source);
              setEditorErrors([]);
              setBody(bodyOf(source));
            }}
          />
          <TextField
            label="Name"
            value={draft.name}
            mono
            hint="Lowercase letters, digits, and hyphens. It cannot change later."
            error={nameError}
            onChange={(name) => editor.setDraft((d) => ({ ...d, name }))}
          />
        </EditorSection>
        <PresetEditor
          key={`${editor.generation}-${kind}-${copy}`}
          kind={kind}
          draft={draft.preset}
          secrets={{}}
          secretsUnread={[]}
          onDraft={(update) =>
            editor.setDraft((d) => ({ ...d, preset: update(d.preset) }))
          }
          onErrors={setEditorErrors}
        />
      </fieldset>
      <CannotSetSection kind={kind} />
      {reviewing && mode ? (
        <section
          aria-label="Review the preset"
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Changes to the preset"
            effects={[CREATE_EFFECT]}
            confirm={{ label: "Create" }}
            savedMessage={`The preset ${draft.name} is created.`}
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              editor.written(result, draft.name);
            }}
            onCancel={editor.cancel}
          />
        </section>
      ) : (
        count > 0 &&
        !locked && (
          <ChangeBar
            count={count}
            blocked={nameError ?? editorErrors[0]}
            onDiscard={editor.discard}
            onReview={() =>
              editor.review({
                change: createChange(kind, draft.name, draft.preset),
                kind: "save",
              })
            }
          />
        )
      )}
      {toast && (
        <div className="fixed right-4 bottom-4 z-20">
          <Toast result={toast} onDismiss={dismiss} />
        </div>
      )}
    </div>
  );
}

/** PresetCreateLive is PresetCreate on the live sources, starting from initial. */
export function PresetCreateLive({
  initial,
  mode,
  actions,
}: {
  initial: PresetSnapshot;
  mode?: WriteMode;
  actions: PresetActions;
}): ReactNode {
  const sources = usePresetSources(initial);
  return <PresetCreate sources={sources} mode={mode} actions={actions} />;
}
