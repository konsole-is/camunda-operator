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
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { ReadError, SourceReads } from "@/lib/k8s/unread";
import { defaultRepositories } from "@/lib/merge";
import type { WriteMode } from "@/lib/write/types";
import type { ImagesTable } from "@/startup/bundled-data";
import { AuthSection } from "./auth-section";
import {
  changeCount,
  createChange,
  draftErrors,
  draftOf,
  nameErrors,
  type PlatformCreateDraft,
  type PlatformDraft,
  specOf,
} from "./draft";
import { ImagesSection } from "./images-section";
import {
  CREATE_EFFECT,
  PLATFORM_HREF,
  platformConfigHref,
  unreadMinorKinds,
  unreadUserKinds,
  userMinors,
  usersOf,
} from "./model";
import {
  type PlatformActions,
  type PlatformSnapshot,
  type PlatformSources,
  usePlatformSources,
} from "./sources";

/**
 * PlatformCreate is the create page of a platform config on sources. It
 * starts empty, or as a copy of the spec of the config that Copy from
 * names. The create shows a review with a dry run before the write, and
 * the page moves to the new config once the list shows it. Without mode
 * the form is disabled.
 */
export function PlatformCreate({
  sources,
  imagesTable,
  mode,
  actions,
}: {
  sources: PlatformSources & { errors: ReadError[] } & SourceReads;
  imagesTable: ImagesTable;
  mode?: WriteMode;
  actions: PlatformActions;
}): ReactNode {
  const names = sources.platformConfigs.map((c) => c.metadata.name ?? "");
  const configNamed = (n: string): CamundaPlatformConfig | undefined =>
    sources.platformConfigs.find((c) => c.metadata.name === n);
  const [copy, setCopy] = useState("");
  const editor = useSetupEditor<CamundaPlatformConfig, PlatformCreateDraft>({
    live: undefined,
    isNew: true,
    listed: names,
    draftOf: () => ({ name: "", spec: draftOf(configNamed(copy)) }),
    listHref: PLATFORM_HREF,
    hrefOf: platformConfigHref,
  });
  const { draft, reviewing, locked } = editor;
  const { toast, show, dismiss } = useWriteToast();
  const errors = [...nameErrors(draft.name, names), ...draftErrors(draft.spec)];
  const count =
    changeCount(specOf({}), specOf(draft.spec)) + (draft.name !== "" ? 1 : 0);
  const errorAt = (path: string) =>
    errors.find((e) => e.path === path)?.message;
  const setSpec = (spec: PlatformDraft) =>
    editor.setDraft((d) => ({ ...d, spec }));
  const namespaces = sources.namespaces.map((n) => n.metadata?.name ?? "");
  // A resource can still name a config that was deleted, so the new config
  // can have users from the start.
  const users = usersOf(draft.name, sources);
  const unread = unreadUserKinds(sources);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
        New platform config
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
          <SelectField
            label="Copy from"
            value={copy}
            options={[
              ["", "Nothing, start empty"],
              ...names.map((n) => [n, n] as const),
            ]}
            onChange={(source) => {
              setCopy(source);
              editor.setDraft((d) => ({
                name: d.name,
                spec: draftOf(configNamed(source)),
              }));
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
        </EditorSection>
        {/* The Secret pickers keep their own picks, so a new copy mounts
            the sections again. A key on the fieldset would also mount
            Copy from again and drop its focus. */}
        <Fragment key={`${editor.generation}-${copy}`}>
          <AuthSection
            draft={draft.spec}
            namespaces={namespaces}
            errors={errors}
            onChange={setSpec}
          />
          <ImagesSection
            images={draft.spec.images}
            defaults={defaultRepositories(
              imagesTable,
              userMinors(users, sources),
            )}
            errors={errors}
            usersKnown={unread.length === 0}
            unreadMinorKinds={unreadMinorKinds(sources, users)}
            versionsUnresolved={users.clusters.length + users.planes.length > 0}
            onChange={(images) => setSpec({ ...draft.spec, images })}
          />
        </Fragment>
      </fieldset>
      {reviewing && mode ? (
        <section
          aria-label="Review the platform config"
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Changes to the platform config"
            effects={[CREATE_EFFECT]}
            confirm={{ label: "Create" }}
            savedMessage={`The platform config ${draft.name} is created.`}
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
            blocked={errors[0]?.message}
            onDiscard={editor.discard}
            onReview={() =>
              editor.review({
                change: createChange(draft.name, draft.spec),
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

/** PlatformCreateLive is PlatformCreate on the live sources, starting from initial. */
export function PlatformCreateLive({
  initial,
  imagesTable,
  mode,
  actions,
}: {
  initial: PlatformSnapshot;
  imagesTable: ImagesTable;
  mode?: WriteMode;
  actions: PlatformActions;
}): ReactNode {
  const sources = usePlatformSources(initial);
  return (
    <PlatformCreate
      sources={sources}
      imagesTable={imagesTable}
      mode={mode}
      actions={actions}
    />
  );
}
