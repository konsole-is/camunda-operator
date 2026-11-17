"use client";

import { CircleAlert } from "lucide-react";
import { type ReactNode, useState } from "react";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { ChangeBar } from "@/components/fleet/change-bar";
import { ChangeReview } from "@/components/fleet/change-review";
import { StatusChip } from "@/components/fleet/status-chip";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { TypedConfirm } from "@/components/fleet/typed-confirm";
import {
  type ReadError,
  type SourceReads,
  unreadDeleteText,
} from "@/lib/k8s/unread";
import { EditorSection } from "@/components/setup/form-parts";
import { useSetupEditor } from "@/components/setup/use-setup-editor";
import { defaultRepositories } from "@/lib/merge";
import { equalJSON } from "@/lib/policy";
import { platformUserCount } from "@/lib/usage";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import type { ImagesTable } from "@/startup/bundled-data";
import { AuthSection } from "./auth-section";
import {
  deleteChange,
  draftErrors,
  draftOf,
  type PlatformDraft,
  specOf,
  updateChange,
} from "./draft";
import { ImagesSection } from "./images-section";
import {
  DELETE_EFFECT,
  ROLL_NOTICE,
  unreadMinorKinds,
  unreadUserKinds,
  userMinors,
  usersOf,
} from "./model";
import { type PlatformSources, usePlatformSources } from "./sources";
import { UsersSection } from "./users-section";

/** PlatformActions are the writes of the detail: the server actions on the page, fakes in tests and stories. */
export type PlatformActions = {
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
};

const LIST_HREF = "/setup/platform";

function hrefOf(name: string): string {
  return `${LIST_HREF}/${encodeURIComponent(name)}`;
}

/**
 * PlatformDetail edits the platform config name of sources with the form
 * state of useSetupEditor. A save and a delete each show a review with a dry
 * run before the write. The delete is offered only while nothing uses the
 * config and no list of a kind that can use it failed or is stale. Without
 * mode the form is disabled.
 */
export function PlatformDetail({
  sources,
  imagesTable,
  name,
  mode,
  actions,
}: {
  sources: PlatformSources & { errors: ReadError[] } & SourceReads;
  imagesTable: ImagesTable;
  name: string;
  mode?: WriteMode;
  actions: PlatformActions;
}): ReactNode {
  const live = sources.platformConfigs.find((c) => c.metadata.name === name);
  const [writing, setWriting] = useState(false);
  const editor = useSetupEditor({
    live,
    isNew: false,
    listed: sources.platformConfigs.map((c) => c.metadata.name ?? ""),
    draftOf,
    listHref: LIST_HREF,
    hrefOf,
  });
  const { base, draft, reviewing, locked } = editor;
  const { toast, show, dismiss } = useWriteToast();
  const users = usersOf(name, sources);
  const userCount = platformUserCount(users);
  const unread = unreadUserKinds(sources);
  const errors = draftErrors(draft);
  const count = changeCount(specOf(draftOf(base)), specOf(draft));
  const deleting = reviewing?.kind === "delete";
  // A user can appear, or a kind that can use the config can turn
  // unreadable, after the delete review opened. A delete that already runs
  // cannot stop.
  if (deleting && !writing && (userCount > 0 || unread.length > 0))
    editor.cancel();

  if (!live && !base) {
    return (
      <p role="alert" className="text-sm text-bad">
        The platform config {name} does not exist.
      </p>
    );
  }

  const condition = live?.status?.conditions?.find((c) => c.type === "Ready");
  const setDraft = (next: PlatformDraft) => editor.setDraft(() => next);
  const namespaces = sources.namespaces.map((n) => n.metadata?.name ?? "");

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-center gap-3">
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em] break-all">
          {name}
        </h1>
        {condition && <StatusChip condition={condition} />}
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
      {condition?.status === "False" && (
        <div
          role="alert"
          className="flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text"
        >
          <CircleAlert
            aria-hidden
            className="mt-0.5 size-4 shrink-0 text-bad"
          />
          <p className="break-words">{condition.message}</p>
        </div>
      )}
      {!live && (
        <p role="alert" className="text-sm text-bad">
          The platform config {name} was deleted.
        </p>
      )}
      <p
        role="note"
        className="rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
      >
        {ROLL_NOTICE}
      </p>
      <fieldset
        key={editor.generation}
        disabled={locked || !live || !mode}
        className="flex min-w-0 flex-col gap-4"
      >
        <AuthSection
          draft={draft}
          namespaces={namespaces}
          errors={errors}
          onChange={setDraft}
        />
        <ImagesSection
          images={draft.images}
          defaults={defaultRepositories(
            imagesTable,
            userMinors(users, sources),
          )}
          errors={errors}
          usersKnown={unread.length === 0}
          unreadMinorKinds={unreadMinorKinds(sources, users)}
          versionsUnresolved={users.clusters.length + users.planes.length > 0}
          onChange={(images) => setDraft({ ...draft, images })}
        />
        <UsersSection users={users} unread={unread} />
        {live && (
          <EditorSection title="Delete the platform config">
            {unread.length > 0 ? (
              <p className="text-sm text-muted">{unreadDeleteText(unread)}</p>
            ) : userCount > 0 ? (
              <p className="text-sm text-muted">
                Delete is possible when no cluster, management plane, database
                server, claim, or environment uses this config.
              </p>
            ) : (
              <TypedConfirm
                name={name}
                confirmLabel="Review the delete"
                onConfirm={() =>
                  editor.review({ change: deleteChange(live), kind: "delete" })
                }
              >
                {DELETE_EFFECT}
              </TypedConfirm>
            )}
          </EditorSection>
        )}
      </fieldset>
      {reviewing && mode ? (
        <section
          aria-label={
            deleting ? "Review the delete" : "Review the platform config"
          }
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Changes to the platform config"
            effects={
              deleting
                ? [
                    `The platform config ${name} is deleted.`,
                    DELETE_EFFECT,
                    ...(mode === "git"
                      ? [
                          "In Git mode the delete is a commit that removes the file of the platform config. GitOps must prune the CamundaPlatformConfig to delete it from the cluster.",
                        ]
                      : []),
                  ]
                : [ROLL_NOTICE]
            }
            confirm={
              deleting
                ? {
                    label: "Delete the platform config",
                    gitLabel: "Push the delete to Git",
                    danger: true,
                  }
                : { label: "Save" }
            }
            savedMessage={
              deleting
                ? `The platform config ${name} is deleted.`
                : `The platform config ${name} is saved.`
            }
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
      ) : (
        count > 0 &&
        !locked &&
        base && (
          <ChangeBar
            count={count}
            blocked={errors[0]?.message}
            onDiscard={editor.discard}
            onReview={() =>
              editor.review({ change: updateChange(base, draft), kind: "save" })
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

/** PlatformDetailLive is PlatformDetail on the live sources. */
export function PlatformDetailLive({
  initial,
  imagesTable,
  name,
  mode,
  actions,
}: {
  initial: PlatformSources;
  imagesTable: ImagesTable;
  name: string;
  mode?: WriteMode;
  actions: PlatformActions;
}) {
  const sources = usePlatformSources(initial);
  return (
    <PlatformDetail
      sources={sources}
      imagesTable={imagesTable}
      name={name}
      mode={mode}
      actions={actions}
    />
  );
}

// changeCount counts the top-level fields of the spec that differ.
function changeCount(a: PlatformDraft, b: PlatformDraft): number {
  const before = a as Record<string, unknown>;
  const after = b as Record<string, unknown>;
  const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
  return [...keys].filter((k) => !equalJSON(before[k], after[k])).length;
}
