"use client";

import Link from "next/link";
import { type ReactNode, useId, useRef, useState, useTransition } from "react";
import { InlineConfirm } from "@/components/fleet/inline-confirm";
import { TypedConfirm } from "@/components/fleet/typed-confirm";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import type { Change, WriteResult } from "@/lib/write/types";
import {
  type BackupContext,
  backUpNow,
  backupChange,
  backupNameErrors,
  DIRECT_NOTE,
  deleteBackupChange,
  deleteBlock,
  deleteLines,
  manualBackupName,
  pointInTime,
  pointInTimeChange,
  pointInTimeConfirm,
  pointInTimeError,
  type RestoreConfirm,
  restoreBlock,
  restoreChange,
  restoreConfirm,
  restoreNameOf,
  timestampOf,
  waitNote,
} from "./backup-actions";
import type { BackupRow, RestoreInterval } from "./backups";

/**
 * Write is how a backup action writes. apply is the server action of a
 * change of the backups of cluster. show reports its result in the one
 * toast of the page.
 */
type Write = {
  apply: (
    cluster: { namespace: string; name: string },
    change: Change,
  ) => Promise<WriteResult>;
  show: (result: WriteResult, message: string) => void;
};

/**
 * BackupHeaderActions holds Back up now, Restore to a point in time when
 * the cluster has it, Change schedule, and the note that the writes skip
 * Git.
 */
export function BackupHeaderActions({
  context,
  apply,
  show,
}: { context: BackupContext } & Write) {
  const id = useId();
  const state = backUpNow(context);
  const point = pointInTime(context);
  const [name, setName] = useState<string>();
  const [writing, startWrite] = useTransition();
  const open = name !== undefined;
  const errors = open ? backupNameErrors(name, context.backupNames) : [];

  const start = () => {
    if (name === undefined || state.kind === undefined) return;
    const change = backupChange(context, state.kind, name);
    startWrite(async () => {
      const result = await apply(clusterOf(context), change).catch(rejected);
      show(result, `Backup ${name} created.`);
      if (result.ok) setName(undefined);
    });
  };

  return (
    <div className="flex flex-col items-end gap-3">
      <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-2">
        <p className="text-xs text-muted">{DIRECT_NOTE}</p>
        <Link
          href="?tab=claim&section=attachments"
          className={buttonVariants({ variant: "outline", size: "sm" })}
        >
          Change schedule
        </Link>
        {point && (
          <PointInTimeAction
            context={context}
            state={point}
            reasonId={`${id}-point-reason`}
            apply={apply}
            show={show}
          />
        )}
        {!open && (
          <Button
            size="sm"
            disabled={state.disabled !== undefined}
            aria-describedby={state.disabled ? `${id}-reason` : undefined}
            onClick={() =>
              setName(manualBackupName(context.cluster, context.now))
            }
          >
            Back up now
          </Button>
        )}
      </div>
      {point && "disabled" in point && (
        <p
          id={`${id}-point-reason`}
          className="max-w-md text-right text-xs text-muted"
        >
          {point.disabled}
        </p>
      )}
      {state.disabled && (
        <p
          id={`${id}-reason`}
          className="max-w-md text-right text-xs text-muted"
        >
          {state.disabled}
        </p>
      )}
      {open && (
        <form
          aria-label="Back up now"
          className="flex w-full max-w-md flex-col gap-3 rounded-md border border-border bg-panel-2/50 p-3 text-sm"
          onSubmit={(event) => {
            event.preventDefault();
            start();
          }}
        >
          <div className="flex flex-col gap-1">
            <label htmlFor={`${id}-name`} className="font-medium">
              Name
            </label>
            <Input
              id={`${id}-name`}
              value={name}
              autoFocus
              autoComplete="off"
              spellCheck={false}
              aria-invalid={errors.length > 0 || undefined}
              aria-describedby={errors.length > 0 ? `${id}-errors` : undefined}
              onChange={(event) => setName(event.target.value)}
            />
            {errors.length > 0 && (
              <ul
                id={`${id}-errors`}
                className="flex flex-col gap-0.5 text-xs text-bad"
              >
                {errors.map((error) => (
                  <li key={error}>{error}</li>
                ))}
              </ul>
            )}
          </div>
          {state.waits && <p className="text-xs text-info">{state.waits}</p>}
          <div className="flex flex-wrap gap-2">
            <Button
              type="submit"
              size="sm"
              disabled={
                errors.length > 0 || writing || state.disabled !== undefined
              }
              aria-describedby={state.disabled ? `${id}-reason` : undefined}
            >
              Start backup
            </Button>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => setName(undefined)}
            >
              Cancel
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

/** BackupRowActions holds Restore, on a Completed backup only, and Delete. */
export function BackupRowActions({
  context,
  row,
  apply,
  show,
}: { context: BackupContext; row: BackupRow } & Write) {
  return (
    <div className="flex flex-wrap items-start gap-2">
      {row.phase === "Completed" && (
        <RestoreAction context={context} row={row} apply={apply} show={show} />
      )}
      <DeleteAction context={context} row={row} apply={apply} show={show} />
    </div>
  );
}

function RestoreAction({
  context,
  row,
  apply,
  show,
}: { context: BackupContext; row: BackupRow } & Write) {
  const block = restoreBlock(context, row);
  return (
    <RestoreDialog
      trigger="Restore"
      title={`Restore ${context.cluster}`}
      description={`From backup ${row.name}.`}
      block={block}
      context={context}
      write={(name) => restoreChange(context, row, name)}
      what="restore"
      apply={apply}
      show={show}
    >
      <ConfirmLines context={context} {...restoreConfirm(context, row)} />
    </RestoreDialog>
  );
}

function PointInTimeAction({
  context,
  state,
  reasonId,
  apply,
  show,
}: {
  context: BackupContext;
  state: NonNullable<ReturnType<typeof pointInTime>>;
  reasonId: string;
} & Write) {
  const [local, setLocal] = useState("");
  const id = useId();
  const intervals = "intervals" in state ? state.intervals : [];
  const at = timestampOf(local);
  const error = pointInTimeError(at, intervals, context.now);
  const shown = local !== "" ? error : undefined;
  return (
    <RestoreDialog
      trigger="Restore to a point in time"
      title={`Restore ${context.cluster} to a point in time`}
      description="Choose a time in UTC inside the restore window."
      block={"disabled" in state ? state.disabled : undefined}
      reasonId={reasonId}
      size="sm"
      context={context}
      write={(name) => pointInTimeChange(context, name, at!)}
      what="pitr"
      ready={error === undefined}
      onOpen={() => setLocal("")}
      apply={apply}
      show={show}
    >
      <>
        <div className="flex flex-col gap-1">
          <p className="font-medium text-text">Restore window</p>
          <ul className="flex list-disc flex-col gap-0.5 pl-5 text-text-2">
            {intervals.map((interval) => (
              <li key={interval.from}>{intervalText(interval)}</li>
            ))}
          </ul>
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor={`${id}-at`} className="font-medium">
            Time (UTC)
          </label>
          <Input
            id={`${id}-at`}
            type="datetime-local"
            step={1}
            value={local}
            aria-invalid={shown !== undefined || undefined}
            aria-describedby={shown ? `${id}-error` : undefined}
            onChange={(event) => setLocal(event.target.value)}
          />
          {shown && (
            <p id={`${id}-error`} className="text-xs text-bad">
              {shown}
            </p>
          )}
        </div>
        {error === undefined && at !== undefined && (
          <ConfirmLines
            context={context}
            {...pointInTimeConfirm(context, secondText(at))}
          />
        )}
      </>
    </RestoreDialog>
  );
}

// RestoreDialog is the trigger and the dialog of a restore. The dialog
// closes when the write returns, so that the toast of the page, which an
// open modal dialog hides, reports the result.
function RestoreDialog({
  trigger,
  title,
  description,
  block,
  reasonId,
  size,
  context,
  write,
  what,
  ready = true,
  onOpen,
  apply,
  show,
  children,
}: {
  trigger: string;
  title: string;
  description: string;
  block?: string;
  reasonId?: string;
  size?: "sm";
  context: BackupContext;
  write: (name: string) => Change;
  what: "restore" | "pitr";
  ready?: boolean;
  onOpen?: () => void;
  children: ReactNode;
} & Write) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const [writing, startWrite] = useTransition();
  if (block !== undefined) {
    const describedBy = reasonId ?? `${id}-reason`;
    return (
      <div className="flex flex-col gap-1">
        <Button
          variant="outline"
          size={size}
          disabled
          aria-describedby={describedBy}
          className="self-start"
        >
          {trigger}
        </Button>
        {reasonId === undefined && (
          <p id={describedBy} className="max-w-56 text-xs text-muted">
            {block}
          </p>
        )}
      </div>
    );
  }
  const confirm = () => {
    const name = restoreNameOf(context, what);
    const change = write(name);
    startWrite(async () => {
      const result = await apply(clusterOf(context), change).catch(rejected);
      setOpen(false);
      show(result, `Restore ${name} created.`);
    });
  };
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (next) onOpen?.();
        setOpen(next);
      }}
    >
      <DialogTrigger asChild>
        <Button variant="outline" size={size}>
          {trigger}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4 text-sm">
          {children}
          {ready && (
            <TypedConfirm
              name={context.cluster}
              confirmLabel="Restore"
              pending={writing}
              onConfirm={confirm}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function ConfirmLines({
  context,
  lead,
  lines,
}: { context: BackupContext } & RestoreConfirm) {
  const waits =
    context.running?.what === "Backup"
      ? waitNote(context.running, "restore")
      : undefined;
  return (
    <div className="flex flex-col gap-2">
      <p className="font-medium text-text">{lead}</p>
      <ul className="flex list-disc flex-col gap-1 pl-5 text-text-2">
        {lines.map((line) => (
          <li key={line}>{line}</li>
        ))}
      </ul>
      {waits && <p className="text-xs text-info">{waits}</p>}
    </div>
  );
}

function intervalText(interval: RestoreInterval): string {
  const to = interval.to ? secondText(interval.to) : "now";
  const text = `${secondText(interval.from)} to ${to}`;
  return interval.failing
    ? `${text}. Uploads are failing after this point.`
    : text;
}

// secondText writes an RFC 3339 time as "2026-11-10 09:30:15 UTC": a point
// in time needs the seconds that dateTime leaves out.
function secondText(at: string): string {
  const text = new Date(at).toISOString();
  return `${text.slice(0, 10)} ${text.slice(11, 19)} UTC`;
}

function DeleteAction({
  context,
  row,
  apply,
  show,
}: { context: BackupContext; row: BackupRow } & Write) {
  const id = useId();
  const [, startWrite] = useTransition();
  // A second confirm while the first delete runs would send a second
  // delete, which fails because the backup is gone.
  const writing = useRef(false);
  const block = deleteBlock(context, row);
  if (block !== undefined) {
    return (
      <div className="flex flex-col gap-1">
        <Button
          variant="outline"
          disabled
          aria-describedby={`${id}-reason`}
          className="self-start"
        >
          Delete
        </Button>
        <p id={`${id}-reason`} className="max-w-56 text-xs text-muted">
          {block}
        </p>
      </div>
    );
  }
  const { goes, stays } = deleteLines(row);
  return (
    <InlineConfirm
      triggerLabel="Delete"
      confirmLabel="Delete backup"
      goes={goes}
      stays={stays}
      onConfirm={() => {
        if (writing.current) return;
        writing.current = true;
        const change = deleteBackupChange(context.namespace, row);
        startWrite(async () => {
          const result = await apply(clusterOf(context), change).catch(
            rejected,
          );
          writing.current = false;
          show(result, `Backup ${row.name} deleted.`);
        });
      }}
    />
  );
}

function clusterOf(context: BackupContext): {
  namespace: string;
  name: string;
} {
  return { namespace: context.namespace, name: context.cluster };
}

// A server action rejects when its request fails, such as on a network
// error.
function rejected(error: unknown): WriteResult {
  return {
    ok: false,
    errors: [{ code: "Unavailable", message: String(error) }],
  };
}
