"use client";

import { Fragment, type ReactNode, useState } from "react";
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
import {
  type ReadError,
  type SourceReads,
  unreadKinds,
} from "@/lib/k8s/unread";
import { byCodeUnit } from "@/lib/policy";
import type { WriteMode } from "@/lib/write/types";
import { ContractForm } from "./contract-form";
import {
  type ContractCreateDraft,
  type ContractDraft,
  changeCount,
  copyDraft,
  createChange,
  draftErrors,
  kindWords,
  nameErrors,
  namespaceErrors,
  newSpec,
  planeContractHolder,
} from "./draft";
import type { ContractObject } from "./facts";
import { CREATE_EFFECT } from "./impact";
import {
  type ConnectionActions,
  type ConnectionPageData,
  type ConnectionSources,
  useConnectionSources,
} from "./sources";
import {
  CONNECTION_TABS,
  type ConnectionTab,
  connectionsHref,
  contractHref,
  contractsOf,
} from "./tabs";

/**
 * ContractCreate is the create page of a contract of the kind of tab on
 * sources. It starts with the new spec of the kind, or as a copy of the
 * spec of the contract that Copy from names. The create shows a review with
 * a dry run before the write, and the page moves to the new contract once
 * the list shows it. Without mode the form is disabled.
 */
export function ContractCreate({
  tab: tabValue,
  sources,
  mode,
  actions,
}: {
  tab: ConnectionTab;
  sources: ConnectionSources & { errors: ReadError[] } & SourceReads;
  mode?: WriteMode;
  actions: ConnectionActions;
}): ReactNode {
  const tab = CONNECTION_TABS.find((t) => t.value === tabValue)!;
  const kind = tab.kind;
  const namespaced = kind !== "ManagementAuthConfig";
  const contracts = contractsOf(sources, kind);
  const keyOf = (c: ContractObject) =>
    namespaced
      ? `${c.metadata.namespace ?? ""}/${c.metadata.name ?? ""}`
      : (c.metadata.name ?? "");
  const namesIn = (namespace: string) =>
    contracts
      .filter((c) => !namespaced || c.metadata.namespace === namespace)
      .map((c) => c.metadata.name ?? "");
  const [copy, setCopy] = useState("");
  // The namespace is state of its own: listed must know it before the
  // editor gives the draft.
  const [namespace, setNamespace] = useState("");
  const editor = useSetupEditor<
    ContractObject,
    ContractDraft & { name: string }
  >({
    live: undefined,
    isNew: true,
    listed: namesIn(namespace),
    draftOf: () => ({ name: "", spec: newSpec(kind) }),
    listHref: connectionsHref(tab.value),
    hrefOf: (name, ns) => contractHref({ kind, namespace: ns, name }),
    onReset: () => {
      setCopy("");
      setNamespace("");
    },
  });
  const { reviewing, locked } = editor;
  const draft: ContractCreateDraft = { ...editor.draft, namespace };
  const { toast, show, dismiss } = useWriteToast();
  const errors = [
    ...nameErrors(
      kind,
      draft.name,
      namesIn(draft.namespace),
      namespaced
        ? undefined
        : planeContractHolder(draft.name, sources.planes, sources.planeClaims),
    ),
    ...namespaceErrors(kind, draft.namespace),
    ...draftErrors(kind, draft),
  ];
  const errorAt = (path: string) =>
    errors.find((e) => e.path === path)?.message;
  const count =
    changeCount({ spec: newSpec(kind) }, draft) +
    (draft.name !== "" ? 1 : 0) +
    (draft.namespace !== "" ? 1 : 0);
  const namespaces = sources.namespaces
    .map((n) => n.metadata?.name ?? "")
    .sort(byCodeUnit);
  const referenceNames = (
    referenced: "DatabaseConfig" | "DatabaseServerConfig",
  ) =>
    unreadKinds(sources).includes(referenced)
      ? undefined
      : contractsOf(sources, referenced)
          .filter((c) => c.metadata.namespace === draft.namespace)
          .map((c) => c.metadata.name ?? "");
  const copies = contracts.map(keyOf).sort(byCodeUnit);

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-2">
        <p className="text-sm text-muted">{tab.label}</p>
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
          New {kindWords(kind)} contract
        </h1>
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
      <fieldset
        disabled={locked || !mode}
        className="flex min-w-0 flex-col gap-4"
      >
        <EditorSection title="Basics">
          <SelectField
            label="Copy from"
            value={copy}
            options={[
              ["", "Nothing, start empty"],
              ...copies.map((key) => [key, key] as const),
            ]}
            onChange={(source) => {
              setCopy(source);
              const next = copyDraft(
                kind,
                contracts.find((c) => keyOf(c) === source),
                draft,
              );
              setNamespace(next.namespace);
              editor.setDraft(() => ({ name: next.name, spec: next.spec }));
            }}
          />
          <TextField
            label="Name"
            value={draft.name}
            mono
            hint="Lowercase letters, digits, and hyphens. It cannot change later."
            error={errorAt("name")}
            onChange={(name) => editor.setDraft((d) => ({ ...d, name }))}
          />
          {namespaced && (
            <SelectField
              label="Namespace"
              value={draft.namespace}
              options={[
                ["", "Pick a namespace"],
                ...namespaces.map((n) => [n, n] as const),
              ]}
              error={errorAt("namespace")}
              onChange={setNamespace}
            />
          )}
        </EditorSection>
        {/* The Secret pickers keep their own picks, so a new copy mounts
            the settings again. A key on the fieldset would also mount
            Copy from again and drop its focus. */}
        <Fragment key={`${editor.generation}-${copy}`}>
          <EditorSection title="Settings">
            <ContractForm
              kind={kind}
              namespace={draft.namespace}
              draft={draft}
              errors={errors}
              namespaces={namespaces}
              databaseConfigs={referenceNames("DatabaseConfig")}
              databaseServerConfigs={referenceNames("DatabaseServerConfig")}
              choose
              onChange={(next) =>
                editor.setDraft((d) => ({ ...d, spec: next.spec }))
              }
            />
          </EditorSection>
        </Fragment>
      </fieldset>
      {reviewing && mode ? (
        <section
          aria-label="Review the contract"
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Changes to the contract"
            effects={[CREATE_EFFECT]}
            confirm={{ label: "Create" }}
            savedMessage={`The contract ${draft.name} is created.`}
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              editor.written(
                result,
                draft.name,
                namespaced ? draft.namespace : undefined,
              );
            }}
            onCancel={editor.cancel}
          />
        </section>
      ) : (
        count > 0 &&
        !locked && (
          <ChangeBar
            count={count}
            blocked={errors[0]?.message}
            onDiscard={editor.discard}
            onReview={() =>
              editor.review({
                change: createChange(kind, draft.namespace, draft.name, draft),
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

/** ContractCreateLive is ContractCreate on the live sources, starting from initial. */
export function ContractCreateLive({
  initial,
  tab,
  mode,
  actions,
}: {
  initial: ConnectionPageData;
  tab: ConnectionTab;
  mode?: WriteMode;
  actions: ConnectionActions;
}): ReactNode {
  return (
    <ContractCreate
      tab={tab}
      sources={useConnectionSources(initial)}
      mode={mode}
      actions={actions}
    />
  );
}
