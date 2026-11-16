"use client";

import { type ReactNode, useState } from "react";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { ChangeReview } from "@/components/fleet/change-review";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { TypedConfirm } from "@/components/fleet/typed-confirm";
import type { ReadError } from "@/components/planes/planes-table";
import { EditorSection } from "@/components/setup/form-parts";
import { useSetupEditor } from "@/components/setup/use-setup-editor";
import type { PresetKind } from "@/lib/usage";
import type { WriteMode } from "@/lib/write/types";
import { presetDeleteChange } from "./delete";
import { FactsList } from "./facts-list";
import { inheritingRows } from "./inheriting";
import { InheritingTable } from "./inheriting-table";
import {
  type PresetObject,
  PRESETS_HREF,
  presetBody,
  presetHref,
  presetLabel,
} from "./kinds";
import { GIT_DELETE_EFFECT } from "./preset-list";
import { PresetSettings } from "./preset-settings";
import { changedLabel, presetRows, usedByLabel } from "./rows";
import {
  type PresetActions,
  type PresetSnapshot,
  type PresetSources,
  usePresetSources,
} from "./sources";

/**
 * PresetDetail shows the preset name of kind: its facts and users, each
 * field that it sets with its merge rule, the fields that it cannot set,
 * and the resources that inherit it. A preset that can be deleted offers
 * a delete after a typed confirm and a review. The review closes before
 * its write starts when the preset can no longer be deleted. Without mode
 * nothing can be deleted.
 */
export function PresetDetail({
  sources,
  kind,
  name,
  mode,
  actions,
}: {
  sources: PresetSources & { errors: ReadError[]; pending: string[] };
  kind: PresetKind;
  name: string;
  mode?: WriteMode;
  actions: PresetActions;
}): ReactNode {
  const list = listOf(kind, sources);
  const live = list.find((p) => p.metadata.name === name);
  const [writing, setWriting] = useState(false);
  const editor = useSetupEditor({
    live,
    isNew: false,
    listed: list.map((p) => p.metadata.name ?? ""),
    draftOf: (preset: PresetObject | undefined) =>
      preset ? presetBody(preset) : undefined,
    listHref: PRESETS_HREF,
    hrefOf: (n) => presetHref(kind, n),
  });
  const { base, reviewing, locked } = editor;
  const { toast, show, dismiss } = useWriteToast();
  const row = presetRows(sources, sources.errors, sources.pending).find(
    (r) => r.kind === kind && r.name === name,
  );
  const shown = live ?? base;
  // A preset can get a user, or an environment can allow it, after the
  // review opened. A write that already runs cannot stop.
  if (reviewing && !writing && !row?.deletable) editor.cancel();

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
      <PresetSettings kind={kind} body={presetBody(shown)} />
      <EditorSection title="Inheriting resources">
        <InheritingTable rows={inheritingRows(shown, sources)} />
      </EditorSection>
      {live && row && (
        <fieldset disabled={locked || !mode} className="min-w-0">
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
      {reviewing && mode && (
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
  mode,
  actions,
}: {
  initial: PresetSnapshot;
  kind: PresetKind;
  name: string;
  mode?: WriteMode;
  actions: PresetActions;
}): ReactNode {
  const sources = usePresetSources(initial);
  return (
    <PresetDetail
      sources={sources}
      kind={kind}
      name={name}
      mode={mode}
      actions={actions}
    />
  );
}
