"use client";

import { type ReactNode, useState } from "react";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { ChangeBar } from "@/components/fleet/change-bar";
import { ChangeReview } from "@/components/fleet/change-review";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { TypedConfirm } from "@/components/fleet/typed-confirm";
import type { ReadError } from "@/components/planes/planes-table";
import { EditorSection } from "@/components/setup/form-parts";
import { useSetupEditor } from "@/components/setup/use-setup-editor";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { PresetKind } from "@/lib/usage";
import type { Change, Preview, WriteMode } from "@/lib/write/types";
import { presetDeleteChange } from "./delete";
import { presetChangeCount, presetDraftOf, presetUpdateChange } from "./draft";
import { FactsList } from "./facts-list";
import { impactUnread, presetImpact } from "./impact";
import { ImpactPreview } from "./impact-preview";
import { inheritingRows, inheritingUnread } from "./inheriting";
import { InheritingTable } from "./inheriting-table";
import {
  type PresetObject,
  PRESETS_HREF,
  presetBody,
  presetHref,
  presetLabel,
} from "./kinds";
import { PresetEditor } from "./preset-editor";
import { GIT_DELETE_EFFECT } from "./preset-list";
import { CannotSetSection, PresetSettings } from "./preset-settings";
import { RotateAction } from "./rotate-action";
import { rotationUnread } from "./rotation";
import { changedLabel, presetRows, usedByLabel } from "./rows";
import {
  type PresetActions,
  type PresetSnapshot,
  type PresetSources,
  usePresetSources,
} from "./sources";

/**
 * PresetDetail shows the preset name of kind: its facts and users, an
 * editor of its fields and description with the merge rule of each field
 * that it sets, the fields that it cannot set, and the resources that
 * inherit it. A save shows a review with the dry run and the impact on
 * each inheriting resource. The save waits while the users or the releases
 * of the impact cannot be read. A cluster preset offers the rotation of the
 * admin password of its clusters while the form has no pending change. A
 * preset that can be deleted offers a delete after a typed confirm and a
 * review. The delete review closes before its write starts when the preset
 * can no longer be deleted. secrets are the Secret refs of each namespace
 * whose Secrets the body can name, and secretsUnread the lists that they
 * need and that were not read. Without mode nothing can change.
 */
export function PresetDetail({
  sources,
  kind,
  name,
  secrets,
  secretsUnread,
  mode,
  actions,
}: {
  sources: PresetSources & { errors: ReadError[]; pending: KindName[] };
  kind: PresetKind;
  name: string;
  secrets: Readonly<Record<string, SecretRefs>>;
  secretsUnread: readonly string[];
  mode?: WriteMode;
  actions: PresetActions;
}): ReactNode {
  const list = listOf(kind, sources);
  const live = list.find((p) => p.metadata.name === name);
  const [writing, setWriting] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [editorErrors, setEditorErrors] = useState<string[]>([]);
  const editor = useSetupEditor({
    live,
    isNew: false,
    listed: list.map((p) => p.metadata.name ?? ""),
    draftOf: presetDraftOf,
    listHref: PRESETS_HREF,
    hrefOf: (n) => presetHref(kind, n),
    onReset: () => setEditorErrors([]),
  });
  const { base, draft, reviewing, locked } = editor;
  const { toast, show, dismiss } = useWriteToast();
  const row = presetRows(sources, sources.errors, sources.pending).find(
    (r) => r.kind === kind && r.name === name,
  );
  const shown = live ?? base;
  const count = presetChangeCount(base, draft);
  const deleting = reviewing?.kind === "delete";
  const unread = impactUnread(kind, sources.errors, sources.pending);
  // A preset can get a user, or an environment can allow it, after the
  // delete review opened. A write that already runs cannot stop.
  if (deleting && !writing && !row?.deletable) editor.cancel();

  if (!shown) {
    return (
      <p role="alert" className="text-sm text-bad">
        The preset {name} does not exist.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-3">
        <div className="flex flex-col gap-1">
          <p className="text-sm text-muted">{presetLabel(kind)} preset</p>
          <h1 className="font-heading text-3xl font-bold tracking-[-0.02em] break-all">
            {name}
          </h1>
          {row?.description && (
            <p className="max-w-2xl text-text-2">{row.description}</p>
          )}
        </div>
        {row && (
          <dl className="grid max-w-3xl gap-x-8 gap-y-3 text-sm sm:grid-cols-[auto_1fr]">
            {row.facts.length > 0 && (
              <>
                <dt className="text-muted">Key facts</dt>
                <dd>
                  <FactsList facts={row.facts} />
                </dd>
              </>
            )}
            <dt className="text-muted">Allowed in</dt>
            <dd>
              {row.allowedUnknown
                ? "Unknown"
                : row.allowedIn.length > 0
                  ? row.allowedIn.join(", ")
                  : "Nowhere"}
            </dd>
            <dt className="text-muted">Used by</dt>
            <dd className="flex flex-col gap-1">
              <span>
                {row.usersUnknown ? "Unknown" : usedByLabel(row.users)}
              </span>
              {!row.usersUnknown && row.users.length > 0 && (
                <ul className="flex flex-wrap gap-x-4 gap-y-0.5 font-mono text-xs text-text-2">
                  {row.users.map((u) => (
                    <li key={`${u.kind}/${u.namespace}/${u.name}`}>
                      {u.namespace}/{u.name}
                    </li>
                  ))}
                </ul>
              )}
            </dd>
            <dt className="text-muted">Last changed</dt>
            <dd>
              {row.lastChanged ? (
                <time dateTime={row.lastChanged} className="tabular-nums">
                  {changedLabel(row.lastChanged)}
                </time>
              ) : (
                "Unknown"
              )}
            </dd>
          </dl>
        )}
      </header>
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
      {!live && (
        <p role="alert" className="text-sm text-bad">
          The preset {name} was deleted.
        </p>
      )}
      <fieldset
        disabled={locked || rotating || !live || !mode}
        className="flex min-w-0 flex-col gap-4"
      >
        <PresetEditor
          key={editor.generation}
          kind={kind}
          draft={draft}
          live={presetBody(shown)}
          secrets={secrets}
          secretsUnread={secretsUnread}
          onDraft={editor.setDraft}
          onErrors={setEditorErrors}
        />
      </fieldset>
      {kind === "CamundaClusterPreset" ? (
        <CannotSetSection kind={kind} />
      ) : (
        <PresetSettings kind={kind} body={draft.body} />
      )}
      <EditorSection title="Inheriting resources">
        <InheritingTable
          rows={inheritingRows(shown, sources)}
          unread={inheritingUnread(kind, sources.errors, sources.pending)}
        />
      </EditorSection>
      {live?.kind === "CamundaClusterPreset" && (
        <EditorSection title="Admin password">
          <RotateAction
            preset={live as Resource<"CamundaClusterPreset">}
            sources={sources}
            unread={rotationUnread(sources.errors, sources.pending)}
            mode={mode}
            disabled={count > 0 || reviewing !== undefined || locked}
            actions={actions}
            show={(result, message) => {
              show(result, message);
              editor.written(result, name);
            }}
            onReviewing={setRotating}
          />
        </EditorSection>
      )}
      {live && row && (
        <fieldset disabled={locked || rotating || !mode} className="min-w-0">
          <EditorSection title="Delete the preset">
            {row.deletable ? (
              <TypedConfirm
                name={name}
                confirmLabel="Review the delete"
                onConfirm={() =>
                  editor.review({
                    change: presetDeleteChange([live]),
                    kind: "delete",
                  })
                }
              >
                No resource, claim, or request uses this preset, and no
                environment allows it.
              </TypedConfirm>
            ) : (
              <p className="text-sm text-muted">{row.blockedBy}</p>
            )}
          </EditorSection>
        </fieldset>
      )}
      {reviewing && !deleting && mode && base && (
        <section
          aria-label="Review the preset"
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Changes to the preset"
            effects={(preview) => [
              <ImpactPreview
                key="impact"
                impact={presetImpact({
                  kind,
                  name,
                  ...specsOf(preview, base, reviewing.change),
                  sources,
                  unread,
                })}
              />,
            ]}
            hold={unread.length > 0}
            confirm={{ label: "Save" }}
            savedMessage={`The preset ${name} is saved.`}
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              editor.written(result, name);
            }}
            onCancel={editor.cancel}
            onWriting={setWriting}
          />
        </section>
      )}
      {!reviewing && count > 0 && !locked && !rotating && base && (
        <ChangeBar
          count={count}
          blocked={editorErrors[0]}
          onDiscard={editor.discard}
          onReview={() =>
            editor.review({
              change: presetUpdateChange(base, draft),
              kind: "save",
            })
          }
        />
      )}
      {deleting && mode && (
        <section
          aria-label="Review the delete"
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Presets to delete"
            effects={[
              `The preset ${name} is deleted.`,
              ...(mode === "git" ? [GIT_DELETE_EFFECT] : []),
            ]}
            confirm={{
              label: "Delete the preset",
              gitLabel: "Push the delete to Git",
              danger: true,
            }}
            savedMessage={`The preset ${name} is deleted.`}
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              editor.written(result, name);
            }}
            onCancel={editor.cancel}
            onWriting={setWriting}
          />
        </section>
      )}
      {toast && (
        <div className="fixed right-4 bottom-4 z-20">
          <Toast result={toast} onDismiss={dismiss} />
        </div>
      )}
    </div>
  );
}

// specsOf returns the spec of the preset before the save and the spec that
// the dry run of preview keeps. Another field manager can keep a field that
// the change leaves out, so the impact reads the preview, not the draft.
// Without a previewed object it falls back to base and the applied spec.
function specsOf(
  preview: Preview,
  base: PresetObject,
  change: Change,
): { before: PresetObject["spec"]; after: PresetObject["spec"] } {
  const object = preview.objects.find(
    (o) => o.ref.kind === base.kind && o.ref.name === base.metadata.name,
  );
  return {
    before: (object?.before?.spec ?? base.spec) as PresetObject["spec"],
    after: (object?.after?.spec ??
      change.apply[0]?.spec ??
      base.spec) as PresetObject["spec"],
  };
}

function listOf(kind: PresetKind, sources: PresetSources): PresetObject[] {
  switch (kind) {
    case "CamundaClusterPreset":
      return sources.clusterPresets;
    case "ElasticsearchClusterPreset":
      return sources.elasticsearchPresets;
    case "DatabaseServerPreset":
      return sources.databasePresets;
  }
}

/** PresetDetailLive is PresetDetail on the live sources, starting from initial. */
export function PresetDetailLive({
  initial,
  kind,
  name,
  secrets,
  secretsUnread,
  mode,
  actions,
}: {
  initial: PresetSnapshot;
  kind: PresetKind;
  name: string;
  secrets: Readonly<Record<string, SecretRefs>>;
  secretsUnread: readonly string[];
  mode?: WriteMode;
  actions: PresetActions;
}): ReactNode {
  const sources = usePresetSources(initial);
  return (
    <PresetDetail
      sources={sources}
      kind={kind}
      name={name}
      secrets={secrets}
      secretsUnread={secretsUnread}
      mode={mode}
      actions={actions}
    />
  );
}
