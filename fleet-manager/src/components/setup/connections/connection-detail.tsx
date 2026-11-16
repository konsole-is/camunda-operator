"use client";

import { type ReactNode, useState } from "react";
import { Panel, Term, Terms } from "@/components/cluster/claim-tab";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { ChangeBar } from "@/components/fleet/change-bar";
import { ChangeReview } from "@/components/fleet/change-review";
import { StatusChip } from "@/components/fleet/status-chip";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { TypedConfirm } from "@/components/fleet/typed-confirm";
import type { ReadError } from "@/components/planes/planes-table";
import { EditorSection } from "@/components/setup/form-parts";
import {
  type SourceReads,
  unreadDeleteText,
  unreadKinds,
} from "@/components/setup/releases/rows";
import { useSetupEditor } from "@/components/setup/use-setup-editor";
import {
  type ContractRef,
  contractKey,
  contractUsage,
  contractWriter,
  unreadUserKinds,
  writerEditable,
  writerLabel,
} from "@/lib/usage";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { requestsOf } from "./connection-list";
import { ContractForm } from "./contract-form";
import {
  type ContractDraft,
  changeCount,
  deleteChange,
  draftErrors,
  draftOf,
  updateChange,
} from "./draft";
import { type ContractObject, contractFacts } from "./facts";
import { readyOf } from "@/lib/policy";
import { saveEffects } from "./impact";
import { SecretRefView } from "./secret-ref-view";
import {
  type ConnectionPageData,
  type ConnectionSources,
  useConnectionSources,
} from "./sources";
import { CONNECTION_TABS, contractHref, contractsOf } from "./tabs";
import { UsersTable } from "./users-table";

/** ContractActions are the writes of the detail: the server actions on the page, fakes in tests and stories. */
export type ContractActions = {
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
};

/**
 * ConnectionDetail is the page of one contract: its own Ready, its writer,
 * its settings, and the objects that use it. A contract that sources does
 * not hold shows as missing, unless its kind is not read. One that leaves
 * sources while the page shows it keeps its last copy and shows as deleted.
 * Each of the read errors shows above.
 *
 * With actions, a contract that a person may change (writerEditable) has a
 * form with the form state of useSetupEditor, and a delete. A save and a
 * delete each show a review with a dry run before the write. Without mode
 * the form is disabled. Any other contract keeps the read view.
 */
export function ConnectionDetail({
  target,
  sources,
  mode,
  actions,
}: {
  target: ContractRef;
  sources: ConnectionSources & { errors: ReadError[] } & SourceReads;
  mode?: WriteMode;
  actions?: ContractActions;
}): ReactNode {
  const contracts = contractsOf(sources, target.kind);
  const live = contracts.find(
    (c) =>
      c.metadata.name === target.name &&
      (target.namespace === undefined ||
        c.metadata.namespace === target.namespace),
  );
  const tab = CONNECTION_TABS.find((t) => t.kind === target.kind)!;
  const editor = useSetupEditor<ContractObject, ContractDraft>({
    live,
    isNew: false,
    listed: contracts
      .filter((c) => c.metadata.namespace === target.namespace)
      .map((c) => c.metadata.name ?? ""),
    draftOf,
    listHref: `/setup/connections?tab=${tab.value}`,
    hrefOf: (name, namespace) =>
      contractHref({ kind: target.kind, namespace, name }),
  });
  const [writing, setWriting] = useState(false);
  const { toast, show, dismiss } = useWriteToast();
  const unread = unreadUserKinds(target.kind, unreadKinds(sources));
  const users = contractUsage(sources).get(contractKey(target)) ?? [];
  const { base, draft, reviewing, locked } = editor;
  // After a delete, the page shows the last copy until it moves to the list.
  const shown = live ?? base;
  const writer =
    shown && contractWriter(shown, requestsOf(sources), sources.planes);
  const editable =
    actions !== undefined && writer !== undefined && writerEditable(writer);
  const deleting = reviewing?.kind === "delete";
  // A writer, a user, or a kind that cannot be read can appear after the
  // review opened. A write that already runs cannot stop.
  if (
    reviewing &&
    !writing &&
    (!editable || unread.length > 0 || (deleting && users.length > 0))
  )
    editor.cancel();

  const errors = sources.errors.map((e) => (
    <p key={e.kind} role="alert" className="text-sm text-bad">
      Could not read {e.kind}: {e.message}
    </p>
  ));
  if (!shown) {
    return (
      <div className="flex flex-col gap-6">
        {errors}
        {!unreadKinds(sources).includes(target.kind) && (
          <p role="alert" className="text-sm text-bad">
            The contract {target.name} does not exist.
          </p>
        )}
      </div>
    );
  }
  const ready = readyOf(shown.status?.conditions);
  const formErrors = draftErrors(target.kind, draft);
  const unreadLine =
    unread.length > 0
      ? `A change is possible when the fleet manager can read ${unread.join(", ")}.`
      : undefined;
  const count = base ? changeCount(base, draft) : 0;
  const namesIn = (kind: "DatabaseConfig" | "DatabaseServerConfig") =>
    unreadKinds(sources).includes(kind)
      ? undefined
      : contractsOf(sources, kind)
          .filter((c) => c.metadata.namespace === target.namespace)
          .map((c) => c.metadata.name ?? "");

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-2">
        <p className="text-sm text-muted">{tab.label}</p>
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]">
            {target.name}
          </h1>
          <StatusChip condition={ready} />
        </div>
        {target.namespace !== undefined && (
          <p className="font-mono text-sm text-muted">{target.namespace}</p>
        )}
      </header>
      {editable && !mode && (
        <p role="alert" className="text-sm text-bad">
          {CHANGES_OFF}
        </p>
      )}
      {errors}
      {!live && (
        <p role="alert" className="text-sm text-bad">
          The contract {target.name} was deleted.
        </p>
      )}
      <div className="grid gap-4 lg:grid-cols-2">
        <Panel label="Status">
          {ready ? (
            <Terms>
              <Term label="Reason">{ready.reason || "None"}</Term>
              <Term label="Message">{ready.message || "None"}</Term>
            </Terms>
          ) : (
            <p className="text-sm text-muted">No status yet</p>
          )}
        </Panel>
        <Panel label="Writer">
          <div className="flex flex-col gap-1 text-sm">
            {writer && <p>{writerLabel(writer)}</p>}
            {writer && !writerEditable(writer) && (
              <p className="text-muted">
                Managed by {writerLabel(writer)}. Change it there.
              </p>
            )}
          </div>
        </Panel>
      </div>
      {editable ? (
        <fieldset
          key={`settings-${editor.generation}`}
          disabled={locked || !live || !mode}
          className="flex min-w-0 flex-col gap-4"
        >
          <EditorSection title="Settings">
            {unreadLine && <p className="text-sm text-muted">{unreadLine}</p>}
            <ContractForm
              kind={target.kind}
              namespace={target.namespace}
              draft={draft}
              errors={formErrors}
              namespaces={sources.namespaces.map((n) => n.metadata?.name ?? "")}
              databaseConfigs={namesIn("DatabaseConfig")}
              databaseServerConfigs={namesIn("DatabaseServerConfig")}
              onChange={(next) => editor.setDraft(() => next)}
            />
          </EditorSection>
        </fieldset>
      ) : (
        <Panel label="Settings">
          <Terms>
            {contractFacts(shown).map((fact) => (
              <Term key={fact.label} label={fact.label} note={fact.note}>
                {fact.secret ? (
                  <SecretRefView secret={fact.secret} />
                ) : (
                  fact.value
                )}
              </Term>
            ))}
          </Terms>
        </Panel>
      )}
      {/* Used by stays out of the form, so its table keeps its controls
          while a review locks the form. */}
      <Panel label="Used by">
        <UsersTable users={users} unread={unread} />
      </Panel>
      {editable && live && (
        <fieldset
          key={`delete-${editor.generation}`}
          disabled={locked || !mode}
          className="flex min-w-0 flex-col gap-4"
        >
          <EditorSection title="Delete the contract">
            {unread.length > 0 ? (
              <p className="text-sm text-muted">{unreadDeleteText(unread)}</p>
            ) : users.length > 0 ? (
              <p className="text-sm text-muted">
                Delete is possible when no resource uses this contract.
              </p>
            ) : (
              <TypedConfirm
                name={target.name}
                confirmLabel="Review the delete"
                onConfirm={() =>
                  editor.review({
                    change: deleteChange(live),
                    kind: "delete",
                  })
                }
              >
                The contract {target.name} is deleted.
              </TypedConfirm>
            )}
          </EditorSection>
        </fieldset>
      )}
      {editable && reviewing && mode ? (
        <section
          aria-label={deleting ? "Review the delete" : "Review the contract"}
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Changes to the contract"
            effects={
              deleting
                ? [
                    `The contract ${target.name} is deleted.`,
                    ...(mode === "git"
                      ? [
                          `In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the ${target.kind} to delete it from the cluster.`,
                        ]
                      : []),
                  ]
                : saveEffects(target.kind, users)
            }
            confirm={
              deleting
                ? {
                    label: "Delete the contract",
                    gitLabel: "Push the delete to Git",
                    danger: true,
                  }
                : { label: "Save" }
            }
            savedMessage={
              deleting
                ? `The contract ${target.name} is deleted.`
                : `The contract ${target.name} is saved.`
            }
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              editor.written(result, target.name, target.namespace);
            }}
            onCancel={editor.cancel}
            onWriting={setWriting}
          />
        </section>
      ) : (
        editable &&
        count > 0 &&
        !locked &&
        base && (
          <ChangeBar
            count={count}
            blocked={unreadLine ?? formErrors[0]?.message}
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

/** ConnectionDetailLive is ConnectionDetail on the live sources, starting from initial. */
export function ConnectionDetailLive({
  initial,
  target,
  mode,
  actions,
}: {
  initial: ConnectionPageData;
  target: ContractRef;
  mode?: WriteMode;
  actions?: ContractActions;
}): ReactNode {
  return (
    <ConnectionDetail
      target={target}
      sources={useConnectionSources(initial)}
      mode={mode}
      actions={actions}
    />
  );
}
