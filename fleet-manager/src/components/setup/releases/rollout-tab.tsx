"use client";

import { CircleAlert } from "lucide-react";
import Link from "next/link";
import { type ReactNode, useState } from "react";
import { ChangeReview } from "@/components/fleet/change-review";
import { DataTable } from "@/components/fleet/data-table";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import type { ReadError } from "@/components/planes/planes-table";
import { Button } from "@/components/ui/button";
import type { Resource } from "@/lib/k8s/kinds";
import type { Change, WriteMode } from "@/lib/write/types";
import type { ReleaseActions } from "./release-detail";
import {
  type MovePick,
  type MovePlan,
  namedApiErrors,
  namedErrors,
  PARTIAL_MOVE_CAUTION,
  planMove,
  type RolloutData,
  type RolloutRow,
  rolloutRows,
  STATUS_LABELS,
  unreadMoveText,
  unreadRolloutKinds,
} from "./rollout";

const EMPTY = <span className="text-muted">-</span>;

type Review = { plan: MovePlan & { change: Change }; picks: MovePick[] };

/**
 * RolloutTab lists every cluster with its eligibility for a move to
 * release, and moves one cluster, or every selected cluster in one write,
 * by its claim. Nothing is selected at first, and only a cluster that can
 * move can be selected. errors and stale say which lists are not read:
 * while a list that the move checks is not read, no cluster can move, and
 * an open review closes, unless its write already runs. A review also
 * closes when a cluster of it no longer offers its move. Without mode the
 * tab offers no move.
 */
export function RolloutTab({
  release,
  data,
  mode,
  actions,
}: {
  release: Resource<"CamundaRelease">;
  data: RolloutData & { errors?: readonly ReadError[]; stale?: readonly string[] };
  mode?: WriteMode;
  actions: Pick<ReleaseActions, "previewMove" | "applyMove">;
}): ReactNode {
  const target = release.metadata?.name ?? "";
  const rows = rolloutRows(release, data);
  const unread = unreadRolloutKinds(data);
  const errors = (data.errors ?? []).filter((e) => unread.includes(e.kind));
  const [selected, setSelected] = useState<string[]>([]);
  const [review, setReview] = useState<Review>();
  const [planErrors, setPlanErrors] = useState<string[]>([]);
  const [writing, setWriting] = useState(false);
  const { toast, show, dismiss } = useWriteToast();

  const actionOf = (key: string) => rows.find((r) => r.key === key)?.action;
  if (
    review &&
    !writing &&
    review.picks.some((p) => actionOf(p.row.key) !== p.row.action)
  )
    setReview(undefined);

  const move = (picks: MovePick[]) => {
    const plan = planMove(release, picks, data);
    if (plan.change === undefined) {
      setPlanErrors(plan.errors);
      return;
    }
    setPlanErrors([]);
    setReview({ plan: { ...plan, change: plan.change }, picks });
  };
  const movable = rows.filter(
    (r) => r.status.kind === "canMove" && selected.includes(r.key),
  );

  return (
    <div className="flex flex-col gap-4">
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {!mode && (
        <p className="text-sm text-text-2">
          Set up the fleet manager settings to move clusters.{" "}
          <Link
            href="/setup/settings"
            className="relative font-medium text-info hit-area hover:underline"
          >
            Open the settings
          </Link>
        </p>
      )}
      {mode && unread.length > 0 && (
        <p className="text-sm text-muted">{unreadMoveText(unread)}</p>
      )}
      {planErrors.length > 0 && (
        <div
          role="alert"
          className="flex gap-3 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm text-text"
        >
          <CircleAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-bad" />
          <ul className="flex flex-col gap-1">
            {planErrors.map((text) => (
              <li key={text}>{text}</li>
            ))}
          </ul>
        </div>
      )}
      {mode && review ? (
        <section
          aria-label="Review the move"
          className="rounded-md border border-border bg-panel p-5"
        >
          <MoveReview
            review={review}
            target={target}
            mode={mode}
            actions={actions}
            show={(result, message) => {
              show(result, message);
              setReview(undefined);
            }}
            onCancel={() => setReview(undefined)}
            onWriting={setWriting}
          />
        </section>
      ) : (
        <>
          <DataTable
            label="Clusters"
            emptyText={
              rows.length === 0 ? "No cluster exists yet." : undefined
            }
            columns={[
              { key: "name", label: "Name", sortable: true },
              { key: "environment", label: "Environment", sortable: true },
              { key: "namespace", label: "Namespace", sortable: true },
              { key: "release", label: "Current release", sortable: true },
              { key: "version", label: "Running version", sortable: true },
              { key: "eligibility", label: "Eligibility", sortable: true },
              { key: "action", label: "Action" },
            ]}
            filters={[
              {
                key: "environment",
                label: "Environment",
                options: data.environments.map((e) => ({
                  value: e.metadata?.name ?? "",
                  label: e.spec.displayName || (e.metadata?.name ?? ""),
                })),
              },
              {
                key: "eligibility",
                label: "Eligibility",
                options: [...new Set(Object.values(STATUS_LABELS))].map(
                  (label) => ({ value: label, label }),
                ),
              },
            ]}
            {...(mode && {
              selection: {
                applicableLabel: "that can move",
                onChange: setSelected,
              },
            })}
            rows={rows.map((row) => ({
              id: row.key,
              label: row.key,
              href: `/fleet/clusters/${encodeURIComponent(row.namespace)}/${encodeURIComponent(row.name)}`,
              search: `${row.name} ${row.namespace}`,
              applicable: row.status.kind === "canMove",
              filters: {
                environment: row.environment ?? "",
                eligibility: STATUS_LABELS[row.status.kind],
              },
              sort: {
                name: row.name,
                environment: row.environment ?? "",
                namespace: row.namespace,
                release: row.currentRelease ?? "",
                version: row.runningVersion ?? "",
                eligibility: STATUS_LABELS[row.status.kind],
              },
              cells: {
                name: row.name,
                environment: (
                  <EnvironmentCell row={row} environments={data.environments} />
                ),
                namespace: row.namespace,
                release: row.currentRelease ? (
                  <span className="font-mono">{row.currentRelease}</span>
                ) : (
                  EMPTY
                ),
                version: row.runningVersion ?? (
                  <span className="text-muted">Not reported</span>
                ),
                eligibility: (
                  <div className="flex max-w-80 flex-col gap-1">
                    <span>{row.status.text}</span>
                    {row.refusal && (
                      <span className="text-xs text-text-2">
                        {row.refusal}
                      </span>
                    )}
                  </div>
                ),
                action: mode ? (
                  <RowAction
                    row={row}
                    onMove={(downgrade) => move([{ row, downgrade }])}
                  />
                ) : (
                  EMPTY
                ),
              },
            }))}
          />
          {mode && (
            <Button
              className="self-start"
              disabled={movable.length === 0}
              onClick={() =>
                move(movable.map((row) => ({ row, downgrade: false })))
              }
            >
              Move selected
            </Button>
          )}
        </>
      )}
      {toast && (
        <div className="fixed right-4 bottom-4 z-20">
          <Toast result={toast} onDismiss={dismiss} />
        </div>
      )}
    </div>
  );
}

function EnvironmentCell({
  row,
  environments,
}: {
  row: RolloutRow;
  environments: RolloutData["environments"];
}): ReactNode {
  if (!row.environment)
    return <span className="text-muted">No environment</span>;
  const environment = environments.find(
    (e) => e.metadata?.name === row.environment,
  );
  return (
    <Link
      href={`/setup/environments/${encodeURIComponent(row.environment)}`}
      className="relative text-text hit-area hover:text-info hover:underline"
    >
      {environment?.spec.displayName || row.environment}
    </Link>
  );
}

function RowAction({
  row,
  onMove,
}: {
  row: RolloutRow;
  onMove: (downgrade: boolean) => void;
}): ReactNode {
  switch (row.action) {
    case "move":
      return (
        <Button size="sm" variant="outline" onClick={() => onMove(false)}>
          Move
        </Button>
      );
    case "confirmDowngrade":
      return (
        <Button size="sm" variant="outline" onClick={() => onMove(true)}>
          Confirm downgrade
        </Button>
      );
    case "openEnvironments":
      return (
        <Link
          href={`/setup/environments/${encodeURIComponent(row.environment ?? "")}`}
          className="relative text-sm font-medium text-info hit-area hover:underline"
        >
          Open Environments
        </Link>
      );
    default:
      return EMPTY;
  }
}

function MoveReview({
  review,
  target,
  mode,
  actions,
  show,
  onCancel,
  onWriting,
}: {
  review: Review;
  target: string;
  mode: WriteMode;
  actions: Pick<ReleaseActions, "previewMove" | "applyMove">;
  show: Parameters<typeof ChangeReview>[0]["show"];
  onCancel: () => void;
  onWriting: (writing: boolean) => void;
}): ReactNode {
  const { change, impacts } = review.plan;
  const [first] = change.apply;
  const downgrade = review.picks.some((p) => p.downgrade);
  const count = change.apply.length;
  return (
    <ChangeReview
      change={change}
      mode={mode}
      diffLabel={`Changes to the claim ${first.metadata.namespace}/${first.metadata.name}`}
      effects={impacts.map((i) => `${i.cluster}: ${i.lines.join(" ")}`)}
      caution={mode === "direct" && count > 1 ? PARTIAL_MOVE_CAUTION : undefined}
      confirm={
        downgrade
          ? { label: "Confirm downgrade", danger: true }
          : { label: "Move" }
      }
      savedMessage={
        count === 1
          ? `Moved ${review.picks[0].row.key} to ${target}.`
          : `Moved ${count} clusters to ${target}.`
      }
      preview={(c) =>
        actions
          .previewMove(c)
          .then((p) => ({ ...p, errors: namedApiErrors(p.errors) }))
      }
      apply={(c) => actions.applyMove(c).then(namedErrors)}
      show={show}
      onCancel={onCancel}
      onWriting={onWriting}
    />
  );
}
