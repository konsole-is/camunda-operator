"use client";

import { type ReactNode, useState } from "react";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { ChangeReview } from "@/components/fleet/change-review";
import { DataTable } from "@/components/fleet/data-table";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import type { ReadError } from "@/components/planes/planes-table";
import { Button } from "@/components/ui/button";
import type { Change, WriteMode } from "@/lib/write/types";
import { presetDeleteChange } from "./delete";
import { FactsList } from "./facts-list";
import { PRESET_KINDS, presetHref, presetLabel } from "./kinds";
import { changedLabel, type PresetRow, presetRows, usedByLabel } from "./rows";
import {
  type PresetActions,
  type PresetSources,
  usePresetSources,
} from "./sources";

/** GIT_DELETE_EFFECT is the effect line of a delete of presets in Git mode. */
export const GIT_DELETE_EFFECT =
  "In Git mode the delete is a commit that removes the file of each preset. GitOps must prune the presets to delete them from the cluster.";

/**
 * PresetList is the list page of the presets of the three kinds. Each of
 * errors shows above the table. The selected presets that can be deleted
 * go to one delete review. The review closes before its write starts when
 * a preset of it can no longer be deleted. Without mode nothing can be
 * deleted.
 */
export function PresetList({
  rows,
  errors,
  mode,
  actions,
}: {
  rows: PresetRow[];
  errors: ReadError[];
  mode?: WriteMode;
  actions: PresetActions;
}): ReactNode {
  const [selected, setSelected] = useState<string[]>([]);
  const [reviewing, setReviewing] = useState<{
    ids: string[];
    change: Change;
  }>();
  const [writing, setWriting] = useState(false);
  const [writes, setWrites] = useState(0);
  const { toast, show, dismiss } = useWriteToast();

  const byId = new Map(rows.map((row) => [row.id, row]));
  const chosen = selected.flatMap((id) => {
    const row = byId.get(id);
    return row?.deletable ? [row] : [];
  });
  // A preset can get a user, or an environment can allow it, after the
  // review opened. A write that already runs cannot stop.
  if (
    reviewing &&
    !writing &&
    reviewing.ids.some((id) => !byId.get(id)?.deletable)
  )
    setReviewing(undefined);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
        Presets
      </h1>
      {!mode && (
        <p role="alert" className="text-sm text-bad">
          {CHANGES_OFF}
        </p>
      )}
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      <DataTable
        key={writes}
        label="Presets"
        emptyText={
          rows.length === 0
            ? "No preset yet. A cluster that fits no preset sets everything itself."
            : undefined
        }
        filters={[
          {
            key: "kind",
            label: "Kind",
            options: PRESET_KINDS.map((k) => ({
              value: k.slug,
              label: k.label,
            })),
          },
        ]}
        selection={{ applicableLabel: "unused", onChange: setSelected }}
        columns={[
          { key: "name", label: "Name", sortable: true },
          { key: "kind", label: "Kind", sortable: true },
          { key: "facts", label: "Key facts" },
          { key: "allowed", label: "Allowed in" },
          { key: "users", label: "Used by", sortable: true },
          { key: "changed", label: "Last changed", sortable: true },
        ]}
        rows={rows.map((row) => {
          const kind = presetLabel(row.kind);
          return {
            id: row.id,
            label: `${row.name} ${kind}`,
            href: presetHref(row.kind, row.name),
            search: `${row.name} ${kind} ${row.description ?? ""}`,
            filters: { kind: row.id.split("/")[0] },
            applicable: row.deletable,
            sort: {
              name: row.name,
              kind,
              users: row.users.length,
              changed: row.lastChanged ?? "",
            },
            cells: {
              // The cell is the content of an inline link, so a block in
              // it would leave the link without a 24 px pointer target.
              name: row.description ? (
                <span className="inline-flex flex-col gap-0.5">
                  <span>{row.name}</span>
                  <span className="max-w-xs text-sm font-normal text-muted">
                    {row.description}
                  </span>
                </span>
              ) : (
                row.name
              ),
              kind,
              facts: <FactsList facts={row.facts} />,
              allowed:
                row.allowedIn.length > 0 ? (
                  row.allowedIn.join(", ")
                ) : (
                  <span className="text-muted">Nowhere</span>
                ),
              users: usedByLabel(row.users),
              changed: row.lastChanged ? (
                <time dateTime={row.lastChanged} className="tabular-nums">
                  {changedLabel(row.lastChanged)}
                </time>
              ) : (
                <span className="text-muted">Unknown</span>
              ),
            },
          };
        })}
      />
      {reviewing && mode ? (
        <section
          aria-label="Review the delete"
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing.change}
            mode={mode}
            diffLabel="Presets to delete"
            effects={[
              ...reviewing.ids.map(
                (id) => `The preset ${byId.get(id)?.name ?? id} is deleted.`,
              ),
              ...(mode === "git" ? [GIT_DELETE_EFFECT] : []),
            ]}
            confirm={{
              label: "Delete",
              gitLabel: "Push the delete to Git",
              danger: true,
            }}
            savedMessage={deletedMessage(reviewing.ids, byId)}
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              if (!result.ok) return;
              setReviewing(undefined);
              setSelected([]);
              setWrites((w) => w + 1);
            }}
            onCancel={() => setReviewing(undefined)}
            onWriting={setWriting}
          />
        </section>
      ) : (
        mode &&
        chosen.length > 0 && (
          <div>
            <Button
              variant="destructive"
              onClick={() =>
                setReviewing({
                  ids: chosen.map((r) => r.id),
                  change: presetDeleteChange(chosen.map((r) => r.preset)),
                })
              }
            >
              {chosen.length === 1
                ? `Delete the preset ${chosen[0].name}`
                : `Delete ${chosen.length} presets`}
            </Button>
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

function deletedMessage(
  ids: readonly string[],
  byId: ReadonlyMap<string, PresetRow>,
): string {
  return ids.length === 1
    ? `The preset ${byId.get(ids[0])?.name ?? ids[0]} is deleted.`
    : `${ids.length} presets are deleted.`;
}

/** PresetListLive is PresetList on the live sources, starting from initial. */
export function PresetListLive({
  initial,
  mode,
  actions,
}: {
  initial: PresetSources;
  mode?: WriteMode;
  actions: PresetActions;
}): ReactNode {
  const sources = usePresetSources(initial);
  return (
    <PresetList
      rows={presetRows(sources, sources.errors)}
      errors={sources.errors}
      mode={mode}
      actions={actions}
    />
  );
}
