"use client";

import { CircleAlert } from "lucide-react";
import { type ReactNode, useMemo, useState } from "react";
import {
  applyEnvironmentChange,
  applyNamespaceLabel,
  previewEnvironmentChange,
  previewNamespaceLabel,
} from "@/app/setup/environments/actions";
import { ChangeBar } from "@/components/fleet/change-bar";
import { ChangeReview } from "@/components/fleet/change-review";
import { StatusChip } from "@/components/fleet/status-chip";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { TypedConfirm } from "@/components/fleet/typed-confirm";
import type { ReadError } from "@/components/planes/planes-table";
import { EditorSection } from "@/components/setup/form-parts";
import { useSetupEditor } from "@/components/setup/use-setup-editor";
import type { FleetEnvironmentConditionType } from "@/generated/conditions";
import type { Environment } from "@/generated/crds/environment";
import { environmentReady } from "@/lib/environments";
import { equalJSON } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { BasicsSection } from "./basics-section";
import {
  createChange,
  deleteChange,
  draftErrors,
  draftOf,
  type EnvironmentDraft,
  specOf,
  updateChange,
} from "./draft";
import { Swatch } from "./environment-name";
import { environmentImpact, withoutKept } from "./impact";
import { ImpactLine, impactLines } from "./impact-line";
import { NamespaceCreationSection } from "./namespace-creation-section";
import { NamespacesSection } from "./namespaces-section";
import { ReferencesSection } from "./references-section";
import { labelledNamespaces } from "./rows";
import { type EnvironmentSources, useEnvironmentSources } from "./sources";

const READY: FleetEnvironmentConditionType = "Ready";

/** EnvironmentActions are the writes of the editor: the server actions on the page, fakes in tests and stories. */
export type EnvironmentActions = {
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
  previewLabel: (change: Change) => Promise<Preview>;
  applyLabel: (change: Change) => Promise<WriteResult>;
};

/**
 * EnvironmentEditor edits the environment name of sources, or a new one
 * without name, with the form state of useSetupEditor. A save and a delete
 * each show a review with a dry run before the write.
 */
export function EnvironmentEditor({
  sources,
  name,
  mode,
  actions,
}: {
  sources: EnvironmentSources & { errors: ReadError[] };
  name?: string;
  mode: WriteMode;
  actions: EnvironmentActions;
}): ReactNode {
  const isNew = name === undefined;
  const live = isNew
    ? undefined
    : sources.environments.find((e) => e.metadata.name === name);
  const [labelErrors, setLabelErrors] = useState<ValidationError[]>([]);
  const editor = useSetupEditor({
    live,
    isNew,
    listed: sources.environments.map((e) => e.metadata.name ?? ""),
    draftOf,
    listHref: LIST_HREF,
    hrefOf,
    onReset: () => setLabelErrors([]),
  });
  const { base, draft, reviewing, locked } = editor;
  const { toast, show, dismiss } = useWriteToast();

  const errors = draftErrors(draft, {
    isNew,
    taken: sources.environments.map((e) => e.metadata.name ?? ""),
  });
  const count = changeCount(base, draft, isNew);
  const impact = useMemo(
    () => environmentImpact(base, draft, sources),
    [base, draft, sources],
  );
  const members = labelledNamespaces(name ?? "", sources.namespaces);
  const namespacesUnread = sources.errors.some((e) => e.kind === "Namespace");
  const blocked =
    errors[0]?.message ??
    (labelErrors.length > 0
      ? "Correct the fixed labels of a new namespace."
      : undefined);
  const setSpec = (spec: EnvironmentDraft["spec"]) =>
    editor.setDraft((d) => ({ ...d, spec }));

  if (!isNew && !live && !base) {
    return (
      <p role="alert" className="text-sm text-bad">
        The environment {name} does not exist.
      </p>
    );
  }

  const ready = live && environmentReady(live);
  const condition = live?.status?.conditions?.find((c) => c.type === READY);
  const title = draft.spec.displayName || draft.name || "New environment";
  const deleting = reviewing?.kind === "delete";
  // A namespace can get the label, or the namespaces can turn unreadable,
  // after the delete review opened.
  if (deleting && (members.length > 0 || namespacesUnread)) editor.cancel();

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-3">
          <Swatch color={draft.spec.color} />
          <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
            {title}
          </h1>
          {condition && <StatusChip condition={condition} />}
        </div>
        {!isNew && title !== name && (
          <p className="font-mono text-sm text-muted">{name}</p>
        )}
      </header>
      {sources.errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {ready && !ready.ready && (
        <div
          role="alert"
          className="flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text"
        >
          <CircleAlert
            aria-hidden
            className="mt-0.5 size-4 shrink-0 text-bad"
          />
          <p>{ready.message}</p>
        </div>
      )}
      {!isNew && !live && (
        <p role="alert" className="text-sm text-bad">
          The environment {name} was deleted.
        </p>
      )}
      <fieldset
        disabled={locked || (!isNew && !live)}
        className="flex min-w-0 flex-col gap-4"
      >
        <BasicsSection
          draft={draft}
          isNew={isNew}
          errors={errors}
          onName={(n) => editor.setDraft((d) => ({ ...d, name: n }))}
          onSpec={setSpec}
        />
        <EditorSection title="Namespaces">
          <NamespacesSection
            environment={name ?? ""}
            isNew={isNew}
            sources={sources}
            mode={mode}
            actions={actions}
            show={show}
          />
          <NamespaceCreationSection
            key={editor.generation}
            value={draft.spec.namespaceCreation}
            errors={errors}
            onChange={(namespaceCreation, rowErrors) => {
              setSpec({ ...draft.spec, namespaceCreation });
              setLabelErrors(rowErrors);
            }}
          />
        </EditorSection>
        <ReferencesSection
          spec={draft.spec}
          sources={sources}
          errors={errors}
          onSpec={setSpec}
        />
        <EditorSection title="When a claim is deleted">
          <DeletionPolicyField
            value={draft.spec.deletionPolicy ?? "Delete"}
            onChange={(deletionPolicy) =>
              setSpec({ ...draft.spec, deletionPolicy })
            }
          />
        </EditorSection>
        {!isNew && live && (
          <EditorSection title="Delete the environment">
            {namespacesUnread ? (
              <p className="text-sm text-muted">
                Delete is possible when the fleet manager can read the
                namespaces.
              </p>
            ) : members.length > 0 ? (
              <p className="text-sm text-muted">
                Delete is possible when no namespace carries the label of this
                environment.
              </p>
            ) : (
              <TypedConfirm
                name={name ?? ""}
                confirmLabel="Review the delete"
                onConfirm={() =>
                  editor.review({ change: deleteChange(live), kind: "delete" })
                }
              >
                Claims cannot pick this environment any more. No namespace
                carries its label.
              </TypedConfirm>
            )}
          </EditorSection>
        )}
      </fieldset>
      {reviewing ? (
        <section
          aria-label={deleting ? "Review the delete" : "Review the environment"}
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Changes to the environment"
            effects={
              deleting
                ? [
                    `The environment ${name} is deleted.`,
                    ...(mode === "git"
                      ? [
                          "In Git mode the delete is a commit that removes the file of the environment. GitOps must prune the Environment to delete it from the cluster.",
                        ]
                      : []),
                  ]
                : (preview) =>
                    impactLines(
                      withoutKept(impact, keptSpec(preview, draft.name)),
                    )
            }
            danger={deleting}
            confirmLabel={deleting ? "Delete the environment" : "Save"}
            gitConfirmLabel={deleting ? "Push the delete to Git" : undefined}
            savedMessage={
              deleting
                ? `The environment ${name} is deleted.`
                : `The environment ${draft.name} is saved.`
            }
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
          <div className="flex flex-col gap-3">
            <ImpactLine impact={impact} />
            <ChangeBar
              count={count}
              blocked={blocked}
              onDiscard={editor.discard}
              onReview={() => {
                const change = isNew
                  ? createChange(draft)
                  : base && updateChange(base, draft);
                if (change) editor.review({ change, kind: "save" });
              }}
            />
          </div>
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

const LIST_HREF = "/setup/environments";

function hrefOf(name: string): string {
  return `${LIST_HREF}/${encodeURIComponent(name)}`;
}

// keptSpec returns the spec of the environment name in the dry run of a
// save, the spec as the server keeps it.
function keptSpec(
  preview: Preview,
  name: string,
): Environment["spec"] | undefined {
  const after = preview.objects.find(
    (o) => o.ref.kind === "Environment" && o.ref.name === name,
  )?.after;
  return after?.spec as Environment["spec"] | undefined;
}

/** EnvironmentEditorLive is EnvironmentEditor on the live sources and the server actions of the page. */
export function EnvironmentEditorLive({
  initial,
  name,
  mode,
}: {
  initial: EnvironmentSources;
  name?: string;
  mode: WriteMode;
}) {
  const sources = useEnvironmentSources(initial);
  return (
    <EnvironmentEditor
      sources={sources}
      name={name}
      mode={mode}
      actions={SERVER_ACTIONS}
    />
  );
}

const SERVER_ACTIONS: EnvironmentActions = {
  previewChange: previewEnvironmentChange,
  applyChange: applyEnvironmentChange,
  previewLabel: previewNamespaceLabel,
  applyLabel: applyNamespaceLabel,
};

const POLICIES = [
  ["Delete", "Delete the resources of the claim"],
  ["Orphan", "Keep the resources of the claim"],
] as const;

function DeletionPolicyField({
  value,
  onChange,
}: {
  value: "Delete" | "Orphan";
  onChange: (value: "Delete" | "Orphan") => void;
}) {
  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="mb-1 text-sm text-muted">
        This is the default for new claims.
      </legend>
      {POLICIES.map(([policy, text]) => (
        <label key={policy} className="flex items-center gap-2 text-sm">
          <input
            type="radio"
            name="deletion-policy"
            className="size-4 accent-accent"
            checked={value === policy}
            onChange={() => onChange(policy)}
          />
          {text}
        </label>
      ))}
    </fieldset>
  );
}

// changeCount counts the top-level fields of the spec that the draft
// changes, and the name of a new environment.
function changeCount(
  base: Environment | undefined,
  draft: EnvironmentDraft,
  isNew: boolean,
): number {
  const before = specOf(draftOf(base)) as Record<string, unknown>;
  const after = specOf(draft) as Record<string, unknown>;
  const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
  const changed = [...keys].filter((k) => !equalJSON(before[k], after[k]));
  return changed.length + (isNew && draft.name !== "" ? 1 : 0);
}
