"use client";

import { type ReactNode, useId, useState } from "react";
import { LockedField } from "@/components/fleet/locked-field";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { lockLabel, type FieldLock } from "@/lib/fields";
import { useClusterEdit } from "./edit/cluster-edit";
import { changeInputOf } from "./edit/change-input";
import { clusterImpacts } from "./impacts";
import type { ClusterSlotContext } from "./slots";
import type { ComponentMode } from "./topology";
import {
  modeWrite,
  othersPending,
  topologyChange,
  type ModeProcess,
  type ModeRow,
  type TopologyChange,
} from "./topology-change";
import { TopologyBoxes } from "./topology-view";

const MODES: readonly ComponentMode[] = ["Standalone", "Embedded"];

/** Refusal is a pick that a lock refused. */
type Refusal = { process: ModeProcess; lock: FieldLock };

/**
 * TopologyEditor is the Change topology action of a managed cluster. Its
 * picks join the session of the ClusterEditProvider above it, and Review
 * opens the review of that provider. It renders nothing while the page has
 * no session.
 */
export function TopologyEditor({ context }: { context: ClusterSlotContext }) {
  const edit = useClusterEdit();
  const [open, setOpen] = useState(false);
  if (!edit.session) return null;
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          Change topology
        </Button>
      </DialogTrigger>
      <DialogContent
        showCloseButton={false}
        className="max-h-[85vh] overflow-y-auto sm:max-w-3xl"
      >
        <DialogHeader>
          <DialogTitle>Change topology</DialogTitle>
          <DialogDescription>
            Pick where each process runs. Your picks join the pending changes of
            this cluster. Review them to save.
          </DialogDescription>
        </DialogHeader>
        <EditorBody context={context} />
        <DialogFooter showCloseButton>
          <Button
            disabled={edit.pending === 0}
            onClick={() => {
              setOpen(false);
              edit.review();
            }}
          >
            Review
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function EditorBody({ context }: { context: ClusterSlotContext }) {
  const { session, set, reset } = useClusterEdit();
  const [refused, setRefused] = useState<Refusal>();
  if (!session) return null;
  const input = changeInputOf(context, session);
  const onPick = (process: ModeProcess, mode: ComponentMode) => {
    const write = modeWrite(input, process, mode);
    const lock =
      "reset" in write ? reset(write.reset) : set(write.set, write.value);
    setRefused(lock && { process, lock });
  };
  return (
    <TopologyChangePanel
      change={topologyChange(input)}
      impacts={clusterImpacts(input)}
      others={othersPending(session)}
      refused={refused}
      onPick={onPick}
    />
  );
}

/**
 * TopologyChangePanel shows a mode choice for each row of change, the
 * topology now and after the change, and impacts. others says that the
 * change holds pending changes that are not a mode. refused is the last
 * pick that a lock refused.
 */
export function TopologyChangePanel({
  change,
  impacts,
  others,
  refused,
  onPick,
}: {
  change: TopologyChange;
  impacts: string[];
  others: boolean;
  refused?: Refusal;
  onPick: (process: ModeProcess, mode: ComponentMode) => void;
}) {
  const label = (p: ModeProcess) =>
    change.modes.find((m) => m.process === p)?.label ?? p;
  return (
    <div className="flex flex-col gap-5">
      <div className="grid gap-4 sm:grid-cols-2">
        {change.modes.map((row) => (
          <ModeChoice key={row.process} row={row} onPick={onPick} />
        ))}
      </div>
      {refused && (
        <p role="alert" className="text-sm text-bad">
          {label(refused.process)} keeps its mode. {lockLabel(refused.lock)}.
        </p>
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        <Column title="Now">
          <TopologyBoxes
            boxes={change.before}
            marks={change.marks.before}
            pods={false}
            label="Now"
          />
        </Column>
        <Column title="After the change">
          <TopologyBoxes
            boxes={change.after}
            marks={change.marks.after}
            pods={false}
            label="After the change"
          />
        </Column>
      </div>
      <section aria-label="What happens" className="flex flex-col gap-2">
        <h3 className="font-heading text-sm font-semibold text-text">
          What happens
        </h3>
        {impacts.length > 0 ? (
          <ul className="flex list-disc flex-col gap-1 pl-5 text-sm text-text-2">
            {impacts.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-muted">Nothing changes yet.</p>
        )}
        {others && (
          <p className="text-xs text-muted">
            These lines include the other pending changes of this cluster.
          </p>
        )}
      </section>
    </div>
  );
}

function Column({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <h3 className="font-heading text-sm font-semibold text-text">{title}</h3>
      {children}
    </div>
  );
}

function ModeChoice({
  row,
  onPick,
}: {
  row: ModeRow;
  onPick: (process: ModeProcess, mode: ComponentMode) => void;
}) {
  const id = useId();
  if (row.lock)
    return (
      <LockedField
        label={row.label}
        value={row.after}
        reason={lockLabel(row.lock)}
      />
    );
  return (
    <fieldset className="flex min-w-0 flex-col gap-1.5">
      <legend className="mb-1.5 text-sm font-medium text-text">
        {row.label}
      </legend>
      <div className="grid grid-cols-2 gap-0.5 rounded-md border border-border bg-panel-2 p-0.5">
        {MODES.map((mode) => (
          <label
            key={mode}
            className="flex cursor-pointer items-center justify-center rounded px-3 py-1.5 text-sm text-text-2 has-checked:bg-panel has-checked:font-medium has-checked:text-text has-checked:ring-1 has-checked:ring-accent has-focus-visible:ring-3 has-focus-visible:ring-ring/50"
          >
            <input
              type="radio"
              name={id}
              value={mode}
              checked={row.after === mode}
              onChange={() => onPick(row.process, mode)}
              className="sr-only"
            />
            {mode}
          </label>
        ))}
      </div>
      <p className="text-xs text-muted">{hint(row)}</p>
    </fieldset>
  );
}

function hint(row: ModeRow): string {
  if (row.after === "Standalone") return "Own Deployment and Service.";
  return row.host === "zeebe" ? "Runs in the brokers." : "Runs in the gateway.";
}
