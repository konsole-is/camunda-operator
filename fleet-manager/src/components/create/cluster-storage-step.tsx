"use client";

import { useMemo } from "react";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { SectionCards } from "@/components/fleet/section-cards";
import type { RenderedHost } from "@/lib/claims/hosts";
import { type ClusterDraft, draftSelections } from "@/lib/clusters/draft";
import { ACCESS_SECTION } from "@/lib/labels";
import {
  type BuiltinContext,
  type DeletionChoice,
  type DeletionPolicy,
  type PluginSelection,
  type SectionView,
  bucketNote,
} from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { DeletionChoiceField } from "@/components/fleet/deletion-choice";

/**
 * ClusterStorageStep is step 2 of the cluster create flow: one SectionCards
 * for each view, in order. A section that the step showed before restores
 * its selections from draft.storage; any other preselects its default.
 * Under the cards of a section it shows the bucket line of each picked
 * built-in option, with the presets of context, and under the access cards
 * the hosts and their conflicts. A section with no option and no empty
 * option says "Your admin allows no option here." Each view keeps its
 * object while its content does not change. onDeletion gets each pick.
 */
export function ClusterStorageStep({
  draft,
  views,
  registrations,
  sections,
  context,
  hosts,
  conflicts,
  onSection,
  deletion,
  onDeletion,
}: {
  draft: ClusterDraft;
  views: readonly SectionView[];
  registrations: readonly PluginRegistration[];
  sections: readonly PluginSection[];
  context: BuiltinContext;
  hosts: readonly RenderedHost[];
  conflicts: readonly string[];
  onSection: (
    section: string,
    selections: PluginSelection[],
    errors: ValidationError[],
  ) => void;
  deletion: DeletionChoice;
  onDeletion: (policy: DeletionPolicy) => void;
}) {
  const selections = draftSelections(draft);
  return (
    <div className="flex max-w-3xl min-w-0 flex-col gap-8">
      {views.map((view) => {
        const shown = draft.storage && Object.hasOwn(draft.storage, view.name);
        const picked = shown ? draft.storage![view.name] : [];
        const notes = picked.flatMap((s) => {
          const registration = registrations.find(
            (r) => r.metadata.name === s.registration,
          );
          const note =
            registration &&
            bucketNote(registration, selections, sections, context);
          return note ? [note] : [];
        });
        return (
          <StorageSection
            key={view.name}
            view={view}
            defaultValue={shown ? picked : undefined}
            notes={notes}
            hosts={view.name === ACCESS_SECTION ? hosts : []}
            conflicts={view.name === ACCESS_SECTION ? conflicts : []}
            onChange={(next, errors) => onSection(view.name, next, errors)}
          />
        );
      })}
      <DeletionChoiceField
        subject="cluster"
        choice={deletion}
        onChange={onDeletion}
      />
    </div>
  );
}

function StorageSection({
  view,
  defaultValue,
  notes,
  hosts,
  conflicts,
  onChange,
}: {
  view: SectionView;
  defaultValue?: PluginSelection[];
  notes: readonly string[];
  hosts: readonly RenderedHost[];
  conflicts: readonly string[];
  onChange: (selections: PluginSelection[], errors: ValidationError[]) => void;
}) {
  // SectionCards takes a new section object as new options and resets what
  // it derived, so a live list update with the same content keeps the object.
  const content = JSON.stringify(view);
  const section = useMemo(() => JSON.parse(content) as SectionView, [content]);
  const empty =
    section.selection === "Many" ||
    section.required ||
    !section.emptyOptionLabel;
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <SectionCards
        section={section}
        defaultValue={defaultValue}
        onChange={onChange}
      />
      {section.options.length === 0 && empty && (
        <p className="text-sm text-muted">Your admin allows no option here.</p>
      )}
      {notes.map((note) => (
        <p key={note} className="text-sm text-text-2">
          {note}
        </p>
      ))}
      {hosts.length > 0 && (
        <div className="text-xs text-muted">
          <p>The cluster gets these hosts:</p>
          <ul className="font-mono text-text-2">
            {hosts.map((h) => (
              <li key={h.parameter}>{h.host}</li>
            ))}
          </ul>
        </div>
      )}
      {conflicts.length > 0 && (
        <ul role="alert" className="flex flex-col gap-0.5 text-xs text-bad">
          {conflicts.map((conflict) => (
            <li key={conflict}>{conflict}</li>
          ))}
        </ul>
      )}
    </div>
  );
}
