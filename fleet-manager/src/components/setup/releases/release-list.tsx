"use client";

import Link from "next/link";
import { type ReactNode, useState } from "react";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { ChangeReview } from "@/components/fleet/change-review";
import { DataTable } from "@/components/fleet/data-table";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { Button } from "@/components/ui/button";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import {
  type ReadError,
  unreadDeleteText,
  unreadKinds,
} from "@/lib/k8s/unread";
import type { Change, WriteMode } from "@/lib/write/types";
import { deleteChange } from "./draft";
import { environmentLabel } from "./environments";
import type { ReleaseActions } from "./release-detail";
import {
  allowedInText,
  defaultInText,
  type ReleaseRow,
  releaseRows,
  usedByText,
} from "./rows";
import { type ReleasesData, useReleaseSources } from "./sources";

const EMPTY = <span className="text-muted">-</span>;

/**
 * ReleaseList is the list page of the releases. releases are the live
 * objects, so a delete names the resourceVersion and the UID that the page
 * showed. stale holds the kinds whose watch has no snapshot on its current
 * connection, as useReleaseSources gives them. Only a row that can be
 * deleted can be selected. A delete review closes, unless its write already
 * runs, when a release of it can no longer be deleted. Without mode the
 * page offers no delete.
 */
export function ReleaseList({
  rows,
  releases,
  environments,
  errors,
  stale = [],
  mode,
  actions,
}: {
  rows: ReleaseRow[];
  releases: readonly CamundaRelease[];
  environments: readonly Environment[];
  errors: ReadError[];
  stale?: readonly string[];
  mode?: WriteMode;
  actions: ReleaseActions;
}): ReactNode {
  const [selected, setSelected] = useState<string[]>([]);
  const [reviewing, setReviewing] = useState<Change>();
  const [writing, setWriting] = useState(false);
  const { toast, show, dismiss } = useWriteToast();
  const deletable = new Set(rows.filter((r) => r.deletable).map((r) => r.name));
  const reviewed = (reviewing?.delete ?? []).map((ref) => ref.name);
  if (reviewing && !writing && reviewed.some((name) => !deletable.has(name)))
    setReviewing(undefined);
  const plural = selected.length === 1 ? "release" : "releases";
  const unread = unreadKinds({ errors, stale });

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
          Releases
        </h1>
        <Button asChild>
          <Link href="/setup/new-release">New release</Link>
        </Button>
      </div>
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
      {mode && unread.length > 0 && (
        <p className="text-sm text-muted">{unreadDeleteText(unread)}</p>
      )}
      <DataTable
        label="Releases"
        emptyText={
          rows.length === 0
            ? "No release yet. Create one with New release."
            : undefined
        }
        columns={[
          { key: "name", label: "Name", sortable: true },
          { key: "camunda", label: "Camunda", sortable: true },
          { key: "connectors", label: "Connectors", sortable: true },
          { key: "elasticsearch", label: "Elasticsearch", sortable: true },
          { key: "postgres", label: "PostgreSQL", sortable: true },
          { key: "defaultIn", label: "Default in", sortable: true },
          { key: "allowedIn", label: "Allowed in", sortable: true },
          { key: "usedBy", label: "Used by", sortable: true },
        ]}
        filters={[
          {
            key: "environment",
            label: "Environment",
            options: environments.map((e) => ({
              value: e.metadata.name ?? "",
              label: environmentLabel(e),
            })),
          },
          {
            key: "usage",
            label: "Usage",
            options: [
              { value: "used", label: "Used" },
              { value: "unused", label: "Unused" },
            ],
          },
        ]}
        selection={{
          applicableLabel: "that can be deleted",
          onChange: setSelected,
        }}
        rows={rows.map((row) => {
          const usedBy = usedByText(row.usage);
          const defaults = defaultInText(row);
          const allowed = allowedInText(row);
          return {
            id: row.name,
            href: `/setup/releases/${encodeURIComponent(row.name)}`,
            search: row.name,
            applicable: row.deletable,
            filters: {
              environment: [
                ...row.allowedIn.map((e) => e.name),
                ...row.defaultIn.map((e) => e.name),
              ],
              usage: usedBy === "Unused" ? "unused" : "used",
            },
            sort: {
              name: row.name,
              camunda: row.camunda,
              connectors: row.connectors ?? "",
              elasticsearch: row.elasticsearch ?? "",
              postgres: row.postgres ?? "",
              defaultIn: defaults ?? "",
              allowedIn: allowed,
              usedBy,
            },
            cells: {
              name: <span className="font-mono">{row.name}</span>,
              camunda: row.camunda,
              connectors: row.connectors ?? EMPTY,
              elasticsearch: row.elasticsearch ?? EMPTY,
              postgres: row.postgres ?? EMPTY,
              defaultIn: defaults ?? EMPTY,
              allowedIn: allowed,
              usedBy,
            },
          };
        })}
      />
      {mode && reviewing ? (
        <section
          aria-label="Review the delete"
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={reviewing}
            mode={mode}
            diffLabel="Releases to delete"
            effects={[
              reviewed.length === 1
                ? `The release ${reviewed[0]} is deleted.`
                : `The releases ${reviewed.join(", ")} are deleted.`,
              ...(mode === "git"
                ? [
                    "In Git mode the delete is a commit that removes the files of the releases. GitOps must prune the CamundaReleases to delete them from the cluster.",
                  ]
                : []),
            ]}
            confirm={{
              label: "Delete",
              gitLabel: "Push the delete to Git",
              danger: true,
            }}
            savedMessage={
              reviewed.length === 1
                ? `The release ${reviewed[0]} is deleted.`
                : `The releases ${reviewed.join(", ")} are deleted.`
            }
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              setReviewing(undefined);
            }}
            onCancel={() => setReviewing(undefined)}
            onWriting={setWriting}
          />
        </section>
      ) : (
        mode &&
        selected.length > 0 && (
          <Button
            variant="destructive"
            className="self-start"
            onClick={() =>
              setReviewing(
                deleteChange(
                  releases.filter((r) =>
                    selected.includes(r.metadata.name ?? ""),
                  ),
                ),
              )
            }
          >
            {`Delete ${selected.length} ${plural}`}
          </Button>
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

/** ReleaseListLive is ReleaseList on the live sources, starting from initial. */
export function ReleaseListLive({
  initial,
  mode,
  actions,
}: {
  initial: ReleasesData;
  mode?: WriteMode;
  actions: ReleaseActions;
}) {
  const sources = useReleaseSources(initial);
  return (
    <ReleaseList
      rows={releaseRows(sources)}
      releases={sources.releases}
      environments={sources.environments}
      errors={sources.errors}
      stale={sources.stale}
      mode={mode}
      actions={actions}
    />
  );
}
