"use client";

import { type ReactNode, useState } from "react";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { ChangeBar } from "@/components/fleet/change-bar";
import { ChangeReview } from "@/components/fleet/change-review";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import {
  secretRefsUnion,
  withSecretRefs,
} from "@/components/fleet/secret-refs-union";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { TypedConfirm } from "@/components/fleet/typed-confirm";
import {
  EditorSection,
  SelectField,
  TextField,
} from "@/components/setup/form-parts";
import { useSetupEditor } from "@/components/setup/use-setup-editor";
import type {
  CamundaRelease,
  CamundaReleaseSpec,
} from "@/generated/crds/camundarelease";
import {
  type ReadError,
  type SourceReads,
  unreadDeleteText,
  unreadKinds,
} from "@/lib/k8s/unread";
import { minorOf } from "@/lib/merge";
import type { ValidationError } from "@/lib/schema";
import { releaseUsage } from "@/lib/usage";
import type { WriteMode } from "@/lib/write/types";
import type { ImagesTable } from "@/startup/bundled-data";
import {
  changeCount,
  copyDraft,
  createChange,
  deleteChange,
  draftErrors,
  draftOf,
  RELEASE_CHANGE_NOTICE,
  saveEffects,
  updateChange,
} from "./draft";
import { namedSecretRefs } from "./env";
import { EnvScopes } from "./env-scopes";
import { releaseImages } from "./images";
import { ImagesPanel } from "./images-panel";
import type { ReleaseActions } from "./release-detail";
import { releaseRows } from "./rows";
import type { ReleasesData } from "./sources";

const LIST_HREF = "/setup/releases";

function hrefOf(name: string): string {
  return `${LIST_HREF}/${encodeURIComponent(name)}`;
}

/**
 * ReleaseSettings edits the release name of sources, or a new release
 * without name, with the form state of useSetupEditor. A new release starts
 * as a copy of the release copyFrom names, when it names one. secrets are
 * the Secret refs of each namespace whose Secrets an env entry can name. A
 * save and a delete each show a review with a dry run before the write.
 * Without mode the form is disabled.
 */
export function ReleaseSettings({
  sources,
  name,
  copyFrom,
  imagesTable,
  secrets,
  mode,
  actions,
}: {
  sources: ReleasesData & { errors: ReadError[] } & SourceReads;
  name?: string;
  copyFrom?: string;
  imagesTable: ImagesTable;
  secrets: Readonly<Record<string, SecretRefs>>;
  mode?: WriteMode;
  actions: ReleaseActions;
}): ReactNode {
  const isNew = name === undefined;
  const names = sources.releases.map((r) => r.metadata.name ?? "");
  const releaseNamed = (n: string): CamundaRelease | undefined =>
    sources.releases.find((r) => r.metadata.name === n);
  const live = name === undefined ? undefined : releaseNamed(name);
  const [copy, setCopy] = useState(() =>
    copyFrom !== undefined && names.includes(copyFrom) ? copyFrom : "",
  );
  const [envErrors, setEnvErrors] = useState<ValidationError[]>([]);
  const [writing, setWriting] = useState(false);
  const editor = useSetupEditor({
    live,
    isNew,
    listed: names,
    draftOf: (release: CamundaRelease | undefined) =>
      isNew ? copyDraft(releaseNamed(copy), "") : draftOf(release),
    listHref: LIST_HREF,
    hrefOf,
    onReset: () => setEnvErrors([]),
  });
  const { base, draft, reviewing, locked } = editor;
  const { toast, show, dismiss } = useWriteToast();

  const errors = [...draftErrors(draft, { isNew, taken: names }), ...envErrors];
  const count = changeCount(base, draft, isNew);
  const usage = releaseUsage(name ?? "", sources);
  const deletable =
    !isNew &&
    (releaseRows(sources).find((r) => r.name === name)?.deletable ?? false);
  const unread = unreadKinds(sources);
  const union = secretRefsUnion(secrets);
  const refs = withSecretRefs(union.refs, namedSecretRefs(draft.spec));
  const errorAt = (path: string) =>
    errors.find((e) => e.path === path)?.message;
  const setSpec = (update: (spec: CamundaReleaseSpec) => CamundaReleaseSpec) =>
    editor.setDraft((d) => ({ ...d, spec: update(d.spec) }));
  const setVersion = (
    block: "connectors" | "elasticsearch" | "databaseServer",
    version: string,
  ) => setSpec((spec) => ({ ...spec, [block]: { ...spec[block], version } }));

  if (!isNew && !live && !base) {
    return (
      <p role="alert" className="text-sm text-bad">
        The release {name} does not exist.
      </p>
    );
  }

  const deleting = reviewing?.kind === "delete";
  // A claim or an environment can name the release, or a list can turn
  // unreadable, after the delete review opened. A delete that already runs
  // cannot stop.
  if (deleting && !writing && !deletable) editor.cancel();
  const savedName = isNew ? draft.name : name;

  return (
    <div className="flex flex-col gap-6">
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
      {!isNew && !live && (
        <p role="alert" className="text-sm text-bad">
          The release {name} was deleted.
        </p>
      )}
      <fieldset
        disabled={locked || (!isNew && !live) || !mode}
        className="flex min-w-0 flex-col gap-4"
      >
        {isNew && (
          <EditorSection title="Basics">
            <SelectField
              label="Copy from"
              value={copy}
              options={[
                ["", "Nothing, start empty"],
                ...names.map((n) => [n, n] as const),
              ]}
              onChange={(source) => {
                setCopy(source);
                setEnvErrors([]);
                editor.setDraft((d) => copyDraft(releaseNamed(source), d.name));
              }}
            />
            <TextField
              label="Release name"
              value={draft.name}
              mono
              hint="A DNS name of at most 253 characters. It cannot change later."
              error={errorAt("name")}
              onChange={(n) => editor.setDraft((d) => ({ ...d, name: n }))}
            />
          </EditorSection>
        )}
        <EditorSection title="Versions">
          <TextField
            label="Camunda version"
            value={draft.spec.version}
            mono
            placeholder="8.9.0"
            error={errorAt("spec.version")}
            onChange={(version) => setSpec((spec) => ({ ...spec, version }))}
          />
          <TextField
            label="Connectors version"
            value={draft.spec.connectors?.version ?? ""}
            mono
            hint="Leave it empty to let each cluster set its own."
            error={errorAt("spec.connectors.version")}
            onChange={(v) => setVersion("connectors", v)}
          />
          <TextField
            label="Elasticsearch version"
            value={draft.spec.elasticsearch?.version ?? ""}
            mono
            error={errorAt("spec.elasticsearch.version")}
            onChange={(v) => setVersion("elasticsearch", v)}
          />
          <TextField
            label="PostgreSQL version"
            value={draft.spec.databaseServer?.version ?? ""}
            mono
            hint="A major version, such as 17."
            error={errorAt("spec.databaseServer.version")}
            onChange={(v) => setVersion("databaseServer", v)}
          />
        </EditorSection>
        <EditorSection title="Images">
          <ImagesPanel
            rows={releaseImages(draft.spec, imagesTable)}
            minor={minorOf(draft.spec.version)}
            pinned={draft.spec.images}
            onPin={(field, value) =>
              setSpec((spec) => ({
                ...spec,
                images: { ...spec.images, [field]: value },
              }))
            }
          />
        </EditorSection>
        <EditorSection title="Environment variables">
          <EnvScopes
            key={`${editor.generation}-${copy}`}
            spec={draft.spec}
            secrets={refs}
            missingIn={union.missingIn}
            onChange={(update, scopeErrors) => {
              setSpec(update);
              setEnvErrors(scopeErrors);
            }}
          />
        </EditorSection>
        {!isNew && live && (
          <EditorSection title="Delete the release">
            {deletable ? (
              <TypedConfirm
                name={name ?? ""}
                confirmLabel="Review the delete"
                onConfirm={() =>
                  editor.review({
                    change: deleteChange([live]),
                    kind: "delete",
                  })
                }
              >
                No resource uses this release, and no environment offers it.
              </TypedConfirm>
            ) : unread.length > 0 ? (
              <p className="text-sm text-muted">{unreadDeleteText(unread)}</p>
            ) : (
              <p className="text-sm text-muted">
                Delete is possible when no cluster, Elasticsearch cluster,
                database server, or claim names this release and no environment
                allows it.
              </p>
            )}
          </EditorSection>
        )}
      </fieldset>
      {!isNew && !reviewing && (
        <p
          role="note"
          className="rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
        >
          {RELEASE_CHANGE_NOTICE}
        </p>
      )}
      {reviewing && mode ? (
        <section
          aria-label={deleting ? "Review the delete" : "Review the release"}
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Changes to the release"
            effects={
              deleting
                ? [
                    `The release ${name} is deleted.`,
                    ...(mode === "git"
                      ? [
                          "In Git mode the delete is a commit that removes the file of the release. GitOps must prune the CamundaRelease to delete it from the cluster.",
                        ]
                      : []),
                  ]
                : [
                    ...saveEffects(base, draft, usage),
                    ...(isNew ? [] : [RELEASE_CHANGE_NOTICE]),
                  ]
            }
            confirm={
              deleting
                ? {
                    label: "Delete the release",
                    gitLabel: "Push the delete to Git",
                    danger: true,
                  }
                : { label: isNew ? "Create" : "Save" }
            }
            savedMessage={
              deleting
                ? `The release ${name} is deleted.`
                : `The release ${savedName} is ${isNew ? "created" : "saved"}.`
            }
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              editor.written(result, draft.name);
            }}
            onCancel={editor.cancel}
            onWriting={setWriting}
          />
        </section>
      ) : (
        count > 0 &&
        !locked && (
          <ChangeBar
            count={count}
            blocked={errors[0]?.message}
            onDiscard={editor.discard}
            onReview={() => {
              const change = isNew
                ? createChange(draft)
                : base && updateChange(base, draft);
              if (change) editor.review({ change, kind: "save" });
            }}
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
